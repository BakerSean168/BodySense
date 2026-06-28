package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// SSEWriter wraps an http.ResponseWriter to emit Server-Sent Events
// following the protocol defined in docs/plan/active/unified-session-redesign.md §6.
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

// ConversationCreated emits when a new conversation is created server-side.
func (s *SSEWriter) ConversationCreated(conversationID, replacesDraftID string) error {
	return s.sendEvent("conversation.created", map[string]string{
		"conversationId":  conversationID,
		"replacesDraftId": replacesDraftID,
	})
}

// MessagePersisted acknowledges that the client message has been stored.
func (s *SSEWriter) MessagePersisted(clientMessageID, messageID, role string) error {
	return s.sendEvent("message.persisted", map[string]string{
		"clientMessageId": clientMessageID,
		"messageId":       messageID,
		"role":            role,
	})
}

// MessageCreated signals the start of a new assistant message stream.
func (s *SSEWriter) MessageCreated(messageID, role, turnID string) error {
	return s.sendEvent("message.created", map[string]string{
		"messageId": messageID,
		"role":      role,
		"status":    "streaming",
		"turnId":    turnID,
	})
}

// TextDelta streams a text chunk for an in-progress message.
func (s *SSEWriter) TextDelta(messageID, delta string) error {
	return s.sendEvent("text.delta", map[string]string{
		"messageId": messageID,
		"delta":     delta,
	})
}

// ToolCall notifies the client that the assistant is invoking a tool.
func (s *SSEWriter) ToolCall(messageID, tool string, args any) error {
	return s.sendEvent("tool.call", map[string]any{
		"messageId": messageID,
		"tool":      tool,
		"args":      args,
	})
}

// ToolResult sends the result of a completed tool invocation.
func (s *SSEWriter) ToolResult(messageID, tool string, result any) error {
	return s.sendEvent("tool.result", map[string]any{
		"messageId": messageID,
		"tool":      tool,
		"result":    result,
	})
}

// ExtractedInfo delivers structured information extracted during the turn.
func (s *SSEWriter) ExtractedInfo(messageID string, info any) error {
	return s.sendEvent("extracted_info", map[string]any{
		"messageId": messageID,
		"info":      info,
	})
}

// PhaseChange notifies a consultation-phase transition.
func (s *SSEWriter) PhaseChange(messageID, from, to, reason string) error {
	return s.sendEvent("phase_change", map[string]string{
		"messageId": messageID,
		"from":      from,
		"to":        to,
		"reason":    reason,
	})
}

// Citation attaches a knowledge citation to the message stream.
func (s *SSEWriter) Citation(messageID string, citation any) error {
	return s.sendEvent("citation", map[string]any{
		"messageId": messageID,
		"citation":  citation,
	})
}

// RedFlag signals a clinical red-flag detection event.
func (s *SSEWriter) RedFlag(messageID string, flag any) error {
	return s.sendEvent("red_flag", map[string]any{
		"messageId": messageID,
		"flag":      flag,
	})
}

// MessageCompleted marks a message stream as finished.
func (s *SSEWriter) MessageCompleted(messageID string, usage any) error {
	return s.sendEvent("message.completed", map[string]any{
		"messageId":    messageID,
		"status":       "completed",
		"finishReason": "stop",
		"usage":        usage,
	})
}

// MessageFailed reports an error that terminated the message stream.
func (s *SSEWriter) MessageFailed(messageID string, errData any) error {
	return s.sendEvent("message.failed", map[string]any{
		"messageId": messageID,
		"status":    "failed",
		"error":     errData,
	})
}

// TitleGenerated delivers an auto-generated conversation title.
func (s *SSEWriter) TitleGenerated(conversationID, title string) error {
	return s.sendEvent("title.generated", map[string]string{
		"conversationId": conversationID,
		"title":          title,
	})
}

// Done signals the end of the SSE stream.
func (s *SSEWriter) Done() error {
	return s.sendEvent("done", map[string]string{})
}
