package dto

import "encoding/json"

type CreateConsultationRequest struct{}

type SendConsultationMessageRequest struct {
	ClientMessageID string     `json:"clientMessageId" binding:"required,max=128"`
	RequestID       string     `json:"requestId" binding:"required,max=128"`
	Message         MessageDTO `json:"message" binding:"required"`
}

type ResumeConsultationInteractionRequest struct {
	RequestID string          `json:"requestId" binding:"required,max=128"`
	Answer    json.RawMessage `json:"answer" binding:"required"`
}

// UpdateExtractedInfoRequest is the request to update extracted info for a session.
type UpdateExtractedInfoRequest struct {
	ExtractedInfo json.RawMessage `json:"extracted_info" binding:"required"`
}

// ConfirmDiagnosisRequest is the request to confirm a diagnosis.
type ConfirmDiagnosisRequest struct {
	Diagnosis json.RawMessage `json:"diagnosis" binding:"required"`
}
