package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// LifestyleSection is a read projection of one active BodyState fact. It is not
// a separately persisted aggregate.
type LifestyleSection struct {
	Kind        string          `json:"kind"`
	FactID      *uuid.UUID      `json:"fact_id,omitempty"`
	Summary     string          `json:"summary"`
	Details     json.RawMessage `json:"details"`
	ValidFrom   *time.Time      `json:"valid_from,omitempty"`
	UpdatedAt   *time.Time      `json:"updated_at,omitempty"`
	ReviewState string          `json:"review_state,omitempty"`
}

type LifestyleCandidate struct {
	FactID    uuid.UUID       `json:"fact_id"`
	Kind      string          `json:"kind"`
	Summary   string          `json:"summary"`
	Details   json.RawMessage `json:"details"`
	CreatedAt time.Time       `json:"created_at"`
}

type LifestyleSnapshot struct {
	CurrentRevision int64                `json:"current_revision"`
	Activity        LifestyleSection     `json:"activity"`
	Sleep           LifestyleSection     `json:"sleep"`
	Exercise        LifestyleSection     `json:"exercise"`
	Nutrition       LifestyleSection     `json:"nutrition"`
	Substances      LifestyleSection     `json:"substances"`
	Recovery        LifestyleSection     `json:"recovery"`
	PendingUpdates  []LifestyleCandidate `json:"pending_updates"`
}

type LifestyleSectionInput struct {
	Summary string          `json:"summary"`
	Details json.RawMessage `json:"details,omitempty"`
}

// UpdateLifestyleRequest is patch-like: omitted sections are untouched; a
// present section with an empty summary explicitly clears the current section.
type UpdateLifestyleRequest struct {
	ExpectedRevision *int64                 `json:"expected_revision,omitempty"`
	Activity         *LifestyleSectionInput `json:"activity,omitempty"`
	Sleep            *LifestyleSectionInput `json:"sleep,omitempty"`
	Exercise         *LifestyleSectionInput `json:"exercise,omitempty"`
	Nutrition        *LifestyleSectionInput `json:"nutrition,omitempty"`
	Substances       *LifestyleSectionInput `json:"substances,omitempty"`
	Recovery         *LifestyleSectionInput `json:"recovery,omitempty"`
}

type ReviewLifestyleCandidateRequest struct {
	ExpectedRevision *int64 `json:"expected_revision,omitempty"`
}
