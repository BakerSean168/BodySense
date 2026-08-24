package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PrivacyDataCount is safe metadata only; no health values are copied into the
// erasure audit trail.
type PrivacyDataCount struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

type privacyCountSpec struct {
	name   string
	table  string
	column string
}

// Direct user-owned rows. Child rows are removed through FK cascades and are
// documented in the retention matrix rather than duplicated in this report.
var privacyCountSpecs = []privacyCountSpec{
	{name: "identity_profile", table: "user_profiles", column: "user_id"},
	{name: "conversations", table: "conversations", column: "user_id"},
	{name: "runs", table: "runs", column: "user_id"},
	{name: "jobs", table: "jobs", column: "user_id"},
	{name: "ai_output_reviews", table: "ai_output_reviews", column: "user_id"},
	{name: "assessment_reports", table: "assessment_reports", column: "user_id"},
	{name: "training_plans", table: "training_plans", column: "user_id"},
	{name: "training_logs", table: "training_logs", column: "user_id"},
	{name: "uploads", table: "user_uploads", column: "user_id"},
	{name: "thread_projections", table: "thread_projections", column: "user_id"},
	{name: "body_state", table: "body_states", column: "user_id"},
	{name: "body_state_facts", table: "body_state_facts", column: "user_id"},
	{name: "body_state_observations", table: "body_state_observations", column: "user_id"},
	{name: "body_state_revisions", table: "body_state_revisions", column: "user_id"},
	{name: "body_state_evidence", table: "body_state_evidence", column: "user_id"},
	{name: "body_state_hypotheses", table: "body_state_hypotheses", column: "user_id"},
	{name: "diagnosis_analyses", table: "diagnosis_analyses", column: "user_id"},
	{name: "diagnosis_candidate_assessments", table: "diagnosis_candidate_assessments", column: "user_id"},
	{name: "diagnosis_freshness", table: "diagnosis_analysis_freshness", column: "user_id"},
	{name: "treatments", table: "treatments", column: "user_id"},
	{name: "interventions", table: "interventions", column: "user_id"},
	{name: "outcomes", table: "outcomes", column: "user_id"},
}

// PrivacyErasureRepository owns durable erasure workflow state and dry-run
// counts. It intentionally does not delete domain rows itself; the final user
// delete is performed through UserRepository inside one DB transaction.
type PrivacyErasureRepository struct {
	db *gorm.DB
}

func NewPrivacyErasureRepository(db *gorm.DB) *PrivacyErasureRepository {
	return &PrivacyErasureRepository{db: db}
}

// DeleteRestrictBoundAggregates removes only aggregates whose domain FK is
// intentionally RESTRICT in normal operation. This privileged lifecycle step
// runs inside the erasure transaction so normal Diagnosis/Treatment lineage
// remains protected everywhere else.
func (r *PrivacyErasureRepository) DeleteRestrictBoundAggregates(ctx context.Context, userID uuid.UUID) error {
	db := database.FromContext(ctx, r.db)
	if err := db.Exec("DELETE FROM treatments WHERE user_id = ?", userID).Error; err != nil {
		return fmt.Errorf("delete privacy restrict-bound treatments: %w", err)
	}
	return nil
}

func (r *PrivacyErasureRepository) CountUserData(ctx context.Context, userID uuid.UUID) ([]PrivacyDataCount, error) {
	counts := make([]PrivacyDataCount, 0, len(privacyCountSpecs)+1)
	var users int64
	if err := r.db.WithContext(ctx).Table("users").Where("id = ?", userID).Count(&users).Error; err != nil {
		return nil, fmt.Errorf("count user identity: %w", err)
	}
	counts = append(counts, PrivacyDataCount{Name: "account", Count: users})
	for _, spec := range privacyCountSpecs {
		var count int64
		if err := r.db.WithContext(ctx).Table(spec.table).Where(spec.column+" = ?", userID).Count(&count).Error; err != nil {
			return nil, fmt.Errorf("count %s: %w", spec.name, err)
		}
		counts = append(counts, PrivacyDataCount{Name: spec.name, Count: count})
	}
	return counts, nil
}

