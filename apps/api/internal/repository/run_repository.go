package repository

import (
	"context"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RunRepository handles database operations for runs.
type RunRepository struct {
	db *gorm.DB
}

// NewRunRepository creates a new RunRepository.
func NewRunRepository(db *gorm.DB) *RunRepository {
	return &RunRepository{db: db}
}

// Create creates a new run.
func (r *RunRepository) Create(ctx context.Context, run *model.Run) error {
	return r.db.WithContext(ctx).Create(run).Error
}

// CreateWithIdempotency inserts a run atomically using the database unique
// constraint on (user_id, request_id). If a run with the same request_id
// already exists, it returns the existing run.
func (r *RunRepository) CreateWithIdempotency(ctx context.Context, run *model.Run) (*model.Run, bool, error) {
	if run.RequestID == "" {
		return run, false, r.Create(ctx, run)
	}

	var existed bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
	err := r.db.WithContext(ctx).
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
	err := r.db.WithContext(ctx).
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
	err := r.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Order("started_at ASC").
		Find(&runs).Error
	return runs, err
}

// UpdateStatus updates the status of a run.
func (r *RunRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	return r.db.WithContext(ctx).
		Model(&model.Run{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// CompleteRun marks a run as completed with usage and provider response ID.
func (r *RunRepository) CompleteRun(ctx context.Context, id, userID uuid.UUID, usage any, providerResponseID string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&model.Run{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]any{
			"status":               "completed",
			"usage":                usage,
			"provider_response_id": providerResponseID,
			"completed_at":         now,
		}).Error
}

// FailRun marks a run as failed with an error JSON payload.
func (r *RunRepository) FailRun(ctx context.Context, id, userID uuid.UUID, errJSON any) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&model.Run{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]any{
			"status":       "failed",
			"error":        errJSON,
			"completed_at": now,
		}).Error
}
