package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

var ErrRunTerminal = errors.New("run is already terminal")

// RunService handles run business logic.
type RunService struct {
	runRepo runRepo
}

// NewRunService creates a new RunService.
func NewRunService(runRepo runRepo) *RunService {
	return &RunService{runRepo: runRepo}
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
		Status:         "running",
		Model:          modelStr,
	}
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
		Status:         "running",
		Model:          modelStr,
	}
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

// CancelRun is authorized, idempotent cancellation for running/waiting runs.
// It returns (run, transitioned, error). A second cancellation succeeds with
// transitioned=false; completed/failed runs reject with ErrRunTerminal.
func (s *RunService) CancelRun(ctx context.Context, id, userID uuid.UUID, reason string) (*model.Run, bool, error) {
	run, err := s.GetRunForUser(ctx, id, userID)
	if err != nil || run == nil {
		return run, false, err
	}
	if run.Status == "cancelled" {
		return run, false, nil
	}
	if run.Status == "completed" || run.Status == "failed" {
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
		if latest != nil && latest.Status == "cancelled" {
			return latest, false, nil
		}
		return latest, false, ErrRunTerminal
	}
	run.Status = "cancelled"
	return run, true, nil
}

// FailRun marks a run as failed with an error JSON payload.
func (s *RunService) FailRun(ctx context.Context, id, userID uuid.UUID, errJSON any) error {
	if err := s.runRepo.FailRun(ctx, id, userID, errJSON); err != nil {
		return fmt.Errorf("fail run: %w", err)
	}
	return nil
}

// MarkWaitingUser transitions a run from running to waiting_user.
func (s *RunService) MarkWaitingUser(ctx context.Context, id uuid.UUID) error {
	if err := s.runRepo.UpdateStatus(ctx, id, "waiting_user"); err != nil {
		return fmt.Errorf("mark waiting_user: %w", err)
	}
	return nil
}

// ResumeRunning transitions a run from waiting_user back to running.
func (s *RunService) ResumeRunning(ctx context.Context, id uuid.UUID) error {
	if err := s.runRepo.UpdateStatus(ctx, id, "running"); err != nil {
		return fmt.Errorf("resume running: %w", err)
	}
	return nil
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
