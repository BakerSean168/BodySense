package service

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type mockRuntimeEventRepo struct {
	mu                 sync.Mutex
	created            []*model.RuntimeEvent
	conversationEvents []model.RuntimeEvent
}

func (m *mockRuntimeEventRepo) Create(_ context.Context, event *model.RuntimeEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	clone := *event
	m.created = append(m.created, &clone)
	return nil
}

func (m *mockRuntimeEventRepo) CreateBatch(_ context.Context, events []*model.RuntimeEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, event := range events {
		clone := *event
		m.created = append(m.created, &clone)
	}
	return nil
}

func (m *mockRuntimeEventRepo) CreateWithNextSequence(_ context.Context, event *model.RuntimeEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	maxSeq := 0
	for _, existing := range m.created {
		if existing.RunID == event.RunID && existing.Seq > maxSeq {
			maxSeq = existing.Seq
		}
	}
	clone := *event
	clone.Seq = maxSeq + 1
	event.Seq = clone.Seq
	m.created = append(m.created, &clone)
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
	if !ShouldPersistEvent("source.answer_attribution.added") {
		t.Fatal("expected source.answer_attribution.added to be persisted")
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

func TestRecordPublicEventPersistsTerminalStreamEvents(t *testing.T) {
	for _, eventType := range []string{"stream.done", "stream.error"} {
		t.Run(eventType, func(t *testing.T) {
			repo := &mockRuntimeEventRepo{}
			svc := NewRuntimeEventService(repo)
			conversationID := uuid.New()
			runID := uuid.New()

			err := svc.RecordPublicEvent(
				context.Background(),
				conversationID,
				runID,
				nil,
				dto.StreamEvent{
					Version: 1,
					Seq:     9,
					Channel: "stream",
					Type:    eventType,
					IDs:     dto.StreamEventIDs{ConversationID: conversationID.String(), RunID: runID.String()},
					Payload: json.RawMessage(`{}`),
				},
			)
			if err != nil {
				t.Fatalf("RecordPublicEvent returned error: %v", err)
			}
			if len(repo.created) != 1 || repo.created[0].Type != eventType {
				t.Fatalf("expected persisted %s, got %#v", eventType, repo.created)
			}
		})
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

func TestRecordOutOfBandEventAllocatesMonotonicPerRunSequenceConcurrently(t *testing.T) {
	repo := &mockRuntimeEventRepo{}
	svc := NewRuntimeEventService(repo)
	conversationID := uuid.New()
	runID := uuid.New()
	const workers = 32

	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- svc.RecordOutOfBandPublicEvent(
				context.Background(), conversationID, runID, nil,
				"state", "state.interaction.expired",
				dto.StreamEventIDs{ConversationID: conversationID.String(), RunID: runID.String(), InteractionID: uuid.New().String()},
				map[string]any{"interaction_id": uuid.New().String(), "expired_at": "2026-08-23T00:00:00Z"},
			)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()
	seen := make(map[int]bool, workers)
	for _, event := range repo.created {
		if event.RunID != runID {
			continue
		}
		if seen[event.Seq] {
			t.Fatalf("duplicate seq %d", event.Seq)
		}
		seen[event.Seq] = true
	}
	if len(seen) != workers {
		t.Fatalf("got %d sequences, want %d", len(seen), workers)
	}
	for seq := 1; seq <= workers; seq++ {
		if !seen[seq] {
			t.Fatalf("missing monotonic sequence %d", seq)
		}
	}
}
