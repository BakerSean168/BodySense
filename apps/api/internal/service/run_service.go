package service

import (
	"context"
	"fmt"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

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

// CompleteRun marks a run as completed with usage stats and provider response ID.
func (s *RunService) CompleteRun(ctx context.Context, id, userID uuid.UUID, usage any, providerResponseID string) error {
	if err := s.runRepo.CompleteRun(ctx, id, userID, usage, providerResponseID); err != nil {
		return fmt.Errorf("complete run: %w", err)
	}
	return nil
}

// FailRun marks a run as failed with an error JSON payload.
func (s *RunService) FailRun(ctx context.Context, id, userID uuid.UUID, errJSON any) error {
	if err := s.runRepo.FailRun(ctx, id, userID, errJSON); err != nil {
		return fmt.Errorf("fail run: %w", err)
	}
	return nil
}
