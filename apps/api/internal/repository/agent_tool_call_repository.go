package repository

import (
	"context"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AgentToolCallRepository handles database operations for agent tool calls.
type AgentToolCallRepository struct {
	db *gorm.DB
}

// NewAgentToolCallRepository creates a new AgentToolCallRepository.
func NewAgentToolCallRepository(db *gorm.DB) *AgentToolCallRepository {
	return &AgentToolCallRepository{db: db}
}

// UpsertStarted creates or updates a tool call as running.
// Uses (run_id, tool_call_id) as the unique key.
func (r *AgentToolCallRepository) UpsertStarted(ctx context.Context, tc *model.AgentToolCall) error {
	return r.db.WithContext(ctx).
		Where("run_id = ? AND tool_call_id = ?", tc.RunID, tc.ToolCallID).
		Assign(map[string]any{
			"tool_name":  tc.ToolName,
			"arguments":  tc.Arguments,
			"status":     "running",
			"started_at": time.Now(),
		}).
		FirstOrCreate(tc).Error
}

// MarkSucceeded marks a tool call as succeeded with result data.
func (r *AgentToolCallRepository) MarkSucceeded(ctx context.Context, runID uuid.UUID, toolCallID string, result any) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&model.AgentToolCall{}).
		Where("run_id = ? AND tool_call_id = ?", runID, toolCallID).
		Updates(map[string]any{
			"status":      "succeeded",
			"result":      result,
			"finished_at": now,
		}).Error
}

// MarkFailed marks a tool call as failed with error data.
func (r *AgentToolCallRepository) MarkFailed(ctx context.Context, runID uuid.UUID, toolCallID string, errData any) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&model.AgentToolCall{}).
		Where("run_id = ? AND tool_call_id = ?", runID, toolCallID).
		Updates(map[string]any{
			"status":      "failed",
			"error":       errData,
			"finished_at": now,
		}).Error
}

// ListByConversationID retrieves all tool calls for a conversation in timeline order.
func (r *AgentToolCallRepository) ListByConversationID(ctx context.Context, conversationID uuid.UUID) ([]model.AgentToolCall, error) {
	var calls []model.AgentToolCall
	err := r.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Order("created_at ASC").
		Find(&calls).Error
	return calls, err
}

// ListByRunID retrieves all tool calls for a run.
func (r *AgentToolCallRepository) ListByRunID(ctx context.Context, runID uuid.UUID) ([]model.AgentToolCall, error) {
	var calls []model.AgentToolCall
	err := r.db.WithContext(ctx).
		Where("run_id = ?", runID).
		Order("created_at ASC").
		Find(&calls).Error
	return calls, err
}
