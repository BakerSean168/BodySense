package consultation

import "encoding/json"

// MessageInput is the runtime-facing user message contract. HTTP transports
// map their validated request into this type before durable execution begins.
type MessageInput struct {
	Role     string
	Parts    []PartInput
	Metadata json.RawMessage
}

// PartInput is a single runtime message part. Image bytes are never inlined;
// UploadID references the Go-owned upload resource resolved during the turn.
type PartInput struct {
	Type     string
	Text     string
	UploadID string
	MimeType string
	ImageURL string
}

// StartRunInput is the application/runtime command for a consultation turn.
type StartRunInput struct {
	ConversationID  *string
	ClientMessageID string
	RequestID       string
	Message         MessageInput
}

// ResumeInteractionInput continues one exact durable Agent interaction.
type ResumeInteractionInput struct {
	RequestID string
	Answer    json.RawMessage
}
