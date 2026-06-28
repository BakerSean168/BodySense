package dto

import "encoding/json"

// UpdateExtractedInfoRequest is the request to update extracted info for a session.
type UpdateExtractedInfoRequest struct {
	ExtractedInfo json.RawMessage `json:"extracted_info" binding:"required"`
}

// ConfirmDiagnosisRequest is the request to confirm a diagnosis.
type ConfirmDiagnosisRequest struct {
	Diagnosis json.RawMessage `json:"diagnosis" binding:"required"`
}