func (r *PrivacyErasureRepository) CreateOrGet(ctx context.Context, userID uuid.UUID, subjectDigest string, report datatypes.JSON) (*model.PrivacyErasureRequest, error) {
	request := &model.PrivacyErasureRequest{
		ID:            uuid.New(),
		SubjectUserID: &userID,
		SubjectDigest: subjectDigest,
		Status:        "pending",
		Report:        report,
	}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "subject_digest"}}, DoNothing: true}).Create(request).Error; err != nil {
		return nil, fmt.Errorf("create privacy erasure request: %w", err)
	}
	var existing model.PrivacyErasureRequest
	if err := r.db.WithContext(ctx).Where("subject_digest = ?", subjectDigest).First(&existing).Error; err != nil {
		return nil, fmt.Errorf("load privacy erasure request: %w", err)
	}
	return &existing, nil
}

func (r *PrivacyErasureRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.PrivacyErasureRequest, error) {
	var request model.PrivacyErasureRequest
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&request).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &request, nil
}

func (r *PrivacyErasureRepository) TryClaim(ctx context.Context, id uuid.UUID, owner string, lease time.Duration) (*model.PrivacyErasureRequest, bool, error) {
	leaseUntil := time.Now().UTC().Add(lease)
	var request model.PrivacyErasureRequest
	result := r.db.WithContext(ctx).Raw(`
UPDATE privacy_erasure_requests
SET status = 'running',
    attempt_count = attempt_count + 1,
    lease_owner = ?,
    lease_expires_at = ?,
    last_error = NULL,
    updated_at = now()
WHERE id = ?
  AND subject_user_id IS NOT NULL
  AND (
      status IN ('pending', 'retryable')
      OR (status = 'running' AND (lease_expires_at IS NULL OR lease_expires_at < now()))
  )
RETURNING *
`, owner, leaseUntil, id).Scan(&request)
	if result.Error != nil {
		return nil, false, fmt.Errorf("claim privacy erasure request: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, false, nil
	}
	return &request, true, nil
}

func (r *PrivacyErasureRepository) MarkRetryable(ctx context.Context, id uuid.UUID, failure string) error {
	if len(failure) > 2000 {
		failure = failure[:2000]
	}
	return r.db.WithContext(ctx).Model(&model.PrivacyErasureRequest{}).Where("id = ?", id).Updates(map[string]any{
		"status":           "retryable",
		"last_error":       failure,
		"lease_owner":      nil,
		"lease_expires_at": nil,
		"updated_at":       gorm.Expr("now()"),
	}).Error
}

func (r *PrivacyErasureRepository) MarkCompleted(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&model.PrivacyErasureRequest{}).Where("id = ?", id).Updates(map[string]any{
		"status":           "completed",
		"subject_user_id":  nil,
		"last_error":       nil,
		"lease_owner":      nil,
		"lease_expires_at": nil,
		"completed_at":     gorm.Expr("now()"),
		"updated_at":       gorm.Expr("now()"),
	}).Error
}

func (r *PrivacyErasureRepository) ListRecoverable(ctx context.Context, limit int) ([]uuid.UUID, error) {
	if limit <= 0 {
		limit = 10
	}
	var ids []uuid.UUID
	if err := r.db.WithContext(ctx).Model(&model.PrivacyErasureRequest{}).
		Select("id").
		Where("subject_user_id IS NOT NULL").
		Where("status IN ? OR (status = ? AND (lease_expires_at IS NULL OR lease_expires_at < now()))", []string{"pending", "retryable"}, "running").
		Order("requested_at ASC").
		Limit(limit).
		Scan(&ids).Error; err != nil {
		return nil, fmt.Errorf("list recoverable privacy erasures: %w", err)
	}
	return ids, nil
}
