package service

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
	Diagnosis          map[string]any              `json:"diagnosis,omitempty"`
	TrainingPlan       *model.TrainingPlan         `json:"training_plan,omitempty"`
	RecentOutcomes     []model.Outcome             `json:"recent_outcomes"`
	Trends             []HealthWorkspaceTrend      `json:"trends"`
	Capabilities       HealthWorkspaceCapabilities `json:"capabilities"`
	Actions            []HealthWorkspaceAction     `json:"actions"`
}

type HealthWorkspaceCapabilities struct {
	CanContinueConsultation bool `json:"can_continue_consultation"`
	CanEditBodyState        bool `json:"can_edit_body_state"`
	CanRequestDiagnosis     bool `json:"can_request_diagnosis"`
	CanReviewDiagnosis      bool `json:"can_review_diagnosis"`
	CanGenerateTreatment    bool `json:"can_generate_treatment"`
	CanExecuteTreatment     bool `json:"can_execute_treatment"`
	CanRecordOutcome        bool `json:"can_record_outcome"`
	RequiresSafetyReview    bool `json:"requires_safety_review"`
	RequiresDiagnosisReview bool `json:"requires_diagnosis_review"`
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
