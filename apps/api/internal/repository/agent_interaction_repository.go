package repository

import (
	"context"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AgentInteractionRepository handles database operations for agent interactions.
type AgentInteractionRepository struct {
	db *gorm.DB
}

// NewAgentInteractionRepository creates a new AgentInteractionRepository.
func NewAgentInteractionRepository(db *gorm.DB) *AgentInteractionRepository {
	return &AgentInteractionRepository{db: db}
}

// CreatePending creates a new pending interaction.
func (r *AgentInteractionRepository) CreatePending(ctx context.Context, interaction *model.AgentInteraction) error {
	interaction.Status = "pending"
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(interaction).Error
}

// GetByID retrieves an interaction by ID.
func (r *AgentInteractionRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.AgentInteraction, error) {
	var interaction model.AgentInteraction
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&interaction).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &interaction, nil
}

// GetByRunAndToolCall retrieves an interaction by the durable run/tool-call pair.
func (r *AgentInteractionRepository) GetByRunAndToolCall(ctx context.Context, runID uuid.UUID, toolCallID string) (*model.AgentInteraction, error) {
	var interaction model.AgentInteraction
	err := r.db.WithContext(ctx).
		Where("run_id = ? AND tool_call_id = ?", runID, toolCallID).
		First(&interaction).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &interaction, nil
}

// MarkAnswered marks an interaction as answered with the user's response.
func (r *AgentInteractionRepository) MarkAnswered(ctx context.Context, id uuid.UUID, answer any) (bool, error) {
	now := time.Now()
	result := r.db.WithContext(ctx).
		Model(&model.AgentInteraction{}).
		Where("id = ? AND status = 'pending'", id).
		Updates(map[string]any{
			"status":      "answered",
			"answer":      answer,
			"answered_at": now,
		})
	return result.RowsAffected > 0, result.Error
}

// CancelPending marks a pending interaction as cancelled.
func (r *AgentInteractionRepository) CancelPending(ctx context.Context, id uuid.UUID) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&model.AgentInteraction{}).
		Where("id = ? AND status = 'pending'", id).
		Update("status", "cancelled")
	return result.RowsAffected > 0, result.Error
}

// ListPendingByConversation retrieves pending interactions for a conversation.
func (r *AgentInteractionRepository) ListPendingByConversation(ctx context.Context, conversationID uuid.UUID) ([]model.AgentInteraction, error) {
	var interactions []model.AgentInteraction
	err := r.db.WithContext(ctx).
		Where("conversation_id = ? AND status = 'pending'", conversationID).
		Order("created_at ASC").
		Find(&interactions).Error
	return interactions, err
}

// ListByConversation retrieves all interactions for a conversation.
func (r *AgentInteractionRepository) ListByConversation(ctx context.Context, conversationID uuid.UUID) ([]model.AgentInteraction, error) {
	var interactions []model.AgentInteraction
	err := r.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Order("created_at ASC").
		Find(&interactions).Error
	return interactions, err
}

// ListByRunID retrieves all interactions for a run.
func (r *AgentInteractionRepository) ListByRunID(ctx context.Context, runID uuid.UUID) ([]model.AgentInteraction, error) {
	var interactions []model.AgentInteraction
	err := r.db.WithContext(ctx).
		Where("run_id = ?", runID).
		Order("created_at ASC").
		Find(&interactions).Error
	return interactions, err
}
