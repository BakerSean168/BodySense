package stream

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/bodysense/api/internal/dto"
)

// Runtime creates StreamWriters that own sequence allocation and ID enrichment.
type Runtime struct{}

// NewRuntime creates a new stream Runtime.
func NewRuntime() *Runtime {
	return &Runtime{}
}

// StreamWriter sends SSE events with automatic sequence numbering and ID enrichment.
type StreamWriter struct {
	sse     *SSEWriter
	nextSeq func() int
	baseIDs dto.StreamEventIDs
}

// NewWriter creates a StreamWriter bound to an HTTP response and base IDs.
func (r *Runtime) NewWriter(w http.ResponseWriter, baseIDs dto.StreamEventIDs) *StreamWriter {
	sse := NewSSEWriter(w)
	outSeq := 0
	nextSeq := func() int {
		outSeq++
		return outSeq
	}
	return &StreamWriter{
		sse:     sse,
		nextSeq: nextSeq,
		baseIDs: baseIDs,
	}
}

// NewEvent builds a fully enriched StreamEvent with the next sequence number.
func (w *StreamWriter) NewEvent(channel string, eventType string, ids dto.StreamEventIDs, payload any, messageID string) (dto.StreamEvent, error) {
	event, err := dto.NewStreamEvent(0, channel, eventType, ids, payload)
	if err != nil {
		return dto.StreamEvent{}, err
	}
	return w.enrichEvent(event, messageID), nil
}

// EnrichEvent applies sequence number and fills missing IDs from base IDs.
func (w *StreamWriter) EnrichEvent(event dto.StreamEvent, messageID string) dto.StreamEvent {
	return w.enrichEvent(event, messageID)
}

// WriteEvent writes an already-enriched event.
func (w *StreamWriter) WriteEvent(_ context.Context, event dto.StreamEvent) error {
	return w.sse.WriteEvent(event)
}

// Send writes a pre-built StreamEvent after enriching it with sequence and missing IDs.
func (w *StreamWriter) Send(_ context.Context, event dto.StreamEvent, messageID string) error {
	event = w.enrichEvent(event, messageID)
	return w.sse.WriteEvent(event)
}

// SendNew creates and writes a new StreamEvent from components.
func (w *StreamWriter) SendNew(_ context.Context, channel string, eventType string, ids dto.StreamEventIDs, payload any, messageID string) error {
	event, err := w.NewEvent(channel, eventType, ids, payload, messageID)
	if err != nil {
		return err
	}
	return w.sse.WriteEvent(event)
}

// enrichEvent applies sequence number and fills missing IDs from base IDs.
func (w *StreamWriter) enrichEvent(event dto.StreamEvent, messageID string) dto.StreamEvent {
	event.Seq = w.nextSeq()
	event.Version = 1
	if event.IDs.ConversationID == "" {
		event.IDs.ConversationID = w.baseIDs.ConversationID
	}
	if event.IDs.RunID == "" {
		event.IDs.RunID = w.baseIDs.RunID
	}
	if event.IDs.TurnID == "" {
		event.IDs.TurnID = w.baseIDs.TurnID
	}
	if event.IDs.MessageID == "" {
		event.IDs.MessageID = messageID
	}
	if len(event.Payload) == 0 {
		event.Payload = json.RawMessage(`{}`)
	}
	return event
}
