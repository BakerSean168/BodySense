package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

var ErrRunTerminal = errors.New("run is already terminal")

// runLeaseDuration bounds how long a run may sit in the running state before
// the runtime is assumed dead and the run is reclaimed. It is comfortably
// larger than the SSE timeout so live runs never expire their lease.
const runLeaseDuration = 30 * time.Minute
const runExecutionLostErrorJSON = `{"message":"run execution lost; lease expired"}`

// RunService handles run business logic.
type runLifecycleEventStore interface {
	Flush(ctx context.Context) error
	PersistPreparedMilestone(ctx context.Context, conversationID, runID uuid.UUID, turnID *uuid.UUID, event dto.StreamEvent) error
	PersistOutOfBandMilestone(ctx context.Context, conversationID, runID uuid.UUID, turnID *uuid.UUID, channel, eventType string, ids dto.StreamEventIDs, payload any) (dto.StreamEvent, error)
}

type runTransactionManager interface {
	WithinTransaction(ctx context.Context, fn func(context.Context) error) error
}

type RunService struct {
	runRepo         runRepo
	leaseOwner      string
	lifecycleEvents runLifecycleEventStore
	transactions    runTransactionManager
}

// NewRunService creates a new RunService.
func NewRunService(runRepo runRepo, leaseOwners ...string) *RunService {
	owner := uuid.NewString()
	if len(leaseOwners) > 0 && leaseOwners[0] != "" {
		owner = leaseOwners[0]
	}
	return &RunService{runRepo: runRepo, leaseOwner: owner}
}

// WithLifecycleEvents binds the transactional public lifecycle log. Terminal
// Run mutations use this coordinator so durable state and the authoritative
// run.completed/failed/cancelled event share one commit boundary.
func (s *RunService) WithLifecycleEvents(events runLifecycleEventStore, transactions runTransactionManager) *RunService {
	s.lifecycleEvents = events
	s.transactions = transactions
	return s
}

func (s *RunService) requireLifecycleCoordinator() error {
	if s.lifecycleEvents == nil || s.transactions == nil {
		return errors.New("run lifecycle coordinator is not configured")
	}
	return nil
}

// CreateRun creates a new LLM inference run.
func (s *RunService) CreateRun(
	ctx context.Context,
	conversationID uuid.UUID,
	turnID uuid.UUID,
	requestID string,
	userID uuid.UUID,
	modelStr string,
) (*model.Run, error) {
	run := &model.Run{
		ID:             uuid.New(),
		ConversationID: conversationID,
		TurnID:         turnID,
		RequestID:      requestID,
		UserID:         userID,
		Status:         model.RunStatusRunning,
		Model:          modelStr,
		LeaseOwner:     s.leaseOwner,
	}
	run.LeaseExpiresAt = leaseExpiry()
	if err := s.runRepo.Create(ctx, run); err != nil {
		return nil, fmt.Errorf("create run: %w", err)
	}
	return run, nil
}

// CreateRunWithIdempotency creates a new run, or returns the existing one if a run
// with the same (user_id, request_id) already exists. Uses database-level unique
// constraint for atomicity — safe under concurrent requests with the same requestID.
func (s *RunService) CreateRunWithIdempotency(
	ctx context.Context,
	conversationID uuid.UUID,
	turnID uuid.UUID,
	requestID string,
	userID uuid.UUID,
	modelStr string,
) (*model.Run, bool, error) {
	run := &model.Run{
		ID:             uuid.New(),
		ConversationID: conversationID,
		TurnID:         turnID,
		RequestID:      requestID,
		UserID:         userID,
		Status:         model.RunStatusRunning,
		Model:          modelStr,
		LeaseOwner:     s.leaseOwner,
	}
	run.LeaseExpiresAt = leaseExpiry()
	result, existed, err := s.runRepo.CreateWithIdempotency(ctx, run)
	if err != nil {
		return nil, false, fmt.Errorf("create run: %w", err)
	}
	return result, existed, nil
}

// CheckIdempotency checks if a run with the given requestID already exists for the user.
// Returns the existing run (if any) and whether this is a duplicate request.
func (s *RunService) CheckIdempotency(ctx context.Context, userID uuid.UUID, requestID string) (*model.Run, bool, error) {
	existing, err := s.runRepo.GetByRequestID(ctx, userID, requestID)
	if err != nil {
		return nil, false, fmt.Errorf("check idempotency: %w", err)
	}
	if existing != nil {
		return existing, true, nil
	}
	return nil, false, nil
}

// GetRunForUser returns a run only when it belongs to the requesting user.
// Consultation HITL resume uses this to pin continuation to the immutable
// configuration recorded on the interrupted source run.
func (s *RunService) GetRunForUser(ctx context.Context, id, userID uuid.UUID) (*model.Run, error) {
	run, err := s.runRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get run: %w", err)
	}
	if run == nil || run.UserID != userID {
		return nil, nil
	}
	return run, nil
}

