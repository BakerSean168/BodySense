package dto

// CreateSessionRequest is the request to create a new consultation session.
type CreateSessionRequest struct {
	// Empty for now - session is created with defaults
}

// SendMessageRequest is the request to send a message in a consultation session.
type SendMessageRequest struct {
	Content string `json:"content" binding:"required"`
}

// UpdateExtractedInfoRequest is the request to update extracted info for a session.
type UpdateExtractedInfoRequest struct {
	ExtractedInfo any `json:"extracted_info" binding:"required"`
}

// ConfirmDiagnosisRequest is the request to confirm a diagnosis.
type ConfirmDiagnosisRequest struct {
	Diagnosis any `json:"diagnosis" binding:"required"`
}
