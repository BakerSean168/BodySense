package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
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
	return database.FromContext(ctx, r.db).Create(job).Error
}

// CreateWithEvent persists the initial job row and its authoritative lifecycle
// event in one transaction. Either both become visible or neither does.
func (r *JobRepository) CreateWithEvent(ctx context.Context, job *model.Job, event *model.JobEvent) error {
	return database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(job).Error; err != nil {
			return err
		}
		if event == nil {
			return nil
		}
		event.JobID = job.ID
		return tx.Create(event).Error
	})
}

// CreateWithIdempotency inserts a job atomically using the database unique
// idempotency key. If the key already exists, it returns the existing job.
func (r *JobRepository) CreateWithIdempotency(ctx context.Context, job *model.Job) (*model.Job, bool, error) {
	return r.CreateWithIdempotencyEvent(ctx, job, nil)
}

// CreateWithIdempotencyEvent keeps first creation and its authoritative
// job.created event in the same transaction. Idempotent hits never append a
// duplicate lifecycle event.
func (r *JobRepository) CreateWithIdempotencyEvent(ctx context.Context, job *model.Job, event *model.JobEvent) (*model.Job, bool, error) {
	if job.IdempotencyKey == nil || *job.IdempotencyKey == "" {
		if err := r.CreateWithEvent(ctx, job, event); err != nil {
			return nil, false, err
		}
		return job, false, nil
	}

	var existed bool
	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
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
			if event != nil {
				event.JobID = job.ID
				return tx.Create(event).Error
			}
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

// ClaimPending atomically claims one pending job for execution and increments
// its durable attempt counter. The lifecycle event is committed in the same
// transaction as the state transition.
func (r *JobRepository) ClaimPending(ctx context.Context, id uuid.UUID, event *model.JobEvent) (bool, error) {
	now := time.Now()
	won := false
	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.Job{}).
			Where("id = ? AND status = ? AND attempts < max_attempts", id, model.JobStatusPending).
			Updates(map[string]any{
				"status":      model.JobStatusRunning,
				"attempts":    gorm.Expr("attempts + 1"),
				"started_at":  now,
				"finished_at": nil,
				"updated_at":  now,
			})
		if result.Error != nil || result.RowsAffected != 1 {
			return result.Error
		}
		won = true
		if event != nil {
			event.JobID = id
			if err := tx.Create(event).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return won, nil
}

// GetByID retrieves a job by ID.
func (r *JobRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Job, error) {
	var job model.Job
	err := database.FromContext(ctx, r.db).Where("id = ?", id).First(&job).Error
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
	err := database.FromContext(ctx, r.db).Where("id = ? AND user_id = ?", id, userID).First(&job).Error
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
	err := database.FromContext(ctx, r.db).Where("idempotency_key = ?", key).First(&job).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// ListRecoverable returns pending jobs and stale running jobs for a job type.
func (r *JobRepository) ListRecoverable(ctx context.Context, jobType string, staleRunningBefore time.Time, limit int) ([]model.Job, error) {
	var jobs []model.Job
	err := database.FromContext(ctx, r.db).
		Where(
			"job_type = ? AND (status = ? OR (status = ? AND updated_at < ?))",
			jobType,
			"pending",
			"running",
			staleRunningBefore,
		).
		Order("created_at ASC").
		Limit(limit).
		Find(&jobs).Error
	return jobs, err
}

// TransitionStatus performs one legal lifecycle mutation with compare-and-set
// semantics and appends its authoritative event in the same transaction. A
// false result means another transition already won the race.
func (r *JobRepository) TransitionStatus(
	ctx context.Context,
	id uuid.UUID,
	from, to model.JobStatus,
	resultData, errData any,
	event *model.JobEvent,
) (bool, error) {
	now := time.Now()
	updates := map[string]any{"status": to, "updated_at": now}
	encodedResult, err := encodeJobJSON(resultData)
	if err != nil {
		return false, err
	}
	encodedError, err := encodeJobJSON(errData)
	if err != nil {
		return false, err
	}
	switch to {
	case model.JobStatusRunning:
		updates["started_at"] = now
		updates["finished_at"] = nil
	case model.JobStatusPending:
		updates["started_at"] = nil
		updates["finished_at"] = nil
	case model.JobStatusCompleted, model.JobStatusFailed, model.JobStatusCancelled, model.JobStatusTimedOut:
		updates["finished_at"] = now
	}
	if encodedResult != nil {
		updates["result"] = encodedResult
	}
	if encodedError != nil {
		updates["error"] = encodedError
	}

	won := false
	err = database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		update := tx.Model(&model.Job{}).Where("id = ? AND status = ?", id, from).Updates(updates)
		if update.Error != nil || update.RowsAffected != 1 {
			return update.Error
		}
		won = true
		if event != nil {
			event.JobID = id
			if err := tx.Create(event).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return won, nil
}

func encodeJobJSON(value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	switch typed := value.(type) {
	case datatypes.JSON:
		return typed, nil
	case json.RawMessage:
		return datatypes.JSON(typed), nil
	case []byte:
		return datatypes.JSON(typed), nil
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		return datatypes.JSON(encoded), nil
	}
}

// UpdateProgress stores job progress without changing its lifecycle status.
func (r *JobRepository) UpdateProgress(ctx context.Context, id uuid.UUID, progress any) error {
	return database.FromContext(ctx, r.db).
		Model(&model.Job{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"progress":   progress,
			"updated_at": gorm.Expr("NOW()"),
		}).Error
}

// AppendEvent appends an event to the job_events table.
func (r *JobRepository) AppendEvent(ctx context.Context, event *model.JobEvent) error {
	return database.FromContext(ctx, r.db).Create(event).Error
}

// ListByRunID retrieves all jobs for a run.
func (r *JobRepository) ListByRunID(ctx context.Context, runID uuid.UUID) ([]model.Job, error) {
	var jobs []model.Job
	err := database.FromContext(ctx, r.db).
		Where("run_id = ?", runID).
		Order("created_at ASC").
		Find(&jobs).Error
	return jobs, err
}
