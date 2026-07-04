package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type mockThreadProjectionRuntimeEventSource struct {
	conversationID     uuid.UUID
	runID              uuid.UUID
	conversationEvents []model.RuntimeEvent
	runEvents          []model.RuntimeEvent
}

func (m *mockThreadProjectionRuntimeEventSource) ListAllRunEvents(_ context.Context, conversationID, runID uuid.UUID) ([]model.RuntimeEvent, error) {
	m.conversationID = conversationID
	m.runID = runID
	return m.runEvents, nil
}

func (m *mockThreadProjectionRuntimeEventSource) ListConversationEvents(_ context.Context, conversationID uuid.UUID) ([]model.RuntimeEvent, error) {
	m.conversationID = conversationID
	return m.conversationEvents, nil
}

func TestSelectActiveTurnRunIDPrefersLatestPendingInteraction(t *testing.T) {
	activeRunID := uuid.New()
	oldPendingRunID := uuid.New()
	latestPendingRunID := uuid.New()

	runID := selectActiveTurnRunID(&activeRunID, []model.AgentInteraction{
		{
			RunID:     oldPendingRunID,
			Status:    "pending",
			CreatedAt: time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC),
		},
		{
			RunID:     latestPendingRunID,
			Status:    "pending",
			CreatedAt: time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC),
		},
	})

	if runID == nil {
		t.Fatal("expected selected run id")
	}
	if *runID != latestPendingRunID {
		t.Fatalf("expected latest pending run %s, got %s", latestPendingRunID, *runID)
	}
}

func TestSelectActiveTurnRunIDFallsBackToActiveRun(t *testing.T) {
	activeRunID := uuid.New()

	runID := selectActiveTurnRunID(&activeRunID, nil)
	if runID == nil {
		t.Fatal("expected selected run id")
	}
	if *runID != activeRunID {
		t.Fatalf("expected active run %s, got %s", activeRunID, *runID)
	}
}

func TestLoadActiveTurnEventsReturnsSelectedRunEvents(t *testing.T) {
	conversationID := uuid.New()
	runID := uuid.New()
	events := []model.RuntimeEvent{{Seq: 1, Type: "message.created"}, {Seq: 2, Type: "message.text.delta"}}
	source := &mockThreadProjectionRuntimeEventSource{runEvents: events}
	svc := &ThreadProjectionService{runtimeEventRepo: source}

	loaded, err := svc.loadActiveTurnEvents(context.Background(), conversationID, &runID)
	if err != nil {
		t.Fatalf("loadActiveTurnEvents returned error: %v", err)
	}
	if source.conversationID != conversationID || source.runID != runID {
		t.Fatalf("unexpected lookup ids: conversation=%s run=%s", source.conversationID, source.runID)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 events, got %d", len(loaded))
	}
	if loaded[1].Type != "message.text.delta" {
		t.Fatalf("unexpected event payload: %+v", loaded[1])
	}
}

func TestBuildThreadProjectionToolCallsFromEvents(t *testing.T) {
	runID := uuid.New()
	conversationID := uuid.New()
	messageID := uuid.New()
	start := time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC)
	finish := start.Add(2 * time.Second)

	callIDs, _ := json.Marshal(map[string]string{
		"conversation_id": conversationID.String(),
		"run_id":          runID.String(),
		"message_id":      messageID.String(),
		"tool_call_id":    "tc-1",
	})
	callPayload, _ := json.Marshal(map[string]any{
		"tool": "search_knowledge",
		"args": map[string]any{"query": "neck pain"},
	})
	resultIDs := callIDs
	resultPayload, _ := json.Marshal(map[string]any{
		"tool":   "search_knowledge",
		"result": map[string]any{"has_results": true},
	})

	projected := buildThreadProjectionToolCallsFromEvents([]model.RuntimeEvent{
		{
			ConversationID: conversationID,
			RunID:          runID,
			Type:           "tool.call",
			IDs:            datatypes.JSON(callIDs),
			Payload:        datatypes.JSON(callPayload),
			CreatedAt:      start,
		},
		{
			ConversationID: conversationID,
			RunID:          runID,
			Type:           "tool.result",
			IDs:            datatypes.JSON(resultIDs),
			Payload:        datatypes.JSON(resultPayload),
			CreatedAt:      finish,
		},
	})

	if len(projected) != 1 {
		t.Fatalf("expected 1 projected tool call, got %d", len(projected))
	}
	if projected[0].ToolCallID != "tc-1" || projected[0].ToolName != "search_knowledge" {
		t.Fatalf("unexpected tool call projection: %+v", projected[0])
	}
	if projected[0].Status != "succeeded" {
		t.Fatalf("expected succeeded status, got %s", projected[0].Status)
	}
	if projected[0].MessageID == nil || *projected[0].MessageID != messageID {
		t.Fatalf("expected message id %s, got %+v", messageID, projected[0].MessageID)
	}
	if projected[0].FinishedAt == nil || !projected[0].FinishedAt.Equal(finish) {
		t.Fatalf("expected finished_at %s, got %+v", finish, projected[0].FinishedAt)
	}
}
