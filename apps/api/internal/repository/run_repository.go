package repository

import (
	"context"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RunRepository handles database operations for runs.
type RunRepository struct {
	db *gorm.DB
}

const runLeaseErrorJSON = `{"message":"run execution lost; lease expired"}`

// NewRunRepository creates a new RunRepository.
func NewRunRepository(db *gorm.DB) *RunRepository {
	return &RunRepository{db: db}
}

// Create creates a new run.
func (r *RunRepository) Create(ctx context.Context, run *model.Run) error {
	return database.FromContext(ctx, r.db).Create(run).Error
}

// CreateWithIdempotency inserts a run atomically using the database unique
// constraint on (user_id, request_id). If a run with the same request_id
// already exists, it returns the existing run.
func (r *RunRepository) CreateWithIdempotency(ctx context.Context, run *model.Run) (*model.Run, bool, error) {
	if run.RequestID == "" {
		return run, false, r.Create(ctx, run)
	}

	var existed bool
	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "request_id"}},
			DoNothing: true,
		}).Create(run)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			return nil
		}

		existed = true
		var existing model.Run
		if err := tx.Where("user_id = ? AND request_id = ?", run.UserID, run.RequestID).First(&existing).Error; err != nil {
			return err
		}
		*run = existing
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return run, existed, nil
}

