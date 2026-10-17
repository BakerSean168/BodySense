package model

import (
	"encoding/json"
	"time"

	"github.com/bodysense/api/internal/domain/trainingplan"
	"github.com/google/uuid"
)

// TrainingPlanDraft is the complete materialized candidate, never an accepted
// prescription. Update candidates are pinned to one base revision.
type TrainingPlanDraft struct {
	Title               string              `json:"title"`
	Goal                string              `json:"goal"`
	RelatedConditionIDs []uuid.UUID         `json:"relatedConditionIds"`
	RelatedBodyRefs     []PlanBodyRef       `json:"relatedBodyRefs"`
	Content             TrainingPlanContent `json:"content"`
}
type PlanBodyRef struct {
	Kind          string    `json:"kind"`
	ID            uuid.UUID `json:"id"`
	SourceVersion int64     `json:"sourceVersion"`
}
type TrainingPlanContent struct {
	Sessions    []PlanSessionTemplate `json:"sessions"`
	Frequency   PlanFrequency         `json:"frequency"`
	Schedule    PlanSchedule          `json:"schedule"`
	Constraints []string              `json:"constraints"`
}
type PlanFrequency struct {
	SessionsPerWeek *int   `json:"sessionsPerWeek,omitempty"`
	Description     string `json:"description,omitempty"`
}
type PlanSchedule struct {
	Mode        string           `json:"mode"`
	TimeZone    string           `json:"timeZone,omitempty"`
	Occurrences []PlanOccurrence `json:"occurrences,omitempty"`
}
type PlanOccurrence struct {
	OccurrenceID string `json:"occurrenceId"`
	TemplateID   string `json:"templateId"`
	LocalDate    string `json:"localDate"`
}
type PlanSessionTemplate struct {
	TemplateID string      `json:"templateId"`
	Title      string      `json:"title"`
	Stages     []PlanStage `json:"stages"`
}
type PlanStage struct {
	Phase     string         `json:"phase"`
	Exercises []PlanExercise `json:"exercises"`
}
type PlanExercise struct {
	PlanExerciseID  string     `json:"planExerciseId"`
	ExerciseID      string     `json:"exerciseId"`
	NameSnapshot    string     `json:"nameSnapshot"`
	Sets            int        `json:"sets"`
	Target          PlanTarget `json:"target"`
	RestSeconds     *float64   `json:"restSeconds,omitempty"`
	Side            string     `json:"side,omitempty"`
	LoadOrIntensity string     `json:"loadOrIntensity,omitempty"`
	Instructions    string     `json:"instructions,omitempty"`
	DemoAssetID     string     `json:"demoAssetId,omitempty"`
}
type PlanTarget struct {
	Kind    string   `json:"kind"`
	Min     float64  `json:"min,omitempty"`
	Max     *float64 `json:"max,omitempty"`
	Seconds float64  `json:"seconds,omitempty"`
	Meters  float64  `json:"meters,omitempty"`
}

type PlanProposal struct {
	ID                         uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID                     uuid.UUID `gorm:"type:uuid"`
	Kind                       trainingplan.ProposalKind
	State                      trainingplan.ProposalState
	InitiatedBy                trainingplan.Initiator
	SourceID                   string
	PlanID                     *uuid.UUID      `gorm:"type:uuid"`
	BasedOnRevisionID          *uuid.UUID      `gorm:"type:uuid"`
	Draft                      json.RawMessage `gorm:"type:jsonb"`
	IdempotencyKey             string
	RequestFingerprint         string
	ContentFingerprint         string
	RelevantContextFingerprint string
	CreatedAt                  time.Time
	ExpiresAt                  time.Time
	DecisionID                 *uuid.UUID `gorm:"type:uuid"`
	ExpiredReason              *string
}

func (PlanProposal) TableName() string { return "plan_proposals" }

type TrainingPlanRevision struct {
	ID               uuid.UUID       `gorm:"type:uuid;primaryKey"`
	UserID           uuid.UUID       `gorm:"type:uuid"`
	PlanID           uuid.UUID       `gorm:"type:uuid"`
	ParentRevisionID *uuid.UUID      `gorm:"type:uuid"`
	Snapshot         json.RawMessage `gorm:"type:jsonb"`
	ChangeSet        json.RawMessage `gorm:"type:jsonb"`
	SourceKind       string
	SourceID         string
	AuthorizationID  uuid.UUID `gorm:"type:uuid"`
	CreatedAt        time.Time
}

func (TrainingPlanRevision) TableName() string { return "training_plan_revisions" }

type PlanDecisionReceipt struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID         uuid.UUID `gorm:"type:uuid"`
	ProposalID     uuid.UUID `gorm:"type:uuid"`
	DecisionID     uuid.UUID `gorm:"type:uuid"`
	Operation      string
	IdempotencyKey string
	Fingerprint    string
	PlanID         *uuid.UUID `gorm:"type:uuid"`
	RevisionID     *uuid.UUID `gorm:"type:uuid"`
	CreatedAt      time.Time
}

func (PlanDecisionReceipt) TableName() string { return "plan_decision_receipts" }
