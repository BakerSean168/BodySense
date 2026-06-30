package service

import (
	"context"
	"log"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// AgentToolCallRepo defines the repository interface for tool call persistence.
type AgentToolCallRepo interface {
	UpsertStarted(ctx context.Context, tc *model.AgentToolCall) error
	MarkSucceeded(ctx context.Context, runID uuid.UUID, toolCallID string, result any) error
	MarkFailed(ctx context.Context, runID uuid.UUID, toolCallID string, errData any) error
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
		log.Printf("skipping tool call audit: empty tool_call_id for tool %s", toolName)
		return
	}

	tc := &model.AgentToolCall{
		RunID:          runID,
		ConversationID: conversationID,
		MessageID:      messageID,
		ToolCallID:     toolCallID,
		ToolName:       toolName,
		Arguments:      arguments,
		Status:         "running",
	}

	if err := s.repo.UpsertStarted(ctx, tc); err != nil {
		log.Printf("failed to record tool call %s for run %s: %v", toolCallID, runID, err)
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
		if err := s.repo.MarkFailed(ctx, runID, toolCallID, result); err != nil {
			log.Printf("failed to mark tool call %s as failed for run %s: %v", toolCallID, runID, err)
		}
	} else {
		if err := s.repo.MarkSucceeded(ctx, runID, toolCallID, result); err != nil {
			log.Printf("failed to mark tool call %s as succeeded for run %s: %v", toolCallID, runID, err)
		}
	}
}
