package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// DiagnosisAnalysis is an immutable analytical artifact pinned to one exact
// BodyState revision. Later health changes never rewrite this row.
type DiagnosisAnalysisRecord struct {
	ID                       uuid.UUID      `gorm:"type:uuid;primaryKey" json:"analysis_id"`
	UserID                   uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	BodyStateRevision        int64          `gorm:"not null;index" json:"body_state_revision"`
	Status                   string         `gorm:"type:varchar(40);not null" json:"status"`
	Scope                    string         `gorm:"type:varchar(40);not null;default:'full_body'" json:"scope"`
	Summary                  string         `gorm:"type:text;not null;default:''" json:"summary"`
	CrossConcernPatterns     datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"cross_concern_patterns"`
	InformationGaps          datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"information_gaps"`
	SafetySummary            datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"safety_summary"`
	Citations                datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"citations"`
	Governance               datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"governance"`
	AgentConfigurationID     string         `gorm:"type:varchar(80);not null;default:'';index" json:"agent_configuration_id"`
	AgentConfiguration       datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"agent_configuration"`
	DecisionTrace            datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"decision_trace"`
	ExecutionProvenance      datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"execution_provenance"`
	EvidenceAcquisitionTrace datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"evidence_acquisition_trace"`
	ReplayInput              datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"-"`
	RawOutput                datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"-"`
	CreatedAt                time.Time      `gorm:"not null;default:now()" json:"created_at"`

	Candidates []DiagnosisCandidateRecord `gorm:"-" json:"candidates"`
}

func (DiagnosisAnalysisRecord) TableName() string { return "diagnosis_analyses" }

// DiagnosisCandidateRecord is application-owned durable identity for one AI
// candidate. The Python model proposes content; Go assigns ID + ordinal.
type DiagnosisCandidateRecord struct {
	ID                    uuid.UUID      `gorm:"type:uuid;primaryKey" json:"candidate_id"`
	AnalysisID            uuid.UUID      `gorm:"type:uuid;not null;index" json:"analysis_id"`
	Ordinal               int            `gorm:"not null" json:"ordinal"`
	ConcernKey            string         `gorm:"type:varchar(120);not null;default:''" json:"concern_key,omitempty"`
	Name                  string         `gorm:"type:text;not null" json:"name"`
	Confidence            string         `gorm:"type:varchar(20);not null" json:"confidence"`
	Severity              *string        `gorm:"type:varchar(20)" json:"severity,omitempty"`
	EvidenceStrength      *string        `gorm:"type:varchar(20)" json:"evidence_strength,omitempty"`
	Impact                *string        `gorm:"type:text" json:"impact,omitempty"`
	Basis                 string         `gorm:"type:text;not null;default:''" json:"basis"`
	TypicalSymptoms       string         `gorm:"type:text;not null;default:''" json:"typical_symptoms"`
	Differential          *string        `gorm:"type:text" json:"differential,omitempty"`
	BasisFactIDs          datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"basis_fact_ids"`
	BasisObservationIDs   datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"basis_observation_ids"`
	SupportingEvidenceIDs datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"supporting_evidence_ids"`
	CounterevidenceIDs    datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"counterevidence_ids"`
	ReasoningSummary      string         `gorm:"type:text;not null;default:''" json:"reasoning_summary"`
	MissingInformation    datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"missing_information"`
	SafetyNotes           datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"safety_notes"`
	RawPayload            datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"-"`
	CreatedAt             time.Time      `gorm:"not null;default:now()" json:"created_at"`
}

func (DiagnosisCandidateRecord) TableName() string { return "diagnosis_candidates" }

type DiagnosisCandidateAssessment struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	AnalysisID  uuid.UUID `gorm:"type:uuid;not null;index" json:"analysis_id"`
	CandidateID uuid.UUID `gorm:"type:uuid;not null;index" json:"candidate_id"`
	UserID      uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	State       string    `gorm:"type:varchar(30);not null" json:"state"`
	AssessedAt  time.Time `gorm:"not null;default:now()" json:"assessed_at"`
}

func (DiagnosisCandidateAssessment) TableName() string { return "diagnosis_candidate_assessments" }
