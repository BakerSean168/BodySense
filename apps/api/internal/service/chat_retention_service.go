package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// ChatRetentionPruneMaxAttempts bounds how often one prune job retries a
// failed checkpoint cleanup or a stale plan before it is marked failed.
const ChatRetentionPruneMaxAttempts = 5

// ErrCheckpointCleanupFailed wraps any failure to obtain a cleanup receipt
// from the ai-service. Nothing was deleted in Go and R did not move.
var ErrCheckpointCleanupFailed = errors.New("checkpoint cleanup failed")

// CheckpointCleaner removes the LangGraph checkpoint copies of pruned Runs.
type CheckpointCleaner interface {
	CleanupConsultationCheckpoints(
		ctx context.Context,
		conversationID, owner uuid.UUID,
		runIDs []uuid.UUID,
	) (*model.CheckpointCleanupReceipt, error)
}

type checkpointCleanupRequest struct {
	ConversationID string   `json:"conversation_id"`
	UserID         string   `json:"user_id"`
	PrunedRunIDs   []string `json:"pruned_run_ids"`
}

// CleanupConsultationCheckpoints asks the ai-service to drop the given Runs
// from the consultation thread (thread id = Conversation id) and return a
// receipt read back from checkpoint storage.
func (c *AIClient) CleanupConsultationCheckpoints(
	ctx context.Context,
	conversationID, owner uuid.UUID,
	runIDs []uuid.UUID,
) (*model.CheckpointCleanupReceipt, error) {
	req := checkpointCleanupRequest{
		ConversationID: conversationID.String(),
		UserID:         owner.String(),
		PrunedRunIDs:   make([]string, len(runIDs)),
	}
	for i, id := range runIDs {
		req.PrunedRunIDs[i] = id.String()
	}
	raw, err := c.callJSON(ctx, "/runtime/threads/"+conversationID.String()+"/checkpoints/cleanup", req)
	if err != nil {
		return nil, err
	}
	var receipt model.CheckpointCleanupReceipt
	if err := json.Unmarshal(raw, &receipt); err != nil {
		return nil, fmt.Errorf("decode checkpoint cleanup receipt: %w", err)
	}
	return &receipt, nil
}

// ChatRetentionService runs prune passes (PC-RET-001). A pass is planned under
// the Conversation lock, the ai-service cleans the checkpoint copies of the
// planned Runs, and only a verified receipt lets Go delete raw messages and
// advance R. Any failure before the commit leaves every cursor where it was.
type ChatRetentionService struct {
	repo    *repository.ChatRetentionRepository
	cleaner CheckpointCleaner
	jobs    *JobRuntime
	// budget is the context window token budget (model.ChatContextTokenBudget).
	budget int
}

func NewChatRetentionService(repo *repository.ChatRetentionRepository, cleaner CheckpointCleaner, jobs *JobRuntime) *ChatRetentionService {
	return &ChatRetentionService{repo: repo, cleaner: cleaner, jobs: jobs, budget: model.DefaultChatContextTokenBudget}
}

// WithContextTokenBudget sets the context window budget; values below
// model.MinChatContextTokenBudget keep the default.
func (s *ChatRetentionService) WithContextTokenBudget(budget int) *ChatRetentionService {
	if budget >= model.MinChatContextTokenBudget {
		s.budget = budget
	}
	return s
}

// ContextTokenBudget is the budget this service applies.
func (s *ChatRetentionService) ContextTokenBudget() int {
	return s.budget
}

// ReconcileWindow advances P over every settled interaction, moves C so the
// retained raw messages fit the token budget, and enqueues a prune job when
// the prunable boundary min(C - 1, P) is past R. The job key carries P and C,
// so repeating the call for an unchanged window enqueues nothing new.
func (s *ChatRetentionService) ReconcileWindow(ctx context.Context, owner uuid.UUID) (*model.ChatRetentionCursor, *model.Job, error) {
	cursor, _, err := s.repo.ReconcileWindow(ctx, owner, s.budget)
	if err != nil {
		return nil, nil, err
	}
	if min(cursor.ContextStartSeq-1, cursor.ProcessedThroughSeq) <= cursor.PrunedThroughSeq {
		return cursor, nil, nil
	}
	job, err := s.EnqueuePrune(ctx, owner, fmt.Sprintf("window:%d:%d", cursor.ProcessedThroughSeq, cursor.ContextStartSeq))
	if err != nil {
		return cursor, nil, err
	}
	return cursor, job, nil
}

// OnRunSettled is the settle trigger: the consultation runtime calls it once
// a Run and its messages reached their final state (completed, failed,
// cancelled, or parked on Ask User). It never fails the caller; a missed
// trigger is caught up by the next settlement because reconciliation always
// looks at the whole live transcript.
func (s *ChatRetentionService) OnRunSettled(ctx context.Context, owner uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if _, _, err := s.ReconcileWindow(ctx, owner); err != nil && !errors.Is(err, model.ErrRetentionNotFound) {
		log.Printf("chat retention window reconcile for user %s: %v", owner, err)
	}
}

// OnConversationSettled is OnRunSettled for callers that only know the
// Conversation, such as the Ask User expiry worker: an expired or cancelled
// interaction was what kept its Run unsettled, so the window may move now.
func (s *ChatRetentionService) OnConversationSettled(ctx context.Context, conversationID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	owner, err := s.repo.OwnerOf(ctx, conversationID)
	if err != nil {
		if !errors.Is(err, model.ErrRetentionNotFound) {
			log.Printf("chat retention owner of conversation %s: %v", conversationID, err)
		}
		return
	}
	s.OnRunSettled(ctx, owner)
}

