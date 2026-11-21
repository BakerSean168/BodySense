package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

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
