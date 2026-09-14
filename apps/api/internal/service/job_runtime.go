package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// Legal job status transitions. The graph is intentionally finite: callers
// cannot invent a new state by passing an arbitrary string.
var jobTransitions = map[model.JobStatus]map[model.JobStatus]struct{}{
	model.JobStatusPending: {
		model.JobStatusRunning:   {},
		model.JobStatusCancelled: {},
	},
	model.JobStatusRunning: {
		model.JobStatusPending:     {},
		model.JobStatusCompleted:   {},
		model.JobStatusFailed:      {},
		model.JobStatusCancelled:   {},
		model.JobStatusWaitingUser: {},
		model.JobStatusTimedOut:    {},
	},
	model.JobStatusWaitingUser: {
		model.JobStatusRunning:   {},
		model.JobStatusCancelled: {},
	},
	model.JobStatusCompleted: {},
	model.JobStatusFailed:    {},
	model.JobStatusCancelled: {},
	model.JobStatusTimedOut:  {},
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
		Status:         model.JobStatusPending,
		Input:          input,
	}
	event, err := newJobEvent("job.created", map[string]any{"job_type": jobType})
	if err != nil {
		return nil, err
	}
	if err := r.repo.CreateWithEvent(ctx, job, event); err != nil {
		return nil, fmt.Errorf("create job: %w", err)
	}
	return job, nil
}

// TransitionTo moves a job through the finite lifecycle graph. The repository
// performs the compare-and-set and authoritative lifecycle event append in the
// same transaction, so a stale reader cannot overwrite a transition winner.
func (r *JobRuntime) TransitionTo(
	ctx context.Context,
	jobID uuid.UUID,
	newStatus model.JobStatus,
	result, errData any,
) error {
	job, err := r.repo.GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}
	if job == nil {
		return fmt.Errorf("job not found: %s", jobID)
	}
	if !ValidateTransition(job.Status, newStatus) {
		return fmt.Errorf("illegal transition: %s -> %s", job.Status, newStatus)
	}

	event, err := newJobEvent(jobEventTypeForStatus(newStatus), map[string]any{
		"from": job.Status,
		"to":   newStatus,
	})
	if err != nil {
		return err
	}
	transitioned, err := r.repo.TransitionStatus(ctx, jobID, job.Status, newStatus, result, errData, event)
	if err != nil {
		return fmt.Errorf("transition job: %w", err)
	}
	if !transitioned {
		return fmt.Errorf("job transition lost race: %s -> %s", job.Status, newStatus)
	}
	return nil
}

// UpdateProgress stores job progress and appends a stream-compatible progress event.
func (r *JobRuntime) UpdateProgress(ctx context.Context, jobID uuid.UUID, progress any) error {
	if err := r.repo.UpdateProgress(ctx, jobID, progress); err != nil {
		return fmt.Errorf("update progress: %w", err)
	}
	r.appendTelemetryEvent(ctx, jobID, "job.progress", progress)
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
		Status:         model.JobStatusPending,
		Input:          input,
		IdempotencyKey: &idempotencyKey,
		MaxAttempts:    maxAttempts,
	}
	event, err := newJobEvent("job.created", map[string]any{"job_type": jobType})
	if err != nil {
		return nil, false, err
	}
	job, existed, err := r.repo.CreateWithIdempotencyEvent(ctx, job, event)
	if err != nil {
		return nil, false, fmt.Errorf("create job: %w", err)
	}
	return job, existed, nil
}

// ClaimPending atomically transitions a pending job to running. The returned
// job reflects the incremented attempt count.
func (r *JobRuntime) ClaimPending(ctx context.Context, jobID uuid.UUID) (*model.Job, bool, error) {
	event, err := newJobEvent("job.running", map[string]any{
		"from": model.JobStatusPending,
		"to":   model.JobStatusRunning,
	})
	if err != nil {
		return nil, false, err
	}
	claimed, err := r.repo.ClaimPending(ctx, jobID, event)
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
	return job, true, nil
}

func newJobEvent(eventType string, payload any) (*model.JobEvent, error) {
	event := &model.JobEvent{EventType: eventType}
	if payload == nil {
		event.Payload = datatypes.JSON(`{}`)
		return event, nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal job lifecycle event %s: %w", eventType, err)
	}
	event.Payload = datatypes.JSON(data)
	return event, nil
}

// appendTelemetryEvent is intentionally best-effort. Unlike lifecycle events,
// progress telemetry is not the authority for job state and may be dropped if
// its append fails; the durable jobs.progress column remains the source of truth.
func (r *JobRuntime) appendTelemetryEvent(ctx context.Context, jobID uuid.UUID, eventType string, payload any) {
	event, err := newJobEvent(eventType, payload)
	if err != nil {
		return
	}
	event.JobID = jobID
	_ = r.repo.AppendEvent(ctx, event)
}

// ValidateTransition checks if a transition is legal without executing it.
func ValidateTransition(from, to model.JobStatus) bool {
	_, ok := jobTransitions[from][to]
	return ok
}

func jobEventTypeForStatus(status model.JobStatus) string {
	switch status {
	case model.JobStatusCompleted:
		return "job.completed"
	case model.JobStatusFailed:
		return "job.failed"
	case model.JobStatusCancelled:
		return "job.cancelled"
	case model.JobStatusTimedOut:
		return "job.timed_out"
	default:
		return "job." + string(status)
	}
}
