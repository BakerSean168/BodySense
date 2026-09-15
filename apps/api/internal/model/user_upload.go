package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// UploadOCRStatus is the finite OCR lifecycle for one upload.
type UploadOCRStatus string

const (
	UploadOCRPending    UploadOCRStatus = "pending"
	UploadOCRProcessing UploadOCRStatus = "processing"
	UploadOCRCompleted  UploadOCRStatus = "completed"
	UploadOCRFailed     UploadOCRStatus = "failed"
)

func (s UploadOCRStatus) IsTerminal() bool {
	return s == UploadOCRCompleted || s == UploadOCRFailed
}

// UploadAnalysisStatus is the finite posture-analysis lifecycle for one upload.
type UploadAnalysisStatus string

const (
	UploadAnalysisNone       UploadAnalysisStatus = "none"
	UploadAnalysisPending    UploadAnalysisStatus = "pending"
	UploadAnalysisProcessing UploadAnalysisStatus = "processing"
	UploadAnalysisCompleted  UploadAnalysisStatus = "completed"
	UploadAnalysisFailed     UploadAnalysisStatus = "failed"
)

func (s UploadAnalysisStatus) IsTerminal() bool {
	return s == UploadAnalysisCompleted || s == UploadAnalysisFailed
}

// UserUpload represents a user's uploaded file (photo or health report).
type UserUpload struct {
	ID             uuid.UUID       `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	UserID         uuid.UUID       `gorm:"type:uuid;not null" json:"user_id"`
	FileType       string          `gorm:"type:varchar(50);not null" json:"file_type"`
	OriginalName   string          `gorm:"type:varchar(255);not null" json:"original_name"`
	StorageBackend string          `gorm:"type:varchar(20);not null" json:"-"`
	StorageKey     string          `gorm:"type:varchar(500);not null" json:"-"`
	FileSize       int64           `gorm:"not null" json:"file_size"`
	MimeType       string          `gorm:"type:varchar(100);not null" json:"mime_type"`
	OCRResult      json.RawMessage `gorm:"type:jsonb" json:"ocr_result,omitempty"`
	OCRStatus      UploadOCRStatus `gorm:"type:varchar(20);not null;default:'pending'" json:"ocr_status"`
	// AnalysisResult holds the structured posture-analysis payload for photo
	// uploads (see docs/plan/archive/posture-photo-analysis-plan.md §3). It is
	// deliberately separate from OCRResult, which is report-specific.
	AnalysisResult json.RawMessage      `gorm:"type:jsonb" json:"analysis_result,omitempty"`
	AnalysisStatus UploadAnalysisStatus `gorm:"type:varchar(20);not null;default:'none'" json:"analysis_status"`
	// North-Star: exact immutable Agent configuration used for this analysis.
	AgentConfigurationID string    `gorm:"type:varchar(80);not null;default:'';index" json:"agent_configuration_id"`
	CreatedAt            time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt            time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

// TableName specifies the table name for GORM.
func (UserUpload) TableName() string {
	return "user_uploads"
}