// CompleteRun marks a run as completed unless another terminal transition won first.
func (s *RunService) CompleteRun(ctx context.Context, id, userID uuid.UUID, usage any, providerResponseID string) error {
	completed, err := s.TryCompleteRun(ctx, id, userID, usage, providerResponseID)
	if err != nil {
		return err
	}
	if !completed {
		return ErrRunTerminal
	}
	return nil
}

// TryCompleteRun atomically completes an active run and reports whether this
// caller won the terminal-state race.
func (s *RunService) TryCompleteRun(ctx context.Context, id, userID uuid.UUID, usage any, providerResponseID string) (bool, error) {
	completed, err := s.runRepo.TryCompleteRun(ctx, id, userID, usage, providerResponseID)
	if err != nil {
		return false, fmt.Errorf("complete run: %w", err)
	}
	return completed, nil
}

// TryCompleteRunWithEvent atomically commits an active Run as completed and
// persists the already-sequenced public run.completed milestone. Pending text
// deltas are flushed before opening the transaction so rollback cannot lose a
// buffer batch that was only provisionally written.
func (s *RunService) TryCompleteRunWithEvent(
	ctx context.Context,
	run *model.Run,
	userID uuid.UUID,
	usage any,
	providerResponseID string,
	event dto.StreamEvent,
) (bool, error) {
	if run == nil {
		return false, errors.New("complete run with event: run is nil")
	}
	if err := s.requireLifecycleCoordinator(); err != nil {
		return false, err
	}
	if err := s.lifecycleEvents.Flush(ctx); err != nil {
		return false, fmt.Errorf("flush runtime events before run completion: %w", err)
	}
	committed := false
	err := s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		won, err := s.runRepo.TryCompleteRun(txCtx, run.ID, userID, usage, providerResponseID)
		if err != nil {
			return err
		}
		if !won {
			return nil
		}
		turnID := run.TurnID
		if err := s.lifecycleEvents.PersistPreparedMilestone(txCtx, run.ConversationID, run.ID, &turnID, event); err != nil {
			return err
		}
		committed = true
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("complete run with lifecycle event: %w", err)
	}
	return committed, nil
}

// CompleteRunOutOfBandWithEvent atomically closes a non-live active Run and
// appends its public run.completed milestone with a database-allocated sequence.
// HITL resume uses this for the interrupted source run before creating the
// continuation run.
func (s *RunService) CompleteRunOutOfBandWithEvent(
	ctx context.Context,
	run *model.Run,
	userID uuid.UUID,
) (bool, error) {
	if run == nil {
		return false, errors.New("complete out-of-band run: run is nil")
	}
	if err := s.requireLifecycleCoordinator(); err != nil {
		return false, err
	}
	if err := s.lifecycleEvents.Flush(ctx); err != nil {
		return false, fmt.Errorf("flush runtime events before out-of-band completion: %w", err)
	}
	committed := false
	err := s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		won, err := s.runRepo.TryCompleteRun(txCtx, run.ID, userID, nil, "")
		if err != nil {
			return err
		}
		if !won {
			return nil
		}
		turnID := run.TurnID
		_, err = s.lifecycleEvents.PersistOutOfBandMilestone(
			txCtx, run.ConversationID, run.ID, &turnID,
			"run", "run.completed",
			dto.StreamEventIDs{ConversationID: run.ConversationID.String(), RunID: run.ID.String(), TurnID: run.TurnID.String()},
			map[string]any{"status": "completed"},
		)
		if err != nil {
			return err
		}
		committed = true
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("complete out-of-band run with lifecycle event: %w", err)
	}
	return committed, nil
}

// FailRunWithEvent is the failed-state counterpart to TryCompleteRunWithEvent.
func (s *RunService) FailRunWithEvent(
	ctx context.Context,
	run *model.Run,
	userID uuid.UUID,
	errJSON any,
	event dto.StreamEvent,
) (bool, error) {
	if run == nil {
		return false, errors.New("fail run with event: run is nil")
	}
	if err := s.requireLifecycleCoordinator(); err != nil {
		return false, err
	}
	if err := s.lifecycleEvents.Flush(ctx); err != nil {
		return false, fmt.Errorf("flush runtime events before run failure: %w", err)
	}
	committed := false
	err := s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		won, err := s.runRepo.FailRun(txCtx, run.ID, userID, errJSON)
		if err != nil {
			return err
		}
		if !won {
			return nil
		}
		turnID := run.TurnID
		if err := s.lifecycleEvents.PersistPreparedMilestone(txCtx, run.ConversationID, run.ID, &turnID, event); err != nil {
			return err
		}
		committed = true
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("fail run with lifecycle event: %w", err)
	}
	return committed, nil
}

