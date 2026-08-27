package dto

import (
	"time"

	"github.com/google/uuid"
)

// InjuryHistorySnapshot is a user-facing projection of the active BodyState
// injury summary. It is not a separately persisted record.
type InjuryHistorySnapshot struct {
	CurrentRevision int64      `json:"current_revision"`
	FactID          *uuid.UUID `json:"fact_id,omitempty"`
	Summary         string     `json:"summary"`
	ValidFrom       *time.Time `json:"valid_from,omitempty"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty"`
}

type UpdateInjuryHistoryRequest struct {
	ExpectedRevision *int64 `json:"expected_revision,omitempty"`
	Summary          string `json:"summary"`
}
