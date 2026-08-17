package dto

import (
	"encoding/json"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// HealthWorkspace is the product projection of the continuous health loop. It
// intentionally exposes capabilities rather than one terminal linear stage.
type HealthWorkspace struct {
	GeneratedAt        time.Time                   `json:"generated_at"`
	ConversationID     *uuid.UUID                  `json:"conversation_id,omitempty"`
	ProfileReady       bool                        `json:"profile_ready"`
	BodyState          *HealthWorkspaceBodyState   `json:"body_state"`
	Diagnosis          map[string]any              `json:"diagnosis,omitempty"`
	Treatment          *model.Treatment            `json:"treatment,omitempty"`
	TrainingPlan       *model.TrainingPlan         `json:"training_plan,omitempty"`
	TreatmentRevisions []model.TreatmentRevision   `json:"treatment_revisions"`
	RecentOutcomes     []model.Outcome             `json:"recent_outcomes"`
	Trends             []HealthWorkspaceTrend      `json:"trends"`
	Capabilities       HealthWorkspaceCapabilities `json:"capabilities"`
	Actions            []HealthWorkspaceAction     `json:"actions"`
}

type HealthWorkspaceBodyState struct {
	CurrentRevision     int64                        `json:"current_revision"`
	SafetyState         json.RawMessage              `json:"safety_state"`
	Facts               []model.BodyStateFact        `json:"facts"`
	Observations        []model.BodyStateObservation `json:"observations"`
	PendingObservations []model.BodyStateObservation `json:"pending_observations"`
	Hypotheses          []model.BodyStateHypothesis  `json:"hypotheses"`
	RecentRevisions     []model.BodyStateRevision    `json:"recent_revisions"`
}

type HealthWorkspaceCapabilities struct {
	CanContinueConsultation bool `json:"can_continue_consultation"`
	CanEditBodyState        bool `json:"can_edit_body_state"`
	CanRequestDiagnosis     bool `json:"can_request_diagnosis"`
	CanReviewDiagnosis      bool `json:"can_review_diagnosis"`
	CanGenerateTreatment    bool `json:"can_generate_treatment"`
	CanAcceptTreatment      bool `json:"can_accept_treatment"`
	CanExecuteTreatment     bool `json:"can_execute_treatment"`
	CanRecordOutcome        bool `json:"can_record_outcome"`
	CanReviewTreatment      bool `json:"can_review_treatment"`
	RequiresSafetyReview    bool `json:"requires_safety_review"`
	RequiresDiagnosisReview bool `json:"requires_diagnosis_review"`
	RequiresTreatmentReview bool `json:"requires_treatment_review"`
}

type HealthWorkspaceAction struct {
	Kind     string         `json:"kind"`
	Priority int            `json:"priority"`
	Enabled  bool           `json:"enabled"`
	Reason   string         `json:"reason"`
	Target   map[string]any `json:"target,omitempty"`
}

type HealthWorkspaceTrend struct {
	Key          string                      `json:"key"`
	ConcernKey   string                      `json:"concern_key"`
	BodyRegion   string                      `json:"body_region"`
	Kind         string                      `json:"kind"`
	CurrentTrend string                      `json:"current_trend"`
	Points       []HealthWorkspaceTrendPoint `json:"points"`
}

type HealthWorkspaceTrendPoint struct {
	OccurredAt     time.Time       `json:"occurred_at"`
	SourceType     string          `json:"source_type"`
	Value          json.RawMessage `json:"value"`
	Notes          string          `json:"notes,omitempty"`
	CausalityLevel string          `json:"causality_level,omitempty"`
}
