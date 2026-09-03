package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// ReviewAction enumerates the append-only human actions a reviewer may apply
// to a machine-extracted indicator candidate.
type ReviewAction string

const (
	ReviewActionConfirm ReviewAction = "confirm"
	ReviewActionCorrect ReviewAction = "correct"
	ReviewActionReject  ReviewAction = "reject"
)

// DocumentIndicatorReview is an immutable, append-only record of a reviewer's
// decision about one machine-extracted indicator candidate. It is bound to an
// exact document_extraction_runs row plus indicator index so playback is
// unambiguous, and it snapshots both the machine candidate and the reviewed
// payload so later re-extraction or OCRResult mutation cannot rewrite history.
type DocumentIndicatorReview struct {
	ID               uuid.UUID      `gorm:"type:uuid;primaryKey;default:uuidv7()" json:"id"`
	UserID           uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	UploadID         uuid.UUID      `gorm:"type:uuid;not null;index" json:"upload_id"`
	ExtractionRunID  uuid.UUID      `gorm:"type:uuid;not null;index" json:"extraction_run_id"`
	IndicatorIndex   int            `gorm:"not null" json:"indicator_index"`
	Action           string         `gorm:"type:varchar(20);not null" json:"action"`
	IdempotencyKey   string         `gorm:"type:varchar(128);not null" json:"idempotency_key"`
	ReviewedPayload  datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"reviewed_payload,omitempty"`
	MachineCandidate datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"machine_candidate,omitempty"`
	SourceRefs       datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"source_refs,omitempty"`
	PageRef          datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"page_ref,omitempty"`
	ReviewerUserID   uuid.UUID      `gorm:"type:uuid;not null" json:"reviewer_user_id"`
	Note             string         `gorm:"type:text;not null;default:''" json:"note"`
	CreatedAt        time.Time      `gorm:"not null;default:now()" json:"created_at"`
}

// TableName specifies the table name for GORM.
func (DocumentIndicatorReview) TableName() string { return "document_indicator_reviews" }
