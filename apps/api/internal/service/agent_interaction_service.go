package service

import (
	"context"
	"fmt"
	"log"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// AgentInteractionService handles user interaction persistence and resume.
type AgentInteractionService struct {
	repo     *repository.AgentInteractionRepository
	runRepo  *repository.RunRepository
}

// NewAgentInteractionService creates a new AgentInteractionService.
func NewAgentInteractionService(
	repo *repository.AgentInteractionRepository,
	runRepo *repository.RunRepository,
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
) error {
	interaction := &model.AgentInteraction{
		RunID:          runID,
		ConversationID: conversationID,
		ToolCallID:     toolCallID,
		ToolName:       "ask_user",
		Question:       question,
		Status:         "pending",
	}
	if err := s.repo.CreatePending(ctx, interaction); err != nil {
		return fmt.Errorf("create pending interaction: %w", err)
	}

	// Mark run as waiting_user
	if err := s.runRepo.UpdateStatus(ctx, runID, "waiting_user"); err != nil {
		log.Printf("failed to mark run %s as waiting_user: %v", runID, err)
	}

	return nil
}

// ResumeInteraction marks an interaction as answered.
// Returns error if interaction not found or already answered.
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
		return fmt.Errorf("interaction not found: %s", interactionID)
	}
	if interaction.Status != "pending" {
		return fmt.Errorf("interaction already %s: %s", interaction.Status, interactionID)
	}

	if err := s.repo.MarkAnswered(ctx, interactionID, answer); err != nil {
		return fmt.Errorf("mark answered: %w", err)
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
