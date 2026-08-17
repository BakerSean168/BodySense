package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// AssessmentReport is an immutable observation report. It never owns Diagnosis
// or Treatment truth; projected observations remain unverified in BodyState until
// the user explicitly confirms them.
type AssessmentReport struct {
	ID                uuid.UUID       `gorm:"type:uuid;primaryKey" json:"id"`
	UserID            uuid.UUID       `gorm:"type:uuid;index;not null" json:"user_id"`
	Status            string          `gorm:"type:varchar(40);not null" json:"status"`
	HealthGrade       string          `gorm:"type:varchar(5);not null" json:"health_grade"`
	DimensionScores   json.RawMessage `gorm:"type:jsonb;not null;default:'{}'" json:"dimension_scores"`
	Observations      json.RawMessage `gorm:"type:jsonb;not null;default:'[]'" json:"observations"`
	Summary           string          `gorm:"type:text;not null;default:''" json:"summary"`
	InformationGaps   json.RawMessage `gorm:"type:jsonb;not null;default:'[]'" json:"information_gaps"`
	SafetyNotes       json.RawMessage `gorm:"type:jsonb;not null;default:'[]'" json:"safety_notes"`
	BodyStateRevision *int64          `json:"body_state_revision,omitempty"`
	CreatedAt         time.Time       `gorm:"not null;default:now()" json:"created_at"`
}

func (AssessmentReport) TableName() string { return "assessment_reports" }
