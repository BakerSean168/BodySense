package service

import (
	"context"
	"log/slog"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// AgentToolCallRepo defines the repository interface for tool call persistence.
type AgentToolCallRepo interface {
	UpsertStarted(ctx context.Context, tc *model.AgentToolCall) error
	MarkSucceeded(ctx context.Context, runID uuid.UUID, toolCallID string, result any) (bool, error)
	MarkFailed(ctx context.Context, runID uuid.UUID, toolCallID string, errData any) (bool, error)
}

// AgentToolService handles tool call audit persistence.
type AgentToolService struct {
	repo AgentToolCallRepo
}

// NewAgentToolService creates a new AgentToolService.
func NewAgentToolService(repo AgentToolCallRepo) *AgentToolService {
	return &AgentToolService{repo: repo}
}

// RecordToolCall persists a tool.call event as running.
// Non-blocking: logs errors but does not fail the caller.
func (s *AgentToolService) RecordToolCall(
	ctx context.Context,
	runID, conversationID uuid.UUID,
	messageID *uuid.UUID,
	toolCallID, toolName string,
	arguments datatypes.JSON,
) {
	if toolCallID == "" {
		slog.Default().Warn("skipping_tool_call_audit_empty_id", "tool_name", toolName)
		return
	}

	tc := &model.AgentToolCall{
		RunID:          runID,
		ConversationID: conversationID,
		MessageID:      messageID,
		ToolCallID:     toolCallID,
		ToolName:       toolName,
		Arguments:      arguments,
		Status:         model.AgentToolCallRunning,
	}

	if err := s.repo.UpsertStarted(ctx, tc); err != nil {
		slog.Default().Error("record_tool_call_failed", "tool_call_id", toolCallID, "run_id", runID, "error", err)
	}
}

// RecordToolResult persists a tool.result event as succeeded or failed.
// Non-blocking: logs errors but does not fail the caller.
func (s *AgentToolService) RecordToolResult(
	ctx context.Context,
	runID uuid.UUID,
	toolCallID string,
	result datatypes.JSON,
	isError bool,
) {
	if toolCallID == "" {
		return
	}

	if isError {
		updated, err := s.repo.MarkFailed(ctx, runID, toolCallID, result)
		if err != nil {
			slog.Default().Error("mark_tool_call_failed_failed", "tool_call_id", toolCallID, "run_id", runID, "error", err)
		} else if !updated {
			slog.Default().Warn("ignored_stale_failed_result_for_terminal_tool_call", "tool_call_id", toolCallID, "run_id", runID)
		}
	} else {
		updated, err := s.repo.MarkSucceeded(ctx, runID, toolCallID, result)
		if err != nil {
			slog.Default().Error("mark_tool_call_succeeded_failed", "tool_call_id", toolCallID, "run_id", runID, "error", err)
		} else if !updated {
			slog.Default().Warn("ignored_stale_succeeded_result_for_terminal_tool_call", "tool_call_id", toolCallID, "run_id", runID)
		}
	}
}
