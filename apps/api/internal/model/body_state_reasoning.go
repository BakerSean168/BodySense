package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// BodyStateHypothesis is an AI-generated explanatory possibility. It remains
// epistemically separate from accepted Facts and can strengthen or weaken over time.
type BodyStateHypothesis struct {
	ID                       uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID                   uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	ConcernKey               string         `gorm:"type:varchar(120);not null;default:'general'" json:"concern_key"`
	Statement                string         `gorm:"type:text;not null" json:"statement"`
	LifecycleState           string         `gorm:"type:varchar(30);not null;default:'active'" json:"lifecycle_state"`
	Confidence               *string        `gorm:"type:varchar(20)" json:"confidence,omitempty"`
	SupportingFactIDs        datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"supporting_fact_ids"`
	SupportingObservationIDs datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"supporting_observation_ids"`
	SupportingEvidenceIDs    datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"supporting_evidence_ids"`
	CounterevidenceIDs       datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"counterevidence_ids"`
	SourceAnalysisID         *uuid.UUID     `gorm:"type:uuid" json:"source_analysis_id,omitempty"`
	Provenance               datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"provenance"`
	CreatedRevision          int64          `gorm:"not null" json:"created_revision"`
	UpdatedRevision          int64          `gorm:"not null" json:"updated_revision"`
	CreatedAt                time.Time      `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt                time.Time      `gorm:"not null;default:now()" json:"updated_at"`
}

func (BodyStateHypothesis) TableName() string { return "body_state_hypotheses" }

// BodyStateEvidence preserves source identity and a minimal retrieved snapshot.
// Evidence supports reasoning; it never becomes a user Fact merely by being stored.
type BodyStateEvidence struct {
	ID            uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID        uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	SourceType    string         `gorm:"type:varchar(40);not null" json:"source_type"`
	SourceKey     string         `gorm:"type:text;not null" json:"source_key"`
	SourceVersion string         `gorm:"type:text;not null;default:''" json:"source_version"`
	Title         string         `gorm:"type:text;not null;default:''" json:"title"`
	Summary       string         `gorm:"type:text;not null;default:''" json:"summary"`
	Excerpt       string         `gorm:"type:text;not null;default:''" json:"excerpt"`
	Metadata      datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"metadata"`
	RetrievedAt   time.Time      `gorm:"not null;default:now()" json:"retrieved_at"`
	CreatedAt     time.Time      `gorm:"not null;default:now()" json:"created_at"`
}

func (BodyStateEvidence) TableName() string { return "body_state_evidence" }
