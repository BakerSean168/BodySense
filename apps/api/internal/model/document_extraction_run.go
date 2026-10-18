package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// DocumentExtractionRun is the derived OCR/extraction run of one report asset.
// It references the asset through the composite (user_id, asset_id)
// owner-scoped foreign key and owns the whole OCR lifecycle
// (pending -> processing -> completed | failed). Indicator snapshots and the
// source summary are the durable minimal evidence fragments that survive
// source removal; result_payload is the validated response cache of this run.
type DocumentExtractionRun struct {
	ID              uuid.UUID        `gorm:"type:uuid;primaryKey;default:uuidv7()" json:"id"`
	UserID          uuid.UUID        `gorm:"type:uuid;not null;index" json:"user_id"`
	AssetID         uuid.UUID        `gorm:"type:uuid;not null;index" json:"asset_id"`
	Status          DerivedRunStatus `gorm:"type:varchar(20);not null;default:'pending'" json:"status"`
	JobID           *uuid.UUID       `gorm:"type:uuid;index" json:"job_id,omitempty"`
	ConfigurationID string           `gorm:"type:varchar(80);not null;index" json:"configuration_id"`
	// The audit identity is filled exactly at completion; pending and
	// processing rows keep these columns NULL (never empty strings, which
	// would violate the hex CHECK constraints).
	MechanismRevision   *string        `gorm:"type:varchar(120)" json:"mechanism_revision,omitempty"`
	DocumentSHA256      *string        `gorm:"type:char(64)" json:"document_sha256,omitempty"`
	ResultSHA256        *string        `gorm:"type:char(64)" json:"result_sha256,omitempty"`
	RawTextSHA256       *string        `gorm:"type:char(64)" json:"raw_text_sha256,omitempty"`
	ResultPayload       datatypes.JSON `gorm:"type:jsonb" json:"result_payload,omitempty"`
	IndicatorSnapshot   datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"indicator_snapshot"`
	SourceSummary       datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"source_summary"`
	MechanismProvenance datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"mechanism_provenance"`
	CreatedAt           time.Time      `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt           time.Time      `gorm:"not null;default:now()" json:"updated_at"`
	// ErasedAt marks a tombstone kept only because a withdrawn Diagnosis
	// condition still links to one of its reviews (AS-002); content is gone.
	ErasedAt *time.Time `json:"-"`
}

func (DocumentExtractionRun) TableName() string { return "document_extraction_runs" }
