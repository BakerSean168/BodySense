package model

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// ErrDiagnosisNotFound deliberately masks absent and foreign identities alike.
var ErrDiagnosisNotFound = errors.New("diagnosis record not found")

// Diagnosis is one aggregate per owner. Only a committed revision changes its head.
type Diagnosis struct {
	UserID          uuid.UUID `gorm:"primaryKey"`
	CurrentRevision int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (Diagnosis) TableName() string { return "diagnoses" }

type DiagnosisLevel string

const (
	DiagnosisSuspected DiagnosisLevel = "suspected"
	DiagnosisLikely    DiagnosisLevel = "likely"
)

type ConditionDisposition string

const (
	ConditionAdopted  ConditionDisposition = "adopted"
	ConditionInactive ConditionDisposition = "inactive"
)

type DiagnosisSourceKind string

const (
	DiagnosisUserSuspicion        DiagnosisSourceKind = "user_suspicion"
	DiagnosisAgentAnalysis        DiagnosisSourceKind = "agent_analysis"
	DiagnosisUserReportedClinical DiagnosisSourceKind = "user_reported_clinical"
	DiagnosisClinicalDocument     DiagnosisSourceKind = "clinical_document"
)

type DiagnosisSourceValidity string

const (
	DiagnosisSourceCurrent    DiagnosisSourceValidity = "current"
	DiagnosisSourceCorrected  DiagnosisSourceValidity = "corrected"
	DiagnosisSourceWithdrawn  DiagnosisSourceValidity = "withdrawn"
	DiagnosisSourceIneligible DiagnosisSourceValidity = "ineligible"
)

type ConditionAnalysisTrigger string

const (
	ConditionNewAnalysis    ConditionAnalysisTrigger = "new_analysis"
	ConditionPartialUpdate  ConditionAnalysisTrigger = "partial_update"
	ConditionFullReanalysis ConditionAnalysisTrigger = "full_reanalysis"
)

type DiagnosisEvidenceType string

const (
	DiagnosisBodyFact         DiagnosisEvidenceType = "body_fact"
	DiagnosisBodyObservation  DiagnosisEvidenceType = "body_observation"
	DiagnosisSymptomEpisode   DiagnosisEvidenceType = "symptom_episode"
	DiagnosisDocumentEvidence DiagnosisEvidenceType = "document_evidence"
)

type DiagnosisEvidenceRole string

const (
	DiagnosisSupporting DiagnosisEvidenceRole = "supporting"
	DiagnosisCounter    DiagnosisEvidenceRole = "counter"
	DiagnosisContext    DiagnosisEvidenceRole = "context"
)

type DiagnosisDecisionKind string

const (
	DiagnosisAccept DiagnosisDecisionKind = "accept"
	DiagnosisReject DiagnosisDecisionKind = "reject"
)

type DiagnosisChangeKind string

const (
	DiagnosisCreate         DiagnosisChangeKind = "create"
	DiagnosisUpdate         DiagnosisChangeKind = "update"
	DiagnosisSetDisposition DiagnosisChangeKind = "set_disposition"
	DiagnosisUpdateRelation DiagnosisChangeKind = "update_relation"
)

type MedicalConceptRef struct {
	System  string `json:"system"`
	Code    string `json:"code"`
	Display string `json:"display,omitempty"`
}
type ConditionQualifiers struct {
	RegionIDs  []string `json:"regionIds,omitempty"`
	Laterality string   `json:"laterality,omitempty"`
	Notes      string   `json:"notes,omitempty"`
}

// DiagnosisEvidenceRef versions are required frozen source versions, not live lookups.
type DiagnosisEvidenceRef struct {
	EvidenceType DiagnosisEvidenceType `json:"evidenceType"`
	EvidenceID   uuid.UUID             `json:"evidenceId"`
	Version      string                `json:"version"`
}
type DiagnosisSourceClaimRef struct {
	ClaimID              string                  `json:"claimId"`
	SourceKind           DiagnosisSourceKind     `json:"sourceKind"`
	SourceID             string                  `json:"sourceId"`
	SourceVersion        string                  `json:"sourceVersion"`
	Statement            string                  `json:"statement"`
	AssertedAt           *time.Time              `json:"assertedAt,omitempty"`
	RecordedAt           time.Time               `json:"recordedAt"`
	EvidenceAdmissionID  string                  `json:"evidenceAdmissionId,omitempty"`
	ValidityAtAssessment DiagnosisSourceValidity `json:"validityAtAssessment"`
}
type ConditionAssessment struct {
	Level             DiagnosisLevel `json:"level"`
	Rationale         string         `json:"rationale"`
	InformationGaps   []string       `json:"informationGaps"`
	DifferentialNotes []string       `json:"differentialNotes"`
	BasedOnRunID      uuid.UUID      `json:"basedOnRunId"`
}
type ConditionCauseAnalysis struct {
	Summary   string `json:"summary"`
	Rationale string `json:"rationale"`
}
type ConditionHandlingPlan struct {
	Summary      string   `json:"summary"`
	Actions      []string `json:"actions"`
	CautionNotes []string `json:"cautionNotes"`
}

// ConditionEntry is a committed projection. No repository adoption writer is
// exposed until the Go decision/Safety transaction is implemented in slice 2.
// JSON serializers preserve optional clinical-only entries without AI assessment.
type ConditionEntry struct {
	UserID            uuid.UUID `gorm:"primaryKey"`
	ID                uuid.UUID `gorm:"primaryKey"`
	MedicalName       string
	ConceptRef        *MedicalConceptRef        `gorm:"serializer:json;type:jsonb"`
	Qualifiers        *ConditionQualifiers      `gorm:"serializer:json;type:jsonb"`
	Assessment        *ConditionAssessment      `gorm:"serializer:json;type:jsonb"`
	SourceClaimRefs   []DiagnosisSourceClaimRef `gorm:"serializer:json;type:jsonb"`
	CauseAnalysis     ConditionCauseAnalysis    `gorm:"serializer:json;type:jsonb"`
	InformationGaps   []string                  `gorm:"serializer:json;type:jsonb"`
	HandlingPlan      ConditionHandlingPlan     `gorm:"serializer:json;type:jsonb"`
	Disposition       ConditionDisposition
	LatestDecisionID  uuid.UUID
	LastAnalysisRunID *uuid.UUID
	CommittedRevision int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (ConditionEntry) TableName() string { return "condition_entries" }

type ConditionEntryEvidence struct {
	UserID               uuid.UUID             `gorm:"primaryKey"`
	ConditionID          uuid.UUID             `gorm:"primaryKey"`
	EvidenceType         DiagnosisEvidenceType `gorm:"primaryKey"`
	EvidenceID           uuid.UUID             `gorm:"primaryKey"`
	EvidenceVersion      int64                 `gorm:"primaryKey"`
	Role                 DiagnosisEvidenceRole `gorm:"primaryKey"`
	Rationale            string
	AnalyzedByRunID      uuid.UUID
	DocumentReviewAction *string
}

func (ConditionEntryEvidence) TableName() string { return "condition_entry_evidence" }

type ConditionAnalysisScope struct {
	ConditionIDs []uuid.UUID            `json:"conditionIds"`
	EvidenceRefs []DiagnosisEvidenceRef `json:"evidenceRefs"`
	Description  string                 `json:"description"`
}
type ConditionAgentHypothesis struct {
	Statement    string                 `json:"statement"`
	EvidenceRefs []DiagnosisEvidenceRef `json:"evidenceRefs"`
}

// ConditionAnalysisRun is the immutable formal-analysis base, not a chat Run.
// Mutable state is reconstructed from slice 2 append-only events.
type ConditionAnalysisRun struct {
	UserID                   uuid.UUID `gorm:"primaryKey"`
	ID                       uuid.UUID `gorm:"primaryKey"`
	RequestMessageID         uuid.UUID
	RequestEvidenceID        uuid.UUID
	RequestSourceVersion     int64
	Trigger                  ConditionAnalysisTrigger
	Scope                    ConditionAnalysisScope `gorm:"serializer:json;type:jsonb"`
	BasedOnBodyStateRevision int64
	BasedOnDiagnosisRevision int64
	InputEvidenceRefs        []DiagnosisEvidenceRef     `gorm:"serializer:json;type:jsonb"`
	AgentHypotheses          []ConditionAgentHypothesis `gorm:"serializer:json;type:jsonb"`
	ReplayInput              datatypes.JSON
	ExecutionProvenance      datatypes.JSON
	IdempotencyKey           string
	RequestFingerprint       string
	StartedAt                time.Time
}

func (ConditionAnalysisRun) TableName() string { return "condition_analysis_runs" }

type DiagnosisProposal struct {
	UserID            uuid.UUID `gorm:"primaryKey"`
	ID                uuid.UUID `gorm:"primaryKey"`
	RunID             uuid.UUID
	TargetConditionID *uuid.UUID
	ChangeKind        DiagnosisChangeKind
	// Exact proposedEntry/changeSet/disposition/relation JSON is fingerprinted;
	// validation and command ingestion arrive with the slice 2 decision service.
	Content              datatypes.JSON
	ContentFingerprint   string
	BaseRevision         int64
	EvidenceRefs         []DiagnosisEvidenceRef `gorm:"serializer:json;type:jsonb"`
	CounterEvidenceRefs  []DiagnosisEvidenceRef `gorm:"serializer:json;type:jsonb"`
	SupersedesProposalID *uuid.UUID
	CreatedAt            time.Time
}

func (DiagnosisProposal) TableName() string { return "diagnosis_proposals" }

// DiagnosisDecision is also the durable decision-recording receipt. It never
// claims adoption; only DiagnosisRevision acknowledges an aggregate commit.
type DiagnosisDecision struct {
	UserID              uuid.UUID `gorm:"primaryKey"`
	ID                  uuid.UUID `gorm:"primaryKey"`
	ProposalID          uuid.UUID
	RunID               uuid.UUID
	ContentFingerprint  string
	DecidedEvidenceRefs []DiagnosisEvidenceRef `gorm:"serializer:json;type:jsonb"`
	Decision            DiagnosisDecisionKind
	ToolCallID          string
	DecidedAt           time.Time
	IdempotencyKey      string
	RequestFingerprint  string
	ExpectedRevision    int64
	ReceiptID           uuid.UUID
}

func (DiagnosisDecision) TableName() string { return "diagnosis_decisions" }

type DiagnosisRevision struct {
	UserID              uuid.UUID `gorm:"primaryKey"`
	ID                  uuid.UUID `gorm:"primaryKey"`
	PreviousRevision    int64
	Revision            int64
	RunID               *uuid.UUID
	DirectEditID        *uuid.UUID
	Patch               datatypes.JSON
	DecisionIDs         []uuid.UUID `gorm:"serializer:json;type:jsonb"`
	BodyStateRevision   int64
	EvidenceRefs        []DiagnosisEvidenceRef `gorm:"serializer:json;type:jsonb"`
	ReplayInput         datatypes.JSON
	ExecutionProvenance datatypes.JSON
	IdempotencyKey      string
	RequestFingerprint  string
	ReceiptID           uuid.UUID
	CommittedAt         time.Time
}

func (DiagnosisRevision) TableName() string { return "diagnosis_revisions" }
