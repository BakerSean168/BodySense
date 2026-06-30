package repository

import (
	"context"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
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

// UpdateStatus updates the status of a job with optional result/error.
func (r *JobRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, result, errData any) error {
	now := time.Now()
	updates := map[string]any{
		"status": status,
	}
	switch status {
	case "running":
		updates["started_at"] = now
	case "succeeded", "failed", "cancelled":
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
