package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// StatementErasureRepository owns the per-statement erasure request, its
// target receipts and the database-resident copies of one user statement
// (statement_evidence) and the BodyState records derived from it. Runtime
// checkpoints are executed by the service through the ai-service port; the
// source chat message is deleted here when nothing else cites it.
type StatementErasureRepository struct {
	db  *gorm.DB
	now func() time.Time
}

func NewStatementErasureRepository(db *gorm.DB) *StatementErasureRepository {
	return &StatementErasureRepository{db: db, now: func() time.Time { return time.Now().UTC() }}
}

func (r *StatementErasureRepository) conn(ctx context.Context) *gorm.DB {
	return database.FromContext(ctx, r.db)
}

// CreateOrGet admits one erasure per owned statement. A repeated command for
// the same statement returns the existing request whatever its key; a key
// already used for another statement is rejected. Missing and foreign
// statements are absent.
func (r *StatementErasureRepository) CreateOrGet(ctx context.Context, userID, statementID uuid.UUID, idempotencyKey string) (*model.StatementErasureReceipt, error) {
	var requestID uuid.UUID
	err := r.conn(ctx).Transaction(func(tx *gorm.DB) error {
		var statement model.StatementEvidence
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND id = ?", userID, statementID).Take(&statement).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return model.ErrStatementErasureNotFound
			}
			return err
		}
		var byKey model.StatementErasureRequest
		err := tx.Where("user_id = ? AND idempotency_key = ?", userID, idempotencyKey).Take(&byKey).Error
		if err == nil && byKey.StatementID != statementID {
			return model.ErrStatementErasureIdempotencyUsed
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var existing model.StatementErasureRequest
		err = tx.Where("user_id = ? AND statement_id = ?", userID, statementID).Take(&existing).Error
		if err == nil {
			requestID = existing.ID
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		now := r.now()
		request := model.StatementErasureRequest{
			ID: uuid.New(), UserID: userID, StatementID: statementID, IdempotencyKey: idempotencyKey,
			InventoryRevision: model.StatementErasureInventoryRevision, Status: model.StatementErasurePending,
			RequestedAt: now, UpdatedAt: now,
		}
		if err := tx.Create(&request).Error; err != nil {
			return err
		}
		targets := make([]model.StatementErasureTarget, 0, len(model.StatementErasureTargets))
		for _, name := range model.StatementErasureTargets {
			targets = append(targets, model.StatementErasureTarget{
				UserID: userID, RequestID: request.ID, Target: name,
				State: model.AssetErasureTargetPending, UpdatedAt: now,
			})
		}
		if err := tx.Create(&targets).Error; err != nil {
			return err
		}
		requestID = request.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.Receipt(ctx, userID, requestID)
}

// GetByStatement returns the owner's receipt for one statement.
func (r *StatementErasureRepository) GetByStatement(ctx context.Context, userID, statementID uuid.UUID) (*model.StatementErasureReceipt, error) {
	var request model.StatementErasureRequest
	if err := r.conn(ctx).Where("user_id = ? AND statement_id = ?", userID, statementID).Take(&request).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.ErrStatementErasureNotFound
		}
		return nil, err
	}
	return r.Receipt(ctx, userID, request.ID)
}

// Receipt loads one request with its targets in inventory order and the
// live review signal: still-asserted derived records, the adopted conditions
// citing the statement and the plans resting on them.
func (r *StatementErasureRepository) Receipt(ctx context.Context, userID, requestID uuid.UUID) (*model.StatementErasureReceipt, error) {
	db := r.conn(ctx)
	var request model.StatementErasureRequest
	if err := db.Where("user_id = ? AND id = ?", userID, requestID).Take(&request).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.ErrStatementErasureNotFound
		}
		return nil, err
	}
	var rows []model.StatementErasureTarget
	if err := db.Where("user_id = ? AND request_id = ?", userID, requestID).Find(&rows).Error; err != nil {
		return nil, err
	}
	byName := make(map[model.StatementErasureTargetName]model.StatementErasureTarget, len(rows))
	for _, row := range rows {
		byName[row.Target] = row
	}
	receipt := &model.StatementErasureReceipt{Request: request, Targets: make([]model.StatementErasureTarget, 0, len(rows))}
	for _, name := range model.StatementErasureTargets {
		if row, ok := byName[name]; ok {
			receipt.Targets = append(receipt.Targets, row)
		}
	}
	if request.Status.Terminal() {
		receipt.Reviews = []model.ErasureReview{}
		return receipt, nil
	}
	derived, err := statementDerivedRecords(db, userID, request.StatementID, false)
	if err != nil {
		return nil, err
	}
	reviews := derived.activeReviews()
	conditions, err := statementPinningConditions(db, userID, request.StatementID, derived)
	if err != nil {
		return nil, err
	}
	dependents, err := dependentReviews(db, userID, conditions, derived.bodyRefs())
	if err != nil {
		return nil, err
	}
	receipt.Reviews = append(reviews, dependents...)
	return receipt, nil
}