// CancelRun is authorized, idempotent cancellation for running/waiting runs.
// It returns (run, transitioned, error). A second cancellation succeeds with
// transitioned=false; completed/failed runs reject with ErrRunTerminal.
func (s *RunService) CancelRun(ctx context.Context, id, userID uuid.UUID, reason string) (*model.Run, bool, error) {
	run, err := s.GetRunForUser(ctx, id, userID)
	if err != nil || run == nil {
		return run, false, err
	}
	if run.Status == model.RunStatusCancelled {
		return run, false, nil
	}
	if run.Status == model.RunStatusCompleted || run.Status == model.RunStatusFailed {
		return run, false, ErrRunTerminal
	}
	if reason == "" {
		reason = "cancelled_by_user"
	}
	transitioned, err := s.runRepo.CancelRun(ctx, id, userID, map[string]any{"reason": reason})
	if err != nil {
		return nil, false, fmt.Errorf("cancel run: %w", err)
	}
	if !transitioned {
		latest, getErr := s.GetRunForUser(ctx, id, userID)
		if getErr != nil {
			return nil, false, getErr
		}
		if latest != nil && latest.Status == model.RunStatusCancelled {
			return latest, false, nil
		}
		return latest, false, ErrRunTerminal
	}
	run.Status = model.RunStatusCancelled
	return run, true, nil
}

// CancelRunWithEvent atomically cancels an active/waiting Run and allocates its
// durable run.cancelled sequence under the same transaction. Live SSE may close
// immediately; browser recovery treats the persisted run.cancelled milestone as
// terminal, so cancellation does not depend on a second best-effort write.
func (s *RunService) CancelRunWithEvent(ctx context.Context, id, userID uuid.UUID, reason string) (*model.Run, bool, error) {
	if err := s.requireLifecycleCoordinator(); err != nil {
		return nil, false, err
	}
	run, err := s.GetRunForUser(ctx, id, userID)
	if err != nil || run == nil {
		return run, false, err
	}
	if run.Status == model.RunStatusCancelled {
		return run, false, nil
	}
	if run.Status.IsTerminal() {
		return run, false, ErrRunTerminal
	}
	if reason == "" {
		reason = "cancelled_by_user"
	}
	if err := s.lifecycleEvents.Flush(ctx); err != nil {
		return nil, false, fmt.Errorf("flush runtime events before cancellation: %w", err)
	}
	committed := false
	err = s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		won, err := s.runRepo.CancelRun(txCtx, id, userID, map[string]any{"reason": reason})
		if err != nil {
			return err
		}
		if !won {
			return nil
		}
		turnID := run.TurnID
		_, err = s.lifecycleEvents.PersistOutOfBandMilestone(
			txCtx, run.ConversationID, run.ID, &turnID,
			"run", "run.cancelled",
			dto.StreamEventIDs{ConversationID: run.ConversationID.String(), RunID: run.ID.String(), TurnID: run.TurnID.String()},
			map[string]any{"status": "cancelled", "reason": reason},
		)
		if err != nil {
			return err
		}
		committed = true
		return nil
	})
	if err != nil {
		return nil, false, fmt.Errorf("cancel run with lifecycle event: %w", err)
	}
	if !committed {
		latest, getErr := s.GetRunForUser(ctx, id, userID)
		if getErr != nil {
			return nil, false, getErr
		}
		if latest != nil && latest.Status == model.RunStatusCancelled {
			return latest, false, nil
		}
		return latest, false, ErrRunTerminal
	}
	run.Status = model.RunStatusCancelled
	return run, true, nil
}

// FailRun marks a run as failed only if it is still active.
func (s *RunService) FailRun(ctx context.Context, id, userID uuid.UUID, errJSON any) error {
	failed, err := s.runRepo.FailRun(ctx, id, userID, errJSON)
	if err != nil {
		return fmt.Errorf("fail run: %w", err)
	}
	if !failed {
		return ErrRunTerminal
	}
	return nil
}

// MarkWaitingUser transitions exactly running -> waiting_user.
func (s *RunService) MarkWaitingUser(ctx context.Context, id uuid.UUID) error {
	transitioned, err := s.runRepo.MarkWaitingUser(ctx, id)
	if err != nil {
		return fmt.Errorf("mark waiting_user: %w", err)
	}
	if !transitioned {
		run, loadErr := s.runRepo.GetByID(ctx, id)
		if loadErr != nil {
			return fmt.Errorf("reload waiting_user run: %w", loadErr)
		}
		if run != nil && run.Status == model.RunStatusWaitingUser {
			return nil
		}
		return ErrRunTerminal
	}
	return nil
}

