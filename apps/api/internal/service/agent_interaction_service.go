package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

var (
	ErrInteractionNotFound = errors.New("interaction not found")
	ErrInteractionConflict = errors.New("interaction answer conflicts with existing answer")
	ErrInteractionClosed   = errors.New("interaction is not pending")
)

// AgentInteractionService handles user interaction persistence and resume.
type AgentInteractionService struct {
	repo    agentInteractionRepo
	runRepo runStatusRepo
}

type agentInteractionRepo interface {
	CreatePending(ctx context.Context, interaction *model.AgentInteraction) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.AgentInteraction, error)
	GetByRunAndToolCall(ctx context.Context, runID uuid.UUID, toolCallID string) (*model.AgentInteraction, error)
	MarkAnswered(ctx context.Context, id uuid.UUID, answer any) (bool, error)
	CancelPending(ctx context.Context, id uuid.UUID) (bool, error)
	ListPendingByConversation(ctx context.Context, conversationID uuid.UUID) ([]model.AgentInteraction, error)
}

type runStatusRepo interface {
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
}

// NewAgentInteractionService creates a new AgentInteractionService.
func NewAgentInteractionService(
	repo agentInteractionRepo,
	runRepo runStatusRepo,
) *AgentInteractionService {
	return &AgentInteractionService{
		repo:    repo,
		runRepo: runRepo,
	}
}

// CreatePendingInteraction creates a pending interaction from an ask_user event.
func (s *AgentInteractionService) CreatePendingInteraction(
	ctx context.Context,
	runID, conversationID uuid.UUID,
	toolCallID string,
	question datatypes.JSON,
) (*model.AgentInteraction, error) {
	interaction := &model.AgentInteraction{
		RunID:          runID,
		ConversationID: conversationID,
		ToolCallID:     toolCallID,
		ToolName:       "ask_user",
		Question:       question,
		Status:         "pending",
	}
	if err := s.repo.CreatePending(ctx, interaction); err != nil {
		return nil, fmt.Errorf("create pending interaction: %w", err)
	}
	created, err := s.repo.GetByRunAndToolCall(ctx, runID, toolCallID)
	if err != nil {
		return nil, fmt.Errorf("load pending interaction: %w", err)
	}
	if created == nil {
		return nil, fmt.Errorf("load pending interaction: %w", ErrInteractionNotFound)
	}

	if err := s.runRepo.UpdateStatus(ctx, runID, "waiting_user"); err != nil {
		return nil, fmt.Errorf("mark run waiting_user: %w", err)
	}

	return created, nil
}

// ResumeInteraction marks an interaction as answered. It is idempotent for
// repeated submissions with the same answer and rejects conflicting answers.
func (s *AgentInteractionService) ResumeInteraction(
	ctx context.Context,
	interactionID uuid.UUID,
	answer datatypes.JSON,
) error {
	interaction, err := s.repo.GetByID(ctx, interactionID)
	if err != nil {
		return fmt.Errorf("get interaction: %w", err)
	}
	if interaction == nil {
		return ErrInteractionNotFound
	}
	if interaction.Status == "answered" {
		if jsonEqual(interaction.Answer, answer) {
			return nil
		}
		return ErrInteractionConflict
	}
	if interaction.Status != "pending" {
		return fmt.Errorf("%w: %s", ErrInteractionClosed, interaction.Status)
	}

	updated, err := s.repo.MarkAnswered(ctx, interactionID, answer)
	if err != nil {
		return fmt.Errorf("mark answered: %w", err)
	}
	if !updated {
		latest, latestErr := s.repo.GetByID(ctx, interactionID)
		if latestErr != nil {
			return fmt.Errorf("reload interaction after answer race: %w", latestErr)
		}
		if latest != nil && latest.Status == "answered" && jsonEqual(latest.Answer, answer) {
			return nil
		}
		return ErrInteractionConflict
	}

	return nil
}

// CancelInteraction marks a pending interaction as cancelled.
func (s *AgentInteractionService) CancelInteraction(ctx context.Context, interactionID uuid.UUID) error {
	interaction, err := s.repo.GetByID(ctx, interactionID)
	if err != nil {
		return fmt.Errorf("get interaction: %w", err)
	}
	if interaction == nil {
		return ErrInteractionNotFound
	}
	if interaction.Status == "cancelled" {
		return nil
	}
	if interaction.Status != "pending" {
		return fmt.Errorf("%w: %s", ErrInteractionClosed, interaction.Status)
	}
	updated, err := s.repo.CancelPending(ctx, interactionID)
	if err != nil {
		return fmt.Errorf("cancel interaction: %w", err)
	}
	if !updated {
		return ErrInteractionClosed
	}
	return nil
}

// GetPendingInteractions returns pending interactions for a conversation.
func (s *AgentInteractionService) GetPendingInteractions(
	ctx context.Context,
	conversationID uuid.UUID,
) ([]model.AgentInteraction, error) {
	return s.repo.ListPendingByConversation(ctx, conversationID)
}

// GetInteractionByID returns an interaction by its ID.
func (s *AgentInteractionService) GetInteractionByID(
	ctx context.Context,
	interactionID uuid.UUID,
) (*model.AgentInteraction, error) {
	return s.repo.GetByID(ctx, interactionID)
}

func jsonEqual(a, b datatypes.JSON) bool {
	a = bytes.TrimSpace(a)
	b = bytes.TrimSpace(b)
	if len(a) == 0 {
		a = []byte("null")
	}
	if len(b) == 0 {
		b = []byte("null")
	}
	return bytes.Equal(a, b)
}
