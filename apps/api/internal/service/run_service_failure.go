package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

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
