package service

import (
	"context"
	"fmt"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

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
