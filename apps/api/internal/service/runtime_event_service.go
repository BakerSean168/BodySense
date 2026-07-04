package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type runtimeEventRepo interface {
	Create(ctx context.Context, event *model.RuntimeEvent) error
	ListByRunID(ctx context.Context, conversationID, runID uuid.UUID, afterSeq, limit int) ([]model.RuntimeEvent, bool, error)
	ListByConversationID(ctx context.Context, conversationID uuid.UUID) ([]model.RuntimeEvent, error)
}

// RuntimeEventService persists and queries replayable public runtime events.
type RuntimeEventService struct {
	repo runtimeEventRepo
}

// NewRuntimeEventService creates a new RuntimeEventService.
func NewRuntimeEventService(repo runtimeEventRepo) *RuntimeEventService {
	return &RuntimeEventService{repo: repo}
}

// ShouldPersistEvent reports whether the public event should be stored durably.
func ShouldPersistEvent(eventType string) bool {
	switch eventType {
	case "conversation.created",
		"run.started",
		"run.resumed",
		"run.interrupted",
		"run.completed",
		"run.failed",
		"message.persisted",
		"message.created",
		"message.text.delta",
		"tool.call",
		"tool.result",
		"state.extracted_info.upsert",
		"state.phase.changed",
		"source.citation.added",
		"source.knowledge_gap",
		"safety.red_flag.detected",
		"state.interaction.required",
		"state.interaction.answered",
		"message.completed",
		"message.failed",
		"title.generated",
		"usage.reported":
		return true
	default:
		return false
	}
}

// RecordPublicEvent maps a public StreamEvent into a durable runtime event.
func (s *RuntimeEventService) RecordPublicEvent(
	ctx context.Context,
	conversationID uuid.UUID,
	runID uuid.UUID,
	turnID *uuid.UUID,
	event dto.StreamEvent,
) error {
	if !ShouldPersistEvent(event.Type) {
		return nil
	}

	idsJSON, err := json.Marshal(event.IDs)
	if err != nil {
		return fmt.Errorf("marshal event ids: %w", err)
	}

	payload := event.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}

	record := &model.RuntimeEvent{
		ConversationID: conversationID,
		RunID:          runID,
		TurnID:         turnID,
		Seq:            event.Seq,
		Channel:        event.Channel,
		Type:           event.Type,
		IDs:            datatypes.JSON(idsJSON),
		Payload:        datatypes.JSON(payload),
		Source:         "go",
		Replayable:     true,
	}
	if err := s.repo.Create(ctx, record); err != nil {
		return fmt.Errorf("create runtime event: %w", err)
	}
	return nil
}

// ListRunEvents returns durable events for a run.
func (s *RuntimeEventService) ListRunEvents(
	ctx context.Context,
	conversationID, runID uuid.UUID,
	afterSeq, limit int,
) ([]model.RuntimeEvent, bool, error) {
	events, hasMore, err := s.repo.ListByRunID(ctx, conversationID, runID, afterSeq, limit)
	if err != nil {
		return nil, false, fmt.Errorf("list runtime events: %w", err)
	}
	return events, hasMore, nil
}

// ListConversationEvents returns all durable events for a conversation in timeline order.
func (s *RuntimeEventService) ListConversationEvents(
	ctx context.Context,
	conversationID uuid.UUID,
) ([]model.RuntimeEvent, error) {
	events, err := s.repo.ListByConversationID(ctx, conversationID)
	if err != nil {
		return nil, fmt.Errorf("list conversation runtime events: %w", err)
	}
	return events, nil
}

// ListAllRunEvents returns all durable events for a run in ascending seq order.
func (s *RuntimeEventService) ListAllRunEvents(
	ctx context.Context,
	conversationID, runID uuid.UUID,
) ([]model.RuntimeEvent, error) {
	const batchSize = 1000

	all := make([]model.RuntimeEvent, 0, batchSize)
	afterSeq := 0

	for {
		events, hasMore, err := s.ListRunEvents(ctx, conversationID, runID, afterSeq, batchSize)
		if err != nil {
			return nil, err
		}
		if len(events) == 0 {
			return all, nil
		}

		all = append(all, events...)
		afterSeq = events[len(events)-1].Seq
		if !hasMore {
			return all, nil
		}
	}
}
