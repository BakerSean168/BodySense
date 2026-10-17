package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// UserUploadView is the assembled public projection of one user asset: the
// asset file identity plus the latest derived OCR / posture run state. It is
// a read model only — nothing persists it. It keeps the documented /uploads
// wire contract (id, file_type, ocr_*, analysis_*) stable while the storage
// model is the sole-identity UserAsset plus derived run tables; the
// asset-facing web views land with AS-001 slice 2.
type UserUploadView struct {
	ID             uuid.UUID             `json:"id"`
	UserID         uuid.UUID             `json:"-"`
	FileType       string                `json:"file_type"`
	OriginalName   string                `json:"original_name"`
	FileSize       int64                 `json:"file_size"`
	MimeType       string                `json:"mime_type"`
	OCRResult      json.RawMessage       `json:"ocr_result,omitempty"`
	OCRStatus      string                `json:"ocr_status"`
	AnalysisResult json.RawMessage       `json:"analysis_result,omitempty"`
	AnalysisStatus string                `json:"analysis_status"`
	Availability   UserAssetAvailability `json:"-"`
	CreatedAt      time.Time             `json:"created_at"`
	UpdatedAt      time.Time             `json:"updated_at"`
}
