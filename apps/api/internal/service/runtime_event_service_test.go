package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type mockRuntimeEventRepo struct {
	created            []*model.RuntimeEvent
	conversationEvents []model.RuntimeEvent
}

func (m *mockRuntimeEventRepo) Create(_ context.Context, event *model.RuntimeEvent) error {
	clone := *event
	m.created = append(m.created, &clone)
	return nil
}

func (m *mockRuntimeEventRepo) CreateBatch(_ context.Context, events []*model.RuntimeEvent) error {
	for _, event := range events {
		clone := *event
		m.created = append(m.created, &clone)
	}
	return nil
}

func (m *mockRuntimeEventRepo) ListByRunID(_ context.Context, conversationID, runID uuid.UUID, afterSeq, limit int) ([]model.RuntimeEvent, bool, error) {
	return nil, false, nil
}

func (m *mockRuntimeEventRepo) ListByConversationID(_ context.Context, conversationID uuid.UUID) ([]model.RuntimeEvent, error) {
	return m.conversationEvents, nil
}

func TestShouldPersistEvent(t *testing.T) {
	if !ShouldPersistEvent("run.started") {
		t.Fatal("expected run.started to be persisted")
	}
	if !ShouldPersistEvent("tool.call") {
		t.Fatal("expected tool.call to be persisted")
	}
	if !ShouldPersistEvent("message.text.delta") {
		t.Fatal("expected message.text.delta to be persisted")
	}
	if !ShouldPersistEvent("source.citation.added") {
		t.Fatal("expected source.citation.added to be persisted")
	}
	if !ShouldPersistEvent("source.knowledge_gap") {
		t.Fatal("expected source.knowledge_gap to be persisted")
	}
	if !ShouldPersistEvent("safety.red_flag.detected") {
		t.Fatal("expected safety.red_flag.detected to be persisted")
	}
}

func TestRecordPublicEventMapsStreamEvent(t *testing.T) {
	repo := &mockRuntimeEventRepo{}
	svc := NewRuntimeEventService(repo)

	conversationID := uuid.New()
	runID := uuid.New()
	turnID := uuid.New()

	payload := json.RawMessage(`{"status":"completed"}`)
	event := dto.StreamEvent{
		Version: 1,
		Seq:     7,
		Channel: "message",
		Type:    "message.completed",
		IDs: dto.StreamEventIDs{
			ConversationID: conversationID.String(),
			RunID:          runID.String(),
			TurnID:         turnID.String(),
			MessageID:      uuid.New().String(),
		},
		Payload: payload,
	}

	if err := svc.RecordPublicEvent(context.Background(), conversationID, runID, &turnID, event); err != nil {
		t.Fatalf("RecordPublicEvent failed: %v", err)
	}

	if len(repo.created) != 1 {
		t.Fatalf("expected 1 event, got %d", len(repo.created))
	}

	created := repo.created[0]
	if created.Seq != 7 || created.Type != "message.completed" || created.Channel != "message" {
		t.Fatalf("unexpected stored event: %+v", created)
	}
	if created.ConversationID != conversationID || created.RunID != runID {
		t.Fatalf("unexpected ids: %+v", created)
	}
	if created.TurnID == nil || *created.TurnID != turnID {
		t.Fatalf("unexpected turn id: %+v", created.TurnID)
	}
	if string(created.Payload) != string(payload) {
		t.Fatalf("payload mismatch: got %s want %s", string(created.Payload), string(payload))
	}
}

func TestRecordPublicEventSkipsNonReplayableEvent(t *testing.T) {
	repo := &mockRuntimeEventRepo{}
	svc := NewRuntimeEventService(repo)

	err := svc.RecordPublicEvent(
		context.Background(),
		uuid.New(),
		uuid.New(),
		nil,
		dto.StreamEvent{Type: "stream.done", Payload: json.RawMessage(`{}`)},
	)
	if err != nil {
		t.Fatalf("RecordPublicEvent returned error: %v", err)
	}
	if len(repo.created) != 0 {
		t.Fatalf("expected no stored event, got %d", len(repo.created))
	}
}

func TestShouldPersistInteractionExpired(t *testing.T) {
	if !ShouldPersistEvent("state.interaction.expired") {
		t.Fatal("expected state.interaction.expired to be persisted")
	}
}

func TestDeltaBufferFlushesOnMilestone(t *testing.T) {
	repo := &mockRuntimeEventRepo{}
	svc := NewRuntimeEventService(repo)
	svc.deltaFlushSize = 10 // large so auto-flush does not trigger mid-test

	conversationID := uuid.New()
	runID := uuid.New()
	turnID := uuid.New()

	for i := 1; i <= 3; i++ {
		event := dto.StreamEvent{
			Version: 1,
			Seq:     i,
			Channel: "message",
			Type:    "message.text.delta",
			IDs: dto.StreamEventIDs{
				ConversationID: conversationID.String(),
				RunID:          runID.String(),
				TurnID:         turnID.String(),
				MessageID:      "msg-1",
			},
			Payload: json.RawMessage(`{"delta":"x"}`),
		}
		if err := svc.RecordPublicEvent(context.Background(), conversationID, runID, &turnID, event); err != nil {
			t.Fatalf("record delta: %v", err)
		}
	}
	if len(repo.created) != 0 {
		t.Fatalf("deltas should be buffered, got %d writes", len(repo.created))
	}

	milestone := dto.StreamEvent{
		Version: 1,
		Seq:     4,
		Channel: "message",
		Type:    "message.completed",
		IDs: dto.StreamEventIDs{
			ConversationID: conversationID.String(),
			RunID:          runID.String(),
			TurnID:         turnID.String(),
			MessageID:      "msg-1",
		},
		Payload: json.RawMessage(`{"status":"completed"}`),
	}
	if err := svc.RecordPublicEvent(context.Background(), conversationID, runID, &turnID, milestone); err != nil {
		t.Fatalf("record milestone: %v", err)
	}
	if len(repo.created) != 4 {
		t.Fatalf("expected 3 deltas + 1 milestone, got %d", len(repo.created))
	}
	for i := 0; i < 3; i++ {
		if repo.created[i].Type != "message.text.delta" || repo.created[i].Seq != i+1 {
			t.Fatalf("buffered delta order broken at %d: %#v", i, repo.created[i])
		}
	}
	if repo.created[3].Type != "message.completed" {
		t.Fatalf("milestone should be last, got %s", repo.created[3].Type)
	}
}

func TestRecordInteractionExpired(t *testing.T) {
	repo := &mockRuntimeEventRepo{}
	svc := NewRuntimeEventService(repo)
	interaction := &model.AgentInteraction{
		ID:             uuid.New(),
		RunID:          uuid.New(),
		ConversationID: uuid.New(),
		ToolCallID:     "tool-1",
		Status:         "expired",
	}
	if err := svc.RecordInteractionExpired(context.Background(), interaction); err != nil {
		t.Fatalf("RecordInteractionExpired: %v", err)
	}
	if len(repo.created) != 1 {
		t.Fatalf("expected 1 event, got %d", len(repo.created))
	}
	if repo.created[0].Type != "state.interaction.expired" {
		t.Fatalf("unexpected type %s", repo.created[0].Type)
	}
}