// FailWaitingUser performs the internal waiting_user -> failed CAS used by the
// interaction lifecycle coordinator. It intentionally does not write a public
// event itself; the caller owns the surrounding transaction and persists the
// matching interaction.expired + run.failed milestones before commit.
func (s *RunService) FailWaitingUser(ctx context.Context, id uuid.UUID, errJSON any) (bool, error) {
	failed, err := s.runRepo.FailWaitingUser(ctx, id, errJSON)
	if err != nil {
		return false, fmt.Errorf("fail waiting_user run: %w", err)
	}
	return failed, nil
}

// ResumeRunning transitions exactly waiting_user -> running and establishes a
// fresh lease owned by this API process.
func (s *RunService) ResumeRunning(ctx context.Context, id uuid.UUID) error {
	expires := time.Now().Add(runLeaseDuration)
	transitioned, err := s.runRepo.ResumeRunning(ctx, id, s.leaseOwner, expires)
	if err != nil {
		return fmt.Errorf("resume running: %w", err)
	}
	if !transitioned {
		run, loadErr := s.runRepo.GetByID(ctx, id)
		if loadErr != nil {
			return fmt.Errorf("reload running run: %w", loadErr)
		}
		if run != nil && run.Status == model.RunStatusRunning {
			return nil
		}
		return ErrRunTerminal
	}
	return nil
}

const runLeaseHeartbeatInterval = 1 * time.Minute

// StartLeaseHeartbeat renews a running execution until the context ends. The
// owner-bound update makes a stale process unable to resurrect a reclaimed run.
func (s *RunService) StartLeaseHeartbeat(ctx context.Context, id, userID uuid.UUID) func() {
	heartbeatCtx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(runLeaseHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case now := <-ticker.C:
				expires := now.Add(runLeaseDuration)
				alive, err := s.runRepo.RenewLease(heartbeatCtx, id, userID, s.leaseOwner, expires, now)
				if err != nil || !alive {
					return
				}
			}
		}
	}()
	return cancel
}

// ReconcileExpiredRuns atomically commits execution_lost and run.failed for
// every expired Run. Listing is advisory; FailExpiredRun is the terminal CAS,
// so concurrent API instances cannot both win.
func (s *RunService) ReconcileExpiredRuns(ctx context.Context, limit int) ([]model.Run, error) {
	if err := s.requireLifecycleCoordinator(); err != nil {
		return nil, err
	}
	if err := s.lifecycleEvents.Flush(ctx); err != nil {
		return nil, fmt.Errorf("flush runtime events before lease reconciliation: %w", err)
	}
	now := time.Now()
	candidates, err := s.runRepo.ListExpiredRuns(ctx, now, limit)
	if err != nil {
		return nil, fmt.Errorf("list expired runs: %w", err)
	}
	reclaimed := make([]model.Run, 0, len(candidates))
	for i := range candidates {
		candidate := candidates[i]
		committed := false
		err := s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
			won, err := s.runRepo.FailExpiredRun(txCtx, candidate.ID, now, datatypes.JSON([]byte(runExecutionLostErrorJSON)))
			if err != nil {
				return err
			}
			if !won {
				return nil
			}
			candidate.Status = model.RunStatusFailed
			candidate.Error = datatypes.JSON([]byte(runExecutionLostErrorJSON))
			candidate.CompletedAt = &now
			candidate.LeaseOwner = ""
			candidate.LeaseExpiresAt = nil
			candidate.LeaseHeartbeatAt = nil
			turnID := candidate.TurnID
			_, err = s.lifecycleEvents.PersistOutOfBandMilestone(
				txCtx, candidate.ConversationID, candidate.ID, &turnID,
				"run", "run.failed",
				dto.StreamEventIDs{ConversationID: candidate.ConversationID.String(), RunID: candidate.ID.String(), TurnID: candidate.TurnID.String()},
				map[string]any{"status": "failed", "reason": "execution_lost"},
			)
			if err != nil {
				return err
			}
			committed = true
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("reconcile expired run %s: %w", candidate.ID, err)
		}
		if committed {
			reclaimed = append(reclaimed, candidate)
		}
	}
	return reclaimed, nil
}

// UpdateAgentConfiguration persists the immutable Agent configuration +
// execution provenance captured from the runtime.agent_configuration event.
func (s *RunService) UpdateAgentConfiguration(
	ctx context.Context,
	id uuid.UUID,
	configurationID string,
	configuration datatypes.JSON,
	provenance datatypes.JSON,
) error {
	if err := s.runRepo.UpdateAgentConfiguration(ctx, id, configurationID, configuration, provenance); err != nil {
		return fmt.Errorf("update run agent configuration: %w", err)
	}
	return nil
}

// leaseExpiry returns the timestamp at which a newly-created run's lease
// expires. The runtime extends the lease as long as the run is actively
// streaming; an un-renewed expiry means the owning process died and the run
// may be reclaimed.
func leaseExpiry() *time.Time {
	expires := time.Now().Add(runLeaseDuration)
	return &expires
}
