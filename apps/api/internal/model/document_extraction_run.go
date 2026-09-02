package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// DocumentExtractionRun is an append-only audit record for one health-document
// extraction execution. Raw report text stays in the current upload projection;
// the run stores only hashes, structured candidates/source refs and provenance.
type DocumentExtractionRun struct {
	ID                  uuid.UUID      `gorm:"type:uuid;primaryKey;default:uuidv7()" json:"id"`
	UploadID            uuid.UUID      `gorm:"type:uuid;not null;index" json:"upload_id"`
	UserID              uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	JobID               *uuid.UUID     `gorm:"type:uuid;index" json:"job_id,omitempty"`
	ConfigurationID     string         `gorm:"type:varchar(80);not null;index" json:"configuration_id"`
	MechanismRevision   string         `gorm:"type:varchar(120);not null" json:"mechanism_revision"`
	DocumentSHA256      string         `gorm:"type:char(64);not null" json:"document_sha256"`
	ResultSHA256        string         `gorm:"type:char(64);not null" json:"result_sha256"`
	RawTextSHA256       string         `gorm:"type:char(64);not null" json:"raw_text_sha256"`
	IndicatorSnapshot   datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"indicator_snapshot"`
	SourceSummary       datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"source_summary"`
	MechanismProvenance datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"mechanism_provenance"`
	CreatedAt           time.Time      `gorm:"not null;default:now()" json:"created_at"`
}

func (DocumentExtractionRun) TableName() string { return "document_extraction_runs" }