// GetByID retrieves a run by ID.
func (r *RunRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Run, error) {
	var run model.Run
	err := database.FromContext(ctx, r.db).
		Where("id = ?", id).
		First(&run).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// GetByRequestID retrieves a run by user ID and request ID (idempotency check).
func (r *RunRepository) GetByRequestID(ctx context.Context, userID uuid.UUID, requestID string) (*model.Run, error) {
	var run model.Run
	err := database.FromContext(ctx, r.db).
		Where("user_id = ? AND request_id = ?", userID, requestID).
		First(&run).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// ListByConversationID retrieves all runs for a conversation.
func (r *RunRepository) ListByConversationID(ctx context.Context, conversationID uuid.UUID) ([]model.Run, error) {
	var runs []model.Run
	err := database.FromContext(ctx, r.db).
		Where("conversation_id = ?", conversationID).
		Order("started_at ASC").
		Find(&runs).Error
	return runs, err
}

// MarkWaitingUser transitions exactly running -> waiting_user and releases the
// execution lease. A false result means another lifecycle transition won.
func (r *RunRepository) MarkWaitingUser(ctx context.Context, id uuid.UUID) (bool, error) {
	result := database.FromContext(ctx, r.db).
		Model(&model.Run{}).
		Where("id = ? AND status = ?", id, model.RunStatusRunning).
		Updates(map[string]any{
			"status":             model.RunStatusWaitingUser,
			"lease_owner":        "",
			"lease_expires_at":   nil,
			"lease_heartbeat_at": nil,
		})
	return result.RowsAffected == 1, result.Error
}

// ResumeRunning transitions exactly waiting_user -> running and establishes a
// new execution lease for the resuming API process.
func (r *RunRepository) ResumeRunning(ctx context.Context, id uuid.UUID, owner string, expiresAt time.Time) (bool, error) {
	now := time.Now()
	result := database.FromContext(ctx, r.db).
		Model(&model.Run{}).
		Where("id = ? AND status = ?", id, model.RunStatusWaitingUser).
		Updates(map[string]any{
			"status":             model.RunStatusRunning,
			"lease_owner":        owner,
			"lease_expires_at":   expiresAt,
			"lease_heartbeat_at": now,
		})
	return result.RowsAffected == 1, result.Error
}

// RenewLease extends only the lease owned by this API process. A false result
// means another terminal transition or reconciler already won the race.
func (r *RunRepository) RenewLease(ctx context.Context, id, userID uuid.UUID, owner string, expiresAt, heartbeatAt time.Time) (bool, error) {
	result := database.FromContext(ctx, r.db).
		Model(&model.Run{}).
		Where("id = ? AND user_id = ? AND status = ? AND lease_owner = ?", id, userID, model.RunStatusRunning, owner).
		Updates(map[string]any{"lease_expires_at": expiresAt, "lease_heartbeat_at": heartbeatAt})
	return result.RowsAffected == 1, result.Error
}

// ReclaimExpiredRuns atomically claims and fails expired running executions.
// SKIP LOCKED lets multiple API instances reconcile concurrently without
// producing duplicate terminal transitions.
func (r *RunRepository) ReclaimExpiredRuns(ctx context.Context, now time.Time, limit int) ([]model.Run, error) {
	if limit <= 0 {
		limit = 100
	}
	var reclaimed []model.Run
	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		var candidates []model.Run
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? AND lease_expires_at IS NOT NULL AND lease_expires_at <= ?", model.RunStatusRunning, now).
			Order("lease_expires_at ASC").Limit(limit).Find(&candidates).Error; err != nil {
			return err
		}
		for _, candidate := range candidates {
			result := tx.Model(&model.Run{}).
				Where("id = ? AND status = ? AND lease_expires_at <= ?", candidate.ID, model.RunStatusRunning, now).
				Updates(map[string]any{
					"status":             model.RunStatusFailed,
					"error":              datatypes.JSON([]byte(runLeaseErrorJSON)),
					"completed_at":       now,
					"lease_owner":        "",
					"lease_expires_at":   nil,
					"lease_heartbeat_at": nil,
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 1 {
				candidate.Status = model.RunStatusFailed
				candidate.Error = datatypes.JSON([]byte(runLeaseErrorJSON))
				candidate.CompletedAt = &now
				candidate.LeaseOwner = ""
				candidate.LeaseExpiresAt = nil
				candidate.LeaseHeartbeatAt = nil
				reclaimed = append(reclaimed, candidate)
			}
		}
		return nil
	})
	return reclaimed, err
}

// CompleteRun marks a run as completed only from an active lifecycle state.
func (r *RunRepository) CompleteRun(ctx context.Context, id, userID uuid.UUID, usage any, providerResponseID string) error {
	_, err := r.TryCompleteRun(ctx, id, userID, usage, providerResponseID)
	return err
}

// TryCompleteRun performs the terminal transition atomically. The boolean is
// false when cancellation/failure/completion won the race first.
func (r *RunRepository) TryCompleteRun(ctx context.Context, id, userID uuid.UUID, usage any, providerResponseID string) (bool, error) {
	now := time.Now()
	result := database.FromContext(ctx, r.db).
		Model(&model.Run{}).
		Where("id = ? AND user_id = ? AND status IN ?", id, userID, []model.RunStatus{model.RunStatusRunning, model.RunStatusWaitingUser}).
		Updates(map[string]any{
			"status":               model.RunStatusCompleted,
			"usage":                usage,
			"provider_response_id": providerResponseID,
			"completed_at":         now,
			"lease_owner":          "",
			"lease_expires_at":     nil,
			"lease_heartbeat_at":   nil,
		})
	return result.RowsAffected == 1, result.Error
}

// CancelRun atomically transitions an active/waiting run to cancelled.
func (r *RunRepository) CancelRun(ctx context.Context, id, userID uuid.UUID, reason any) (bool, error) {
	now := time.Now()
	result := database.FromContext(ctx, r.db).
		Model(&model.Run{}).
		Where("id = ? AND user_id = ? AND status IN ?", id, userID, []model.RunStatus{model.RunStatusRunning, model.RunStatusWaitingUser}).
		Updates(map[string]any{
			"status":             model.RunStatusCancelled,
			"error":              reason,
			"completed_at":       now,
			"lease_owner":        "",
			"lease_expires_at":   nil,
			"lease_heartbeat_at": nil,
		})
	return result.RowsAffected == 1, result.Error
}

// FailRun marks a run as failed with an error JSON payload.
func (r *RunRepository) FailRun(ctx context.Context, id, userID uuid.UUID, errJSON any) (bool, error) {
	now := time.Now()
	result := database.FromContext(ctx, r.db).
		Model(&model.Run{}).
		Where("id = ? AND user_id = ? AND status IN ?", id, userID, []model.RunStatus{model.RunStatusRunning, model.RunStatusWaitingUser}).
		Updates(map[string]any{
			"status":             model.RunStatusFailed,
			"error":              errJSON,
			"completed_at":       now,
			"lease_owner":        "",
			"lease_expires_at":   nil,
			"lease_heartbeat_at": nil,
		})
	return result.RowsAffected == 1, result.Error
}

// UpdateAgentConfiguration persists the immutable Agent configuration +
// execution provenance captured from the runtime.agent_configuration event.
func (r *RunRepository) UpdateAgentConfiguration(
	ctx context.Context,
	id uuid.UUID,
	configurationID string,
	configuration datatypes.JSON,
	provenance datatypes.JSON,
) error {
	return database.FromContext(ctx, r.db).
		Model(&model.Run{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"agent_configuration_id": configurationID,
			"agent_configuration":    configuration,
			"execution_provenance":   provenance,
		}).Error
}
