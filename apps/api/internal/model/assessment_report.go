package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// AssessmentReport is an immutable observation report. It never owns Diagnosis
// or Treatment truth; projected observations remain unverified in BodyState until
// the user explicitly confirms them. It records the exact immutable Assessment
// Agent configuration + execution provenance + generation decision trace that
// produced the report (North-Star platform parity with Diagnosis/Treatment).
type AssessmentReport struct {
	ID                      uuid.UUID       `gorm:"type:uuid;primaryKey" json:"id"`
	UserID                  uuid.UUID       `gorm:"type:uuid;index;not null" json:"user_id"`
	Status                  string          `gorm:"type:varchar(40);not null" json:"status"`
	HealthGrade             string          `gorm:"type:varchar(5);not null" json:"health_grade"`
	DimensionScores         json.RawMessage `gorm:"type:jsonb;not null;default:'{}'" json:"dimension_scores"`
	Observations            json.RawMessage `gorm:"type:jsonb;not null;default:'[]'" json:"observations"`
	Summary                 string          `gorm:"type:text;not null;default:''" json:"summary"`
	InformationGaps         json.RawMessage `gorm:"type:jsonb;not null;default:'[]'" json:"information_gaps"`
	SafetyNotes             json.RawMessage `gorm:"type:jsonb;not null;default:'[]'" json:"safety_notes"`
	BodyStateRevision       *int64          `json:"body_state_revision,omitempty"`
	AgentConfigurationID    string          `gorm:"type:varchar(80);not null;default:'';index" json:"agent_configuration_id"`
	AgentConfiguration      datatypes.JSON  `gorm:"type:jsonb;not null;default:'{}'" json:"agent_configuration"`
	ExecutionProvenance     datatypes.JSON  `gorm:"type:jsonb;not null;default:'{}'" json:"execution_provenance"`
	GenerationDecisionTrace datatypes.JSON  `gorm:"type:jsonb;not null;default:'{}'" json:"generation_decision_trace"`
	CreatedAt               time.Time       `gorm:"not null;default:now()" json:"created_at"`
}

func (AssessmentReport) TableName() string { return "assessment_reports" }
