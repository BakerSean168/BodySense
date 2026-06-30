package stream

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/bodysense/api/internal/dto"
)

// SSEWriter wraps an http.ResponseWriter to emit Server-Sent Events.
type SSEWriter struct {
	w     http.ResponseWriter
	flush func()
}

// NewSSEWriter initialises SSE headers and returns a writer ready to stream.
func NewSSEWriter(w http.ResponseWriter) *SSEWriter {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	flusher, ok := w.(http.Flusher)
	if !ok {
		panic("ResponseWriter does not implement http.Flusher")
	}

	return &SSEWriter{w: w, flush: flusher.Flush}
}

// sendEvent marshals data as JSON and writes a single SSE frame.
func (s *SSEWriter) sendEvent(event string, data any) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", event, string(jsonData))
	s.flush()
	return nil
}

// WriteEvent writes a structured stream event as one SSE frame.
func (s *SSEWriter) WriteEvent(event dto.StreamEvent) error {
	if event.Version == 0 {
		event.Version = 1
	}
	if len(event.Payload) == 0 {
		event.Payload = json.RawMessage(`{}`)
	}
	return s.sendEvent(event.Type, event)
}
