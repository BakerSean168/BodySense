package repository

import (
	"context"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// JobRepository handles database operations for jobs.
type JobRepository struct {
	db *gorm.DB
}

// NewJobRepository creates a new JobRepository.
func NewJobRepository(db *gorm.DB) *JobRepository {
	return &JobRepository{db: db}
}

// Create creates a new job.
func (r *JobRepository) Create(ctx context.Context, job *model.Job) error {
	return r.db.WithContext(ctx).Create(job).Error
}

// CreateWithIdempotency inserts a job atomically using the database unique
// idempotency key. If the key already exists, it returns the existing job.
func (r *JobRepository) CreateWithIdempotency(ctx context.Context, job *model.Job) (*model.Job, bool, error) {
	if job.IdempotencyKey == nil || *job.IdempotencyKey == "" {
		return job, false, r.Create(ctx, job)
	}

	var existed bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "idempotency_key"}},
			TargetWhere: clause.Where{Exprs: []clause.Expression{
				clause.Expr{SQL: "idempotency_key IS NOT NULL"},
			}},
			DoNothing: true,
		}).Create(job)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			return nil
		}

		existed = true
		var existing model.Job
		if err := tx.Where("idempotency_key = ?", *job.IdempotencyKey).First(&existing).Error; err != nil {
			return err
		}
		*job = existing
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return job, existed, nil
}

// GetByID retrieves a job by ID.
func (r *JobRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Job, error) {
	var job model.Job
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&job).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// GetByIDForUser retrieves a job by ID, scoped to a user for authorization.
func (r *JobRepository) GetByIDForUser(ctx context.Context, id, userID uuid.UUID) (*model.Job, error) {
	var job model.Job
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&job).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// GetByIdempotencyKey retrieves a job by its idempotency key.
func (r *JobRepository) GetByIdempotencyKey(ctx context.Context, key string) (*model.Job, error) {
	var job model.Job
	err := r.db.WithContext(ctx).Where("idempotency_key = ?", key).First(&job).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// UpdateStatus updates the status of a job with optional result/error.
func (r *JobRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, result, errData any) error {
	now := time.Now()
	updates := map[string]any{
		"status": status,
	}
	switch status {
	case "running":
		updates["started_at"] = now
	case "completed", "succeeded", "failed", "cancelled", "timed_out":
		updates["finished_at"] = now
		if result != nil {
			updates["result"] = result
		}
		if errData != nil {
			updates["error"] = errData
		}
	}
	return r.db.WithContext(ctx).
		Model(&model.Job{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// UpdateProgress stores job progress without changing its lifecycle status.
func (r *JobRepository) UpdateProgress(ctx context.Context, id uuid.UUID, progress any) error {
	return r.db.WithContext(ctx).
		Model(&model.Job{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"progress":   progress,
			"updated_at": gorm.Expr("NOW()"),
		}).Error
}

// AppendEvent appends an event to the job_events table.
func (r *JobRepository) AppendEvent(ctx context.Context, event *model.JobEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}

// ListByRunID retrieves all jobs for a run.
func (r *JobRepository) ListByRunID(ctx context.Context, runID uuid.UUID) ([]model.Job, error) {
	var jobs []model.Job
	err := r.db.WithContext(ctx).
		Where("run_id = ?", runID).
		Order("created_at ASC").
		Find(&jobs).Error
	return jobs, err
}
