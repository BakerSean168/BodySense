package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// Intervention is one executable unit from an accepted TreatmentRevision.
type Intervention struct {
	ID                  uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID              uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	TreatmentID         uuid.UUID      `gorm:"type:uuid;not null;index" json:"treatment_id"`
	TreatmentRevisionID uuid.UUID      `gorm:"type:uuid;not null;index" json:"treatment_revision_id"`
	Kind                string         `gorm:"type:varchar(40);not null" json:"kind"`
	Title               string         `gorm:"type:text;not null" json:"title"`
	Description         string         `gorm:"type:text;not null;default:''" json:"description"`
	Prescription        datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"prescription"`
	Position            int            `gorm:"not null;default:0" json:"position"`
	Status              string         `gorm:"type:varchar(20);not null" json:"status"`
	StartedAt           *time.Time     `json:"started_at,omitempty"`
	EndedAt             *time.Time     `json:"ended_at,omitempty"`
	CreatedAt           time.Time      `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt           time.Time      `gorm:"not null;default:now()" json:"updated_at"`
}

func (Intervention) TableName() string { return "interventions" }

// Outcome records what happened after an intervention. CausalityLevel defaults
// to association_only so temporal sequence is not misrepresented as proof.
type Outcome struct {
	ID                   uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID               uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	TreatmentID          *uuid.UUID     `gorm:"type:uuid" json:"treatment_id,omitempty"`
	TreatmentRevisionID  *uuid.UUID     `gorm:"type:uuid" json:"treatment_revision_id,omitempty"`
	InterventionID       *uuid.UUID     `gorm:"type:uuid" json:"intervention_id,omitempty"`
	SourceType           string         `gorm:"type:varchar(40);not null" json:"source_type"`
	SourceKey            string         `gorm:"type:text;not null" json:"source_key"`
	Kind                 string         `gorm:"type:varchar(50);not null" json:"kind"`
	ConcernKey           string         `gorm:"type:varchar(120);not null;default:'general'" json:"concern_key"`
	BodyRegion           string         `gorm:"type:text;not null;default:''" json:"body_region"`
	Value                datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"value"`
	Notes                string         `gorm:"type:text;not null;default:''" json:"notes"`
	AssociationStatement string         `gorm:"type:text;not null;default:''" json:"association_statement"`
	CausalityLevel       string         `gorm:"type:varchar(30);not null;default:'association_only'" json:"causality_level"`
	OccurredAt           time.Time      `gorm:"not null" json:"occurred_at"`
	Provenance           datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"provenance"`
	BodyStateRevision    *int64         `json:"body_state_revision,omitempty"`
	CreatedAt            time.Time      `gorm:"not null;default:now()" json:"created_at"`
}

func (Outcome) TableName() string { return "outcomes" }