// ContextWindow is the raw-history window the AI runtime may read.
func (s *ChatRetentionService) ContextWindow(ctx context.Context, owner, conversationID uuid.UUID) (*model.ChatContextWindow, error) {
	return s.repo.ContextWindow(ctx, owner, conversationID)
}

// Prune runs one pass now and records its receipt on a job of its own.
func (s *ChatRetentionService) Prune(ctx context.Context, owner uuid.UUID) (*model.ChatRetentionPruneReceipt, error) {
	return s.prune(ctx, owner, uuid.Nil)
}

func (s *ChatRetentionService) prune(ctx context.Context, owner, jobID uuid.UUID) (*model.ChatRetentionPruneReceipt, error) {
	plan, replay, err := s.repo.PlanPrune(ctx, owner)
	if err != nil {
		return nil, err
	}
	if replay != nil {
		return replay, nil
	}
	cleanup, err := s.cleaner.CleanupConsultationCheckpoints(ctx, plan.ConversationID, owner, plan.RunIDs)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCheckpointCleanupFailed, err)
	}
	return s.repo.CommitPrune(ctx, owner, plan, cleanup, jobID)
}

// EnqueuePrune asks for a pass past the current pruned cursor. trigger names
// why (for example the Run that just settled); the same trigger at the same
// cursor is one job.
func (s *ChatRetentionService) EnqueuePrune(ctx context.Context, owner uuid.UUID, trigger string) (*model.Job, error) {
	if trigger == "" {
		return nil, errors.New("prune trigger is required")
	}
	cursor, err := s.repo.Get(ctx, owner)
	if err != nil {
		return nil, err
	}
	input, err := json.Marshal(map[string]any{"fromPrunedSeq": cursor.PrunedThroughSeq, "trigger": trigger})
	if err != nil {
		return nil, err
	}
	conversationID := cursor.ConversationID
	job, _, err := s.jobs.CreateJobWithIdempotencyAttempts(ctx, owner, model.ChatRetentionPruneJobType,
		datatypes.JSON(input), repository.PruneJobKey(conversationID, cursor.PrunedThroughSeq, trigger),
		ChatRetentionPruneMaxAttempts, nil, &conversationID)
	return job, err
}

// StartPruneWorker polls for pending prune jobs (and running ones whose
// worker died) until ctx ends.
func (s *ChatRetentionService) StartPruneWorker(ctx context.Context, pollInterval, staleRunningAfter time.Duration) {
	go func() {
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		for {
			if _, err := s.RunPendingPrunes(ctx, 10, staleRunningAfter); err != nil && ctx.Err() == nil {
				log.Printf("chat retention prune worker: %v", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

// RunPendingPrunes claims and executes up to limit recoverable prune jobs and
// returns how many it executed.
func (s *ChatRetentionService) RunPendingPrunes(ctx context.Context, limit int, staleRunningAfter time.Duration) (int, error) {
	jobs, err := s.jobs.ListRecoverable(ctx, model.ChatRetentionPruneJobType, staleRunningAfter, limit)
	if err != nil {
		return 0, err
	}
	executed := 0
	for _, job := range jobs {
		if job.Status == model.JobStatusRunning {
			// Its worker stopped heartbeating; hand it back to the queue.
			if err := s.jobs.TransitionTo(ctx, job.ID, model.JobStatusPending, nil, map[string]string{"error": "stale running prune job"}); err != nil {
				continue
			}
		}
		claimed, ok, err := s.jobs.ClaimPending(ctx, job.ID)
		if err != nil {
			return executed, err
		}
		if !ok {
			continue
		}
		executed++
		if err := s.execute(ctx, claimed); err != nil {
			return executed, err
		}
	}
	return executed, nil
}

// execute runs one claimed job. A committed pass completes the job inside the
// commit transaction; every other outcome is recorded here.
func (s *ChatRetentionService) execute(ctx context.Context, job *model.Job) error {
	receipt, err := s.prune(ctx, job.UserID, job.ID)
	switch {
	case err == nil && !receipt.Replayed:
		return nil
	case err == nil:
		// Nothing left to prune (or another pass got there first).
		return s.jobs.TransitionTo(ctx, job.ID, model.JobStatusCompleted,
			map[string]any{"outcome": "nothing_to_prune", "cursor": receipt.PrunedThroughSeq}, nil)
	case errors.Is(err, model.ErrRetentionBusy):
		// An unsettled Run owns the thread. The trigger that settles it asks
		// again; this job does not spin on it.
		return s.jobs.TransitionTo(ctx, job.ID, model.JobStatusCompleted,
			map[string]any{"outcome": "deferred", "reason": "unsettled_run"}, nil)
	case retryablePruneError(err) && job.Attempts < job.MaxAttempts:
		return s.jobs.TransitionTo(ctx, job.ID, model.JobStatusPending, nil,
			map[string]any{"error": err.Error(), "attempt": job.Attempts})
	default:
		return s.jobs.TransitionTo(ctx, job.ID, model.JobStatusFailed, nil,
			map[string]any{"error": err.Error(), "attempt": job.Attempts})
	}
}

func retryablePruneError(err error) bool {
	switch {
	case errors.Is(err, model.ErrRetentionNotFound),
		errors.Is(err, model.ErrRetentionOwnerAnomaly),
		errors.Is(err, model.ErrRetentionCursorInvalid):
		return false
	default:
		// Cleanup failures, stale plans and revision races heal on retry.
		return true
	}
}