// Claim leases a request for one execution pass. Terminal requests and live
// leases are never claimed, so two workers never execute the same request.
func (r *StatementErasureRepository) Claim(ctx context.Context, requestID uuid.UUID, owner string, lease time.Duration) (*model.StatementErasureRequest, bool, error) {
	now := r.now()
	var request model.StatementErasureRequest
	result := r.conn(ctx).Raw(`
UPDATE statement_erasure_requests
SET status = 'running', attempt_count = attempt_count + 1,
    lease_owner = ?, lease_expires_at = ?, updated_at = ?
WHERE id = ?
  AND (status IN ('pending','outstanding') OR (status = 'running' AND lease_expires_at < ?))
RETURNING *`, owner, now.Add(lease), now, requestID, now).Scan(&request)
	if result.Error != nil {
		return nil, false, fmt.Errorf("claim statement erasure: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, false, nil
	}
	return &request, true, nil
}

// Release ends a pass that did not reach a terminal state.
func (r *StatementErasureRepository) Release(ctx context.Context, request *model.StatementErasureRequest) error {
	return r.conn(ctx).Exec(`UPDATE statement_erasure_requests
SET status = 'outstanding', lease_owner = NULL, lease_expires_at = NULL, updated_at = ?
WHERE user_id = ? AND id = ? AND status = 'running' AND lease_owner = ?`,
		r.now(), request.UserID, request.ID, *request.LeaseOwner).Error
}

// RecordOutcome stores one target's result.
func (r *StatementErasureRepository) RecordOutcome(ctx context.Context, request *model.StatementErasureRequest, target model.StatementErasureTargetName, outcome model.AssetErasureOutcome) error {
	return recordStatementErasureOutcome(r.conn(ctx), r.now(), request.UserID, request.ID, target, outcome)
}

func recordStatementErasureOutcome(tx *gorm.DB, now time.Time, userID, requestID uuid.UUID, target model.StatementErasureTargetName, outcome model.AssetErasureOutcome) error {
	var code, reason, verifiedAt any
	outstanding := outcome.Outstanding
	switch outcome.State {
	case model.AssetErasureTargetVerified:
		verifiedAt, outstanding = now, 0
	case model.AssetErasureTargetPending:
	case model.ErasureTargetRetainedByPolicy:
		reason = outcome.RetentionReason
	default:
		code = outcome.FailureCode
	}
	result := tx.Exec(`UPDATE statement_erasure_targets
SET state = ?, erased_count = erased_count + ?, outstanding_count = ?, failure_code = ?, retention_reason = ?,
    attempt_count = attempt_count + 1, verified_at = ?, updated_at = ?
WHERE user_id = ? AND request_id = ? AND target = ?`,
		outcome.State, outcome.Erased, outstanding, code, reason, verifiedAt, now, userID, requestID, target)
	if result.Error != nil {
		return fmt.Errorf("record statement erasure target %s: %w", target, result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("record statement erasure target %s: target row missing", target)
	}
	return nil
}

// ListRecoverable returns requests a worker should run: new ones, expired
// leases, and outstanding ones with a transient failure older than
// retryAfter. Blocked targets wait for the user's review and a repeated
// command; retained_by_policy is terminal and never retried.
func (r *StatementErasureRepository) ListRecoverable(ctx context.Context, retryAfter time.Duration, limit int) ([]uuid.UUID, error) {
	if limit <= 0 {
		limit = 10
	}
	now := r.now()
	var ids []uuid.UUID
	err := r.conn(ctx).Raw(`SELECT q.id FROM statement_erasure_requests q
WHERE q.status = 'pending'
   OR (q.status = 'running' AND q.lease_expires_at < ?)
   OR (q.status = 'outstanding' AND q.updated_at < ? AND EXISTS (
        SELECT 1 FROM statement_erasure_targets t
        WHERE t.user_id = q.user_id AND t.request_id = q.id AND t.state IN ('pending','failed')))
ORDER BY q.updated_at ASC
LIMIT ?`, now, now.Add(-retryAfter), limit).Scan(&ids).Error
	if err != nil {
		return nil, fmt.Errorf("list recoverable statement erasures: %w", err)
	}
	return ids, nil
}
