package dto

import "encoding/json"

type MessageDTO struct {
	Role     string          `json:"role"`
	Parts    []PartDTO       `json:"parts" binding:"required,min=1,dive"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// PartDTO is a single message part. Text parts carry `text`; image parts carry
// `upload_id` (preferred) and optional display metadata. Image bytes are never
// inlined in the persisted part — Go resolves upload_id at turn time.
type PartDTO struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	UploadID string `json:"upload_id,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
	// Optional client-side preview URL (not trusted by the backend).
	ImageURL string `json:"image_url,omitempty"`
}

type PinRequest struct {
	Pinned bool `json:"pinned"`
}

type RenameRequest struct {
	Title string `json:"title" binding:"required,max=200"`
}
