package service

import (
	"context"
	"fmt"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

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
