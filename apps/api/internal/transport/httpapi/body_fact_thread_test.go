package httpapi

import (
	"encoding/json"
	"testing"
	"time"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

func TestBodyFactThreadWirePreservesCommittedToolAndUserMessages(t *testing.T) {
	owner, conversation, run, turn, messageID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	now := time.Now().UTC()
	toolID := "bodyfact-intake-v1:" + run.String()
	receipt := bodyFactHTTPReceipt(owner)
	result, err := json.Marshal(map[string]any{"status": "persisted", "receipt": receipt})
	if err != nil {
		t.Fatal(err)
	}
	parts, err := json.Marshal([]map[string]any{
		{"type": "tool-call", "toolName": "record_body_state", "toolCallId": toolID, "args": map[string]any{"fact_count": 1}},
		{"type": "tool-result", "toolName": "record_body_state", "toolCallId": toolID, "result": json.RawMessage(result)},
		{"type": "text", "text": "synthetic response"},
	})
	if err != nil {
		t.Fatal(err)
	}
	wire := consultationThreadWire{
		ConversationID: conversation,
		Conversation:   consultationThreadConversationWire{ID: conversation, Title: "synthetic", TitleStatus: "generated", Status: "active", Metadata: json.RawMessage(`{}`), MessageCount: 2, CreatedAt: now, UpdatedAt: now},
		Phase:          "collecting", ExtractedInfo: json.RawMessage(`[]`), PendingInteractions: json.RawMessage(`[]`), InteractionHistory: json.RawMessage(`[]`),
		BodyState:        &service.BodyStateSnapshot{UserID: owner, CurrentRevision: 1, SafetyState: json.RawMessage(`{}`), Facts: []model.BodyStateFact{}, Observations: []model.BodyStateObservation{}, RecentRevisions: []model.BodyStateRevision{}},
		ActiveTurnEvents: []threadRuntimeEventWire{},
		Messages: []model.ThreadProjectionMessage{
			{MessageID: uuid.New(), ConversationID: conversation, TurnID: turn, Seq: 1, Role: "user", Status: "completed", Parts: datatypes.JSON(`[{"type":"text","text":"I have no shoulder pain"}]`), Metadata: datatypes.JSON(`{}`), CreatedAt: now, UpdatedAt: now},
			{MessageID: messageID, ConversationID: conversation, TurnID: turn, RunID: &run, Seq: 2, Role: "assistant", Status: "completed", Parts: datatypes.JSON(parts), Metadata: datatypes.JSON(`{}`), CreatedAt: now, UpdatedAt: now},
		},
		ToolCalls: newProjectedToolCallWires([]model.ThreadProjectionToolCall{{ToolCallID: toolID, ConversationID: conversation, RunID: run, MessageID: &messageID, ToolName: "record_body_state", Arguments: datatypes.JSON(`{"fact_count":1}`), Status: "succeeded", Result: datatypes.JSON(result), CreatedAt: now, StartedAt: now, FinishedAt: &now, Metadata: datatypes.JSON(`{}`)}}),
		CreatedAt: now, UpdatedAt: now,
	}
	if _, err := strictOpenAPIConvert[openapiv1.ConsultationThreadResponse]("ConsultationThreadResponse", wire); err != nil {
		t.Fatal(err)
	}
}
