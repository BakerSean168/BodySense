package dto

import "encoding/json"

// StartConsultationRunRequest is the request to start a new consultation run.
// If ConversationID is nil, a new conversation is created.
type StartConsultationRunRequest struct {
	ConversationID  *string    `json:"conversationId"`
	ClientMessageID string     `json:"clientMessageId" binding:"required,max=128"`
	RequestID       string     `json:"requestId" binding:"required,max=128"`
	Message         MessageDTO `json:"message" binding:"required"`
}

type ResumeConsultationInteractionRequest struct {
	RequestID string          `json:"requestId" binding:"required,max=128"`
	Answer    json.RawMessage `json:"answer" binding:"required"`
}
