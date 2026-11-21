package service

import (
	"context"
	"fmt"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

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
