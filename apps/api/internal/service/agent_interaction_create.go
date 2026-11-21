package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// CreatePendingInteraction creates a pending interaction from an ask_user event.
// Tests and non-stream callers may use this state-only form; the production
// consultation stream uses CreatePendingInteractionWithEvents so the public
// lifecycle milestones share the same transaction.
func (s *AgentInteractionService) CreatePendingInteraction(
	ctx context.Context,
	runID, conversationID uuid.UUID,
	toolCallID string,
	question datatypes.JSON,
) (*model.AgentInteraction, error) {
	interaction, _, err := s.createPendingInteraction(ctx, runID, conversationID, toolCallID, question, nil)
	return interaction, err
}

// CreatePendingInteractionWithEvents commits pending interaction state,
// running->waiting_user and the prepared interaction.required/run.interrupted
// milestones as one unit. Returned events are already durable and must only be
// written to SSE after commit.
func (s *AgentInteractionService) CreatePendingInteractionWithEvents(
	ctx context.Context,
	runID, conversationID uuid.UUID,
	toolCallID string,
	question datatypes.JSON,
	prepare interactionPreparedEventsBuilder,
	nativeInterruptID ...string,
) (*model.AgentInteraction, []dto.StreamEvent, error) {
	if prepare == nil {
		return nil, nil, errors.New("create pending interaction: prepared event builder is required")
	}
	return s.createPendingInteraction(ctx, runID, conversationID, toolCallID, question, prepare, nativeInterruptID...)
}

func (s *AgentInteractionService) createPendingInteraction(
	ctx context.Context,
	runID, conversationID uuid.UUID,
	toolCallID string,
	question datatypes.JSON,
	prepare interactionPreparedEventsBuilder,
	nativeInterruptID ...string,
) (*model.AgentInteraction, []dto.StreamEvent, error) {
	if s.transactions == nil {
		return nil, nil, errors.New("create pending interaction: transaction manager is not configured")
	}
	if prepare != nil {
		if s.lifecycleEvents == nil {
			return nil, nil, errors.New("create pending interaction: lifecycle event store is not configured")
		}
		if err := s.lifecycleEvents.Flush(ctx); err != nil {
			return nil, nil, fmt.Errorf("flush runtime events before interaction required: %w", err)
		}
	}
	now := time.Now().UTC()
	expires := now.Add(DefaultInteractionTTL)
	interaction := &model.AgentInteraction{
		ID:             uuid.New(),
		RunID:          runID,
		ConversationID: conversationID,
		ToolCallID:     toolCallID,
		ToolName:       "ask_user",
		Question:       question,
		Status:         model.AgentInteractionPending,
		CreatedAt:      now,
		ExpiresAt:      &expires,
	}

	if len(nativeInterruptID) > 0 {
		raw := nativeInterruptID[0]
		if len(raw) != 32 {
			return nil, nil, errors.New("invalid native interrupt identity")
		}
		parsed, err := uuid.Parse(raw)
		if err != nil || parsed == uuid.Nil {
			return nil, nil, errors.New("invalid native interrupt identity")
		}
		metadata, err := json.Marshal(map[string]string{"runtime_interrupt_id": raw})
		if err != nil {
			return nil, nil, err
		}
		interaction.Metadata = datatypes.JSON(metadata)
	}

	var created *model.AgentInteraction
	var prepared []dto.StreamEvent
	err := s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.repo.CreatePending(txCtx, interaction); err != nil {
			return fmt.Errorf("create pending interaction: %w", err)
		}
		loaded, err := s.repo.GetByRunAndToolCall(txCtx, runID, toolCallID)
		if err != nil {
			return fmt.Errorf("load pending interaction: %w", err)
		}
		if loaded == nil {
			return fmt.Errorf("load pending interaction: %w", ErrInteractionNotFound)
		}
		created = loaded
		if err := s.runLifecycle.MarkWaitingUser(txCtx, runID); err != nil {
			return fmt.Errorf("mark run waiting_user: %w", err)
		}
		if prepare == nil {
			return nil
		}
		prepared, err = prepare(created)
		if err != nil {
			return fmt.Errorf("prepare interaction lifecycle events: %w", err)
		}
		for _, event := range prepared {
			var turnID *uuid.UUID
			if event.IDs.TurnID != "" {
				parsed, parseErr := uuid.Parse(event.IDs.TurnID)
				if parseErr != nil {
					return fmt.Errorf("parse interaction lifecycle turn id: %w", parseErr)
				}
				turnID = &parsed
			}
			if err := s.lifecycleEvents.PersistPreparedMilestone(txCtx, conversationID, runID, turnID, event); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return created, prepared, nil
}
