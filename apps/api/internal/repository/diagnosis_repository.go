package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DiagnosisRepository provides storage reads only, plus idempotent empty-head
// initialization. The future application service owns adoption transactions.
type DiagnosisRepository struct{ db *gorm.DB }

func NewDiagnosisRepository(db *gorm.DB) *DiagnosisRepository { return &DiagnosisRepository{db: db} }

func (r *DiagnosisRepository) EnsureAggregate(ctx context.Context, owner uuid.UUID) (*model.Diagnosis, error) {
	if owner == uuid.Nil {
		return nil, model.ErrDiagnosisNotFound
	}
	row := model.Diagnosis{UserID: owner}
	if err := database.FromContext(ctx, r.db).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}}, DoNothing: true}).Create(&row).Error; err != nil {
		return nil, fmt.Errorf("initialize diagnosis: %w", err)
	}
	return r.GetHead(ctx, owner)
}

func (r *DiagnosisRepository) GetHead(ctx context.Context, owner uuid.UUID) (*model.Diagnosis, error) {
	var row model.Diagnosis
	if err := r.ownerDB(ctx, owner).Take(&row).Error; err != nil {
		return nil, diagnosisReadError(err)
	}
	return &row, nil
}
func (r *DiagnosisRepository) GetCondition(ctx context.Context, owner, id uuid.UUID) (*model.ConditionEntry, error) {
	var row model.ConditionEntry
	if err := r.ownerDB(ctx, owner).Where("id = ?", id).Take(&row).Error; err != nil {
		return nil, diagnosisReadError(err)
	}
	return &row, nil
}
func (r *DiagnosisRepository) ListConditions(ctx context.Context, owner uuid.UUID) ([]model.ConditionEntry, error) {
	rows := []model.ConditionEntry{}
	err := r.ownerDB(ctx, owner).Order("created_at, id").Find(&rows).Error
	return rows, err
}
func (r *DiagnosisRepository) ListEvidence(ctx context.Context, owner, condition uuid.UUID) ([]model.ConditionEntryEvidence, error) {
	// A foreign/missing condition is indistinguishable even when it has no links.
	if _, err := r.GetCondition(ctx, owner, condition); err != nil {
		return nil, err
	}
	rows := []model.ConditionEntryEvidence{}
	err := r.ownerDB(ctx, owner).Where("condition_id = ?", condition).
		Order("evidence_type, evidence_id, evidence_version, role").Find(&rows).Error
	return rows, err
}
func (r *DiagnosisRepository) GetAnalysisRun(ctx context.Context, owner, id uuid.UUID) (*model.ConditionAnalysisRun, error) {
	var row model.ConditionAnalysisRun
	if err := r.ownerDB(ctx, owner).Where("id = ?", id).Take(&row).Error; err != nil {
		return nil, diagnosisReadError(err)
	}
	return &row, nil
}
func (r *DiagnosisRepository) GetProposal(ctx context.Context, owner, id uuid.UUID) (*model.DiagnosisProposal, error) {
	var row model.DiagnosisProposal
	if err := r.ownerDB(ctx, owner).Where("id = ?", id).Take(&row).Error; err != nil {
		return nil, diagnosisReadError(err)
	}
	return &row, nil
}
func (r *DiagnosisRepository) GetDecisionReceipt(ctx context.Context, owner uuid.UUID, key string) (*model.DiagnosisDecision, error) {
	var row model.DiagnosisDecision
	if err := r.ownerDB(ctx, owner).Where("idempotency_key = ?", key).Take(&row).Error; err != nil {
		return nil, diagnosisReadError(err)
	}
	return &row, nil
}
func (r *DiagnosisRepository) GetRevision(ctx context.Context, owner uuid.UUID, revision int64) (*model.DiagnosisRevision, error) {
	var row model.DiagnosisRevision
	if err := r.ownerDB(ctx, owner).Where("revision = ?", revision).Take(&row).Error; err != nil {
		return nil, diagnosisReadError(err)
	}
	return &row, nil
}
func (r *DiagnosisRepository) ownerDB(ctx context.Context, owner uuid.UUID) *gorm.DB {
	return database.FromContext(ctx, r.db).Where("user_id = ?", owner)
}
func diagnosisReadError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.ErrDiagnosisNotFound
	}
	return fmt.Errorf("read diagnosis: %w", err)
}
