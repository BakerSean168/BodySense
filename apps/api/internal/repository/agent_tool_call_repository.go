package repository

import (
	"context"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AgentToolCallRepository handles database operations for agent tool calls.
type AgentToolCallRepository struct {
	db *gorm.DB
}

// NewAgentToolCallRepository creates a new AgentToolCallRepository.
func NewAgentToolCallRepository(db *gorm.DB) *AgentToolCallRepository {
	return &AgentToolCallRepository{db: db}
}

// UpsertStarted creates a running tool call if missing. Duplicate tool.call
// delivery may refresh descriptive fields only while the call is still running;
// it can never reopen a succeeded/failed terminal row.
func (r *AgentToolCallRepository) UpsertStarted(ctx context.Context, tc *model.AgentToolCall) error {
	db := database.FromContext(ctx, r.db)
	tc.Status = model.AgentToolCallRunning
	if tc.StartedAt.IsZero() {
		tc.StartedAt = time.Now()
	}
	created := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "run_id"}, {Name: "tool_call_id"}},
		DoNothing: true,
	}).Create(tc)
	if created.Error != nil || created.RowsAffected == 1 {
		return created.Error
	}
	return db.Model(&model.AgentToolCall{}).
		Where("run_id = ? AND tool_call_id = ? AND status = ?", tc.RunID, tc.ToolCallID, model.AgentToolCallRunning).
		Updates(map[string]any{
			"tool_name": tc.ToolName,
			"arguments": tc.Arguments,
		}).Error
}

// MarkSucceeded performs exactly running -> succeeded. A false result means a
// competing terminal result already won and must not be overwritten.
func (r *AgentToolCallRepository) MarkSucceeded(ctx context.Context, runID uuid.UUID, toolCallID string, result any) (bool, error) {
	now := time.Now()
	updated := database.FromContext(ctx, r.db).
		Model(&model.AgentToolCall{}).
		Where("run_id = ? AND tool_call_id = ? AND status = ?", runID, toolCallID, model.AgentToolCallRunning).
		Updates(map[string]any{
			"status":      model.AgentToolCallSucceeded,
			"result":      result,
			"finished_at": now,
		})
	return updated.RowsAffected == 1, updated.Error
}

// MarkFailed performs exactly running -> failed. A false result means a
// competing terminal result already won and must not be overwritten.
func (r *AgentToolCallRepository) MarkFailed(ctx context.Context, runID uuid.UUID, toolCallID string, errData any) (bool, error) {
	now := time.Now()
	updated := database.FromContext(ctx, r.db).
		Model(&model.AgentToolCall{}).
		Where("run_id = ? AND tool_call_id = ? AND status = ?", runID, toolCallID, model.AgentToolCallRunning).
		Updates(map[string]any{
			"status":      model.AgentToolCallFailed,
			"error":       errData,
			"finished_at": now,
		})
	return updated.RowsAffected == 1, updated.Error
}

// ListByConversationID retrieves all tool calls for a conversation in timeline order.
func (r *AgentToolCallRepository) ListByConversationID(ctx context.Context, conversationID uuid.UUID) ([]model.AgentToolCall, error) {
	var calls []model.AgentToolCall
	err := database.FromContext(ctx, r.db).
		Where("conversation_id = ?", conversationID).
		Order("created_at ASC").
		Find(&calls).Error
	return calls, err
}

// ListByRunID retrieves all tool calls for a run.
func (r *AgentToolCallRepository) ListByRunID(ctx context.Context, runID uuid.UUID) ([]model.AgentToolCall, error) {
	var calls []model.AgentToolCall
	err := database.FromContext(ctx, r.db).
		Where("run_id = ?", runID).
		Order("created_at ASC").
		Find(&calls).Error
	return calls, err
}
