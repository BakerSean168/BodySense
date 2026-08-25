package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// Legal job status transitions.
var jobTransitions = map[string][]string{
	"pending":      {"running", "cancelled"},
	"running":      {"pending", "completed", "failed", "cancelled", "waiting_user", "timed_out"},
	"waiting_user": {"running", "cancelled"},
	"completed":    {},
	"succeeded":    {},
	"failed":       {},
	"cancelled":    {},
	"timed_out":    {},
}

// JobRuntime manages durable job lifecycle.
type JobRuntime struct {
	repo *repository.JobRepository
}

// NewJobRuntime creates a new JobRuntime.
func NewJobRuntime(repo *repository.JobRepository) *JobRuntime {
	return &JobRuntime{repo: repo}
}

// CreateJob creates a new job in pending status.
func (r *JobRuntime) CreateJob(
	ctx context.Context,
	userID uuid.UUID,
	jobType string,
	input datatypes.JSON,
	runID, conversationID *uuid.UUID,
) (*model.Job, error) {
	job := &model.Job{
		RunID:          runID,
		ConversationID: conversationID,
		UserID:         userID,
		JobType:        jobType,
		Status:         "pending",
		Input:          input,
	}
	if err := r.repo.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("create job: %w", err)
	}

	r.appendEvent(ctx, job.ID, "job.created", map[string]any{"job_type": jobType})
	return job, nil
}

// TransitionTo transitions a job to a new status if the transition is legal.
func (r *JobRuntime) TransitionTo(
	ctx context.Context,
	jobID uuid.UUID,
	newStatus string,
	result, errData any,
) error {
	job, err := r.repo.GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}
	if job == nil {
		return fmt.Errorf("job not found: %s", jobID)
	}

	allowed := jobTransitions[job.Status]
	legal := false
	for _, s := range allowed {
		if s == newStatus {
			legal = true
			break
		}
	}
	if !legal {
		return fmt.Errorf("illegal transition: %s -> %s", job.Status, newStatus)
	}

	if err := r.repo.UpdateStatus(ctx, jobID, newStatus, result, errData); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	r.appendEvent(ctx, jobID, jobEventTypeForStatus(newStatus), map[string]any{
		"from": job.Status,
		"to":   newStatus,
	})
	return nil
}

// UpdateProgress stores job progress and appends a stream-compatible progress event.
func (r *JobRuntime) UpdateProgress(ctx context.Context, jobID uuid.UUID, progress any) error {
	if err := r.repo.UpdateProgress(ctx, jobID, progress); err != nil {
		return fmt.Errorf("update progress: %w", err)
	}
	r.appendEvent(ctx, jobID, "job.progress", progress)
	return nil
}

// GetJob retrieves a job by ID.
func (r *JobRuntime) GetJob(ctx context.Context, jobID uuid.UUID) (*model.Job, error) {
	return r.repo.GetByID(ctx, jobID)
}

// GetJobForUser retrieves a job by ID, scoped to a user for authorization.
func (r *JobRuntime) GetJobForUser(ctx context.Context, jobID, userID uuid.UUID) (*model.Job, error) {
	return r.repo.GetByIDForUser(ctx, jobID, userID)
}

// ListRecoverable returns pending jobs and stale running jobs for a job type.
func (r *JobRuntime) ListRecoverable(ctx context.Context, jobType string, staleRunningAfter time.Duration, limit int) ([]model.Job, error) {
	if limit <= 0 {
		limit = 10
	}
	jobs, err := r.repo.ListRecoverable(ctx, jobType, time.Now().Add(-staleRunningAfter), limit)
	if err != nil {
		return nil, fmt.Errorf("list recoverable jobs: %w", err)
	}
	return jobs, nil
}

// CreateJobWithIdempotency creates a job with an idempotency key.
// If a job with the same key already exists, returns the existing job (idempotent hit).
func (r *JobRuntime) CreateJobWithIdempotency(
	ctx context.Context,
	userID uuid.UUID,
	jobType string,
	input datatypes.JSON,
	idempotencyKey string,
	runID, conversationID *uuid.UUID,
) (*model.Job, bool, error) {
	return r.CreateJobWithIdempotencyAttempts(ctx, userID, jobType, input, idempotencyKey, 1, runID, conversationID)
}

// CreateJobWithIdempotencyAttempts creates a durable job with an explicit,
// finite attempt budget. Existing jobs keep their original immutable budget.
func (r *JobRuntime) CreateJobWithIdempotencyAttempts(
	ctx context.Context,
	userID uuid.UUID,
	jobType string,
	input datatypes.JSON,
	idempotencyKey string,
	maxAttempts int,
	runID, conversationID *uuid.UUID,
) (*model.Job, bool, error) {
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	job := &model.Job{
		RunID:          runID,
		ConversationID: conversationID,
		UserID:         userID,
		JobType:        jobType,
		Status:         "pending",
		Input:          input,
		IdempotencyKey: &idempotencyKey,
		MaxAttempts:    maxAttempts,
	}
	job, existed, err := r.repo.CreateWithIdempotency(ctx, job)
	if err != nil {
		return nil, false, fmt.Errorf("create job: %w", err)
	}
	if existed {
		return job, true, nil
	}

	r.appendEvent(ctx, job.ID, "job.created", map[string]any{"job_type": jobType})
	return job, false, nil
}

// ClaimPending atomically transitions a pending job to running. The returned
// job reflects the incremented attempt count.
func (r *JobRuntime) ClaimPending(ctx context.Context, jobID uuid.UUID) (*model.Job, bool, error) {
	claimed, err := r.repo.ClaimPending(ctx, jobID)
	if err != nil {
		return nil, false, fmt.Errorf("claim pending job: %w", err)
	}
	if !claimed {
		return nil, false, nil
	}
	job, err := r.repo.GetByID(ctx, jobID)
	if err != nil {
		return nil, false, fmt.Errorf("reload claimed job: %w", err)
	}
	if job == nil {
		return nil, false, fmt.Errorf("claimed job disappeared: %s", jobID)
	}
	r.appendEvent(ctx, jobID, "job.running", map[string]any{
		"from":    "pending",
		"to":      "running",
		"attempt": job.Attempts,
	})
	return job, true, nil
}

func (r *JobRuntime) appendEvent(ctx context.Context, jobID uuid.UUID, eventType string, payload any) {
	event := &model.JobEvent{
		JobID:     jobID,
		EventType: eventType,
	}
	if payload != nil {
		if data, err := json.Marshal(payload); err == nil {
			event.Payload = data
		}
	}
	if err := r.repo.AppendEvent(ctx, event); err != nil {
		log.Printf("failed to append job event %s for job %s: %v", eventType, jobID, err)
	}
}

// ValidateTransition checks if a transition is legal without executing it.
func ValidateTransition(from, to string) bool {
	allowed := jobTransitions[from]
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

func jobEventTypeForStatus(status string) string {
	switch status {
	case "completed", "succeeded":
		return "job.completed"
	case "failed":
		return "job.failed"
	case "cancelled":
		return "job.cancelled"
	case "timed_out":
		return "job.timed_out"
	default:
		return "job." + status
	}
}
