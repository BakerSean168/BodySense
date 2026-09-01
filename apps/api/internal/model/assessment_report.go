package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// AssessmentReport is an immutable observation report. It never owns Diagnosis
// or Treatment truth; projected observations remain unverified in BodyState until
// the user explicitly confirms them. New reports use the evidence-grounded v2
// contract. Legacy health grade / dimension scores remain nullable only so
// historical v1 reports can be replayed without rewriting their provenance.
type AssessmentReport struct {
	ID                      uuid.UUID       `gorm:"type:uuid;primaryKey" json:"id"`
	UserID                  uuid.UUID       `gorm:"type:uuid;index;not null" json:"user_id"`
	Status                  string          `gorm:"type:varchar(40);not null" json:"status"`
	ContractRevision        string          `gorm:"type:varchar(80);not null;default:'assessment-output-v1'" json:"contract_revision"`
	HealthGrade             *string         `gorm:"type:varchar(5)" json:"health_grade,omitempty"`
	DimensionScores         json.RawMessage `gorm:"type:jsonb" json:"dimension_scores,omitempty"`
	EvidenceCoverage        json.RawMessage `gorm:"type:jsonb;not null;default:'{}'" json:"evidence_coverage"`
	EvidenceGaps            json.RawMessage `gorm:"type:jsonb;not null;default:'[]'" json:"evidence_gaps"`
	Observations            json.RawMessage `gorm:"type:jsonb;not null;default:'[]'" json:"observations"`
	Summary                 string          `gorm:"type:text;not null;default:''" json:"summary"`
	InformationGaps         json.RawMessage `gorm:"type:jsonb;not null;default:'[]'" json:"information_gaps,omitempty"`
	SafetyNotes             json.RawMessage `gorm:"type:jsonb;not null;default:'[]'" json:"safety_notes"`
	BodyStateRevision       *int64          `json:"body_state_revision,omitempty"`
	AgentConfigurationID    string          `gorm:"type:varchar(80);not null;default:'';index" json:"agent_configuration_id"`
	AgentConfiguration      datatypes.JSON  `gorm:"type:jsonb;not null;default:'{}'" json:"agent_configuration"`
	ExecutionProvenance     datatypes.JSON  `gorm:"type:jsonb;not null;default:'{}'" json:"execution_provenance"`
	GenerationDecisionTrace datatypes.JSON  `gorm:"type:jsonb;not null;default:'{}'" json:"generation_decision_trace"`
	ReplayInput             datatypes.JSON  `gorm:"type:jsonb;not null;default:'{}';column:replay_input" json:"-"`
	CreatedAt               time.Time       `gorm:"not null;default:now()" json:"created_at"`
}

func (AssessmentReport) TableName() string { return "assessment_reports" }
