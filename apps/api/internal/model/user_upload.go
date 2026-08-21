package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// UserUpload represents a user's uploaded file (photo or health report).
type UserUpload struct {
	ID           uuid.UUID       `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	UserID       uuid.UUID       `gorm:"type:uuid;not null" json:"user_id"`
	FileType     string          `gorm:"type:varchar(50);not null" json:"file_type"`
	OriginalName string          `gorm:"type:varchar(255);not null" json:"original_name"`
	FilePath     string          `gorm:"type:varchar(500);not null" json:"file_path"`
	FileSize     int64           `gorm:"not null" json:"file_size"`
	MimeType     string          `gorm:"type:varchar(100);not null" json:"mime_type"`
	OCRResult    json.RawMessage `gorm:"type:jsonb" json:"ocr_result,omitempty"`
	OCRStatus    string          `gorm:"type:varchar(20);not null;default:'pending'" json:"ocr_status"`
	// AnalysisResult holds the structured posture-analysis payload for photo
	// uploads (see docs/plan/active/posture-photo-analysis-plan.md §3). It is
	// deliberately separate from OCRResult, which is report-specific.
	AnalysisResult json.RawMessage `gorm:"type:jsonb" json:"analysis_result,omitempty"`
	AnalysisStatus string          `gorm:"type:varchar(20);not null;default:'none'" json:"analysis_status"`
	// North-Star: exact immutable Agent configuration used for this analysis.
	AgentConfigurationID string    `gorm:"type:varchar(80);not null;default:'';index" json:"agent_configuration_id"`
	CreatedAt            time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt            time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

// TableName specifies the table name for GORM.
func (UserUpload) TableName() string {
	return "user_uploads"
}
