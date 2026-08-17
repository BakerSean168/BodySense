package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

const (
	TreatmentStatusActive            = "active"
	TreatmentStatusReviewRecommended = "review_recommended"
	TreatmentStatusPaused            = "paused"
	TreatmentStatusSuperseded        = "superseded"
	TreatmentStatusCompleted         = "completed"

	TreatmentAcceptanceProposed = "proposed"
	TreatmentAcceptanceAccepted = "accepted"
	TreatmentAcceptanceRejected = "rejected"
)

// Treatment is the one current user-scoped intervention strategy aggregate.
// Accepted revisions are immutable; this row only points at the current revision
// and carries mutable lifecycle/review state.
type Treatment struct {
	ID                        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID                    uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex" json:"user_id"`
	CurrentRevision           int            `gorm:"not null;default:0" json:"current_revision"`
	Status                    string         `gorm:"type:varchar(30);not null" json:"status"`
	SourceBodyStateRevision   *int64         `json:"source_body_state_revision,omitempty"`
	SourceDiagnosisAnalysisID *uuid.UUID     `gorm:"type:uuid" json:"source_diagnosis_analysis_id,omitempty"`
	StatusReasons             datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"status_reasons"`
	CreatedAt                 time.Time      `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt                 time.Time      `gorm:"not null;default:now()" json:"updated_at"`

	Current *TreatmentRevision `gorm:"-" json:"current,omitempty"`
}

func (Treatment) TableName() string { return "treatments" }

// TreatmentRevision is an immutable proposed or accepted plan pinned to exact
// DiagnosisAnalysis and BodyState identities.
type TreatmentRevision struct {
	ID                        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	TreatmentID               uuid.UUID      `gorm:"type:uuid;not null;index" json:"treatment_id"`
	Revision                  int            `gorm:"not null" json:"revision"`
	AcceptanceState           string         `gorm:"type:varchar(20);not null" json:"acceptance_state"`
	LifecycleState            string         `gorm:"type:varchar(30);not null" json:"lifecycle_state"`
	SourceBodyStateRevision   int64          `gorm:"not null" json:"source_body_state_revision"`
	SourceDiagnosisAnalysisID uuid.UUID      `gorm:"type:uuid;not null;index" json:"source_diagnosis_analysis_id"`
	Goal                      string         `gorm:"type:text;not null" json:"goal"`
	DurationWeeks             int            `gorm:"not null" json:"duration_weeks"`
	Plan                      datatypes.JSON `gorm:"type:jsonb;not null" json:"plan"`
	UserConstraints           datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"user_constraints"`
	EvidenceIDs               datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"evidence_ids"`
	Governance                datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"governance"`
	ChangeReason              string         `gorm:"type:text;not null;default:''" json:"change_reason"`
	CreatedAt                 time.Time      `gorm:"not null;default:now()" json:"created_at"`
	AcceptedAt                *time.Time     `json:"accepted_at,omitempty"`

	Interventions []Intervention `gorm:"-" json:"interventions"`
}

func (TreatmentRevision) TableName() string { return "treatment_revisions" }

// TreatmentPlanContent is the validated business representation stored in the
// revision's JSONB plan. The schema stays typed at the service/API boundary.
type TreatmentPlanContent struct {
	Summary          string                       `json:"summary"`
	Goal             string                       `json:"goal"`
	DurationWeeks    int                          `json:"duration_weeks"`
	Interventions    []TreatmentInterventionDraft `json:"interventions"`
	DailyHabits      []string                     `json:"daily_habits"`
	ExpectedTimeline string                       `json:"expected_timeline"`
	WarningSigns     []string                     `json:"warning_signs"`
	ReviewTriggers   []string                     `json:"review_triggers"`
	SafetyNotes      []string                     `json:"safety_notes"`
}

// TreatmentInterventionDraft is an AI recommendation before acceptance creates
// durable Intervention rows.
type TreatmentInterventionDraft struct {
	Kind         string         `json:"kind"`
	Title        string         `json:"title"`
	Description  string         `json:"description"`
	Prescription map[string]any `json:"prescription"`
}
