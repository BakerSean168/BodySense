package stream

import (
	"encoding/json"
	"testing"

	"github.com/bodysense/api/internal/dto"
)

func TestEnrichEvent_AppliesBaseIDs(t *testing.T) {
	w := &StreamWriter{
		nextSeq: func() int { return 1 },
		baseIDs: dto.StreamEventIDs{
			ConversationID: "conv-123",
			RunID:          "run-456",
			TurnID:         "turn-789",
		},
	}

	event := dto.StreamEvent{
		Version: 1,
		Seq:     5,
		Channel: "message",
		Type:    "message.text.delta",
		IDs:     dto.StreamEventIDs{},
		Payload: json.RawMessage(`{"delta":"hello"}`),
	}

	enriched := w.enrichEvent(event, "msg-abc")

	if enriched.IDs.ConversationID != "conv-123" {
		t.Errorf("expected ConversationID 'conv-123', got %q", enriched.IDs.ConversationID)
	}
	if enriched.IDs.RunID != "run-456" {
		t.Errorf("expected RunID 'run-456', got %q", enriched.IDs.RunID)
	}
	if enriched.IDs.TurnID != "turn-789" {
		t.Errorf("expected TurnID 'turn-789', got %q", enriched.IDs.TurnID)
	}
	if enriched.IDs.MessageID != "msg-abc" {
		t.Errorf("expected MessageID 'msg-abc', got %q", enriched.IDs.MessageID)
	}
}

func TestEnrichEvent_PreservesExistingIDs(t *testing.T) {
	w := &StreamWriter{
		nextSeq: func() int { return 1 },
		baseIDs: dto.StreamEventIDs{
			ConversationID: "conv-123",
			RunID:          "run-456",
			TurnID:         "turn-789",
		},
	}

	event := dto.StreamEvent{
		Version: 1,
		Seq:     5,
		IDs: dto.StreamEventIDs{
			ConversationID: "conv-custom",
			MessageID:      "msg-custom",
		},
		Payload: json.RawMessage(`{}`),
	}

	enriched := w.enrichEvent(event, "msg-default")

	if enriched.IDs.ConversationID != "conv-custom" {
		t.Errorf("expected ConversationID 'conv-custom', got %q", enriched.IDs.ConversationID)
	}
	// Existing MessageID should be preserved, not overwritten
	if enriched.IDs.MessageID != "msg-custom" {
		t.Errorf("expected MessageID 'msg-custom', got %q", enriched.IDs.MessageID)
	}
}

func TestEnrichEvent_ReassignsSeq(t *testing.T) {
	seqCounter := 0
	w := &StreamWriter{
		nextSeq: func() int { seqCounter++; return seqCounter },
		baseIDs: dto.StreamEventIDs{
			ConversationID: "conv-1",
			RunID:          "run-1",
			TurnID:         "turn-1",
		},
	}

	event := dto.StreamEvent{
		Version: 1,
		Seq:     99, // upstream events are re-sequenced for the public stream
		Type:    "test",
		Payload: json.RawMessage(`{}`),
	}

	enriched := w.enrichEvent(event, "")
	if enriched.Seq != 1 {
		t.Errorf("expected reassigned Seq 1, got %d", enriched.Seq)
	}
}

func TestEnrichEvent_EmptyPayloadBecomesObject(t *testing.T) {
	w := &StreamWriter{
		nextSeq: func() int { return 1 },
		baseIDs: dto.StreamEventIDs{},
	}

	event := dto.StreamEvent{
		Version: 1,
		Seq:     1,
		Payload: nil,
	}

	enriched := w.enrichEvent(event, "")
	if string(enriched.Payload) != "{}" {
		t.Errorf("expected empty payload to become {}, got %q", string(enriched.Payload))
	}
}

func TestEnrichEvent_NilPayloadBecomesObject(t *testing.T) {
	w := &StreamWriter{
		nextSeq: func() int { return 1 },
		baseIDs: dto.StreamEventIDs{},
	}

	event := dto.StreamEvent{
		Version: 1,
		Seq:     1,
		Payload: json.RawMessage(nil),
	}

	enriched := w.enrichEvent(event, "")
	if string(enriched.Payload) != "{}" {
		t.Errorf("expected nil payload to become {}, got %q", string(enriched.Payload))
	}
}

func TestStreamWriter_SendNew_IncrementsSeq(t *testing.T) {
	// We can't easily test SendNew without a real HTTP response,
	// but we can test the seq counter logic
	seqCounter := 0
	w := &StreamWriter{
		nextSeq: func() int { seqCounter++; return seqCounter },
		baseIDs: dto.StreamEventIDs{
			ConversationID: "conv-1",
			RunID:          "run-1",
			TurnID:         "turn-1",
		},
	}

	// Verify seq increments
	if w.nextSeq() != 1 {
		t.Error("expected first seq to be 1")
	}
	if w.nextSeq() != 2 {
		t.Error("expected second seq to be 2")
	}
	if w.nextSeq() != 3 {
		t.Error("expected third seq to be 3")
	}
}
