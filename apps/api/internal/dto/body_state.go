package dto

import (
	"encoding/json"
	"time"
)

// BodyStateFactInput is the explicit workbench/API shape for a durable Fact.
// It intentionally does not expose durable identity fields such as user_id or
// revision; those are assigned/validated by the Go application layer.
type BodyStateFactInput struct {
	ConcernKey     string          `json:"concern_key"`
	Kind           string          `json:"kind" binding:"required"`
	BodyRegion     string          `json:"body_region"`
	Value          string          `json:"value"`
	Details        json.RawMessage `json:"details"`
	Origin         string          `json:"origin"`
	ReviewState    string          `json:"review_state"`
	LifecycleState string          `json:"lifecycle_state"`
	Trend          string          `json:"trend"`
	SourceKey      string          `json:"source_key"`
	Provenance     json.RawMessage `json:"provenance"`
	ObservedAt     *time.Time      `json:"observed_at"`
	ValidFrom      *time.Time      `json:"valid_from"`
	ValidUntil     *time.Time      `json:"valid_until"`
}

type UpsertBodyStateFactRequest struct {
	ExpectedRevision *int64             `json:"expected_revision"`
	Fact             BodyStateFactInput `json:"fact" binding:"required"`
}

type CorrectBodyStateFactRequest struct {
	ExpectedRevision *int64             `json:"expected_revision"`
	Replacement      BodyStateFactInput `json:"replacement" binding:"required"`
}

type UpdateBodyStateFactTemporalRequest struct {
	ExpectedRevision *int64     `json:"expected_revision"`
	LifecycleState   string     `json:"lifecycle_state"`
	Trend            string     `json:"trend"`
	ValidUntil       *time.Time `json:"valid_until"`
}

type ReviewBodyStateFactRequest struct {
	ExpectedRevision *int64 `json:"expected_revision"`
	ReviewState      string `json:"review_state" binding:"required"`
}

type BodyStateObservationInput struct {
	ExpectedRevision *int64          `json:"expected_revision"`
	ConcernKey       string          `json:"concern_key"`
	Kind             string          `json:"kind" binding:"required"`
	BodyRegion       string          `json:"body_region"`
	Method           string          `json:"method"`
	Value            json.RawMessage `json:"value" binding:"required"`
	Condition        json.RawMessage `json:"condition"`
	SourceKey        string          `json:"source_key"`
	Provenance       json.RawMessage `json:"provenance"`
	ObservedAt       *time.Time      `json:"observed_at"`
	LifecycleState   string          `json:"lifecycle_state"`
}

type ReviewBodyStateObservationRequest struct {
	ExpectedRevision *int64 `json:"expected_revision"`
	ReviewState      string `json:"review_state" binding:"required"`
}

// BodyStateHypothesisInput keeps AI/user explanations separate from Facts.
type BodyStateHypothesisInput struct {
	ExpectedRevision         *int64          `json:"expected_revision"`
	ConcernKey               string          `json:"concern_key"`
	Statement                string          `json:"statement" binding:"required"`
	LifecycleState           string          `json:"lifecycle_state"`
	Confidence               *string         `json:"confidence"`
	SupportingFactIDs        json.RawMessage `json:"supporting_fact_ids"`
	SupportingObservationIDs json.RawMessage `json:"supporting_observation_ids"`
	SupportingEvidenceIDs    json.RawMessage `json:"supporting_evidence_ids"`
	CounterevidenceIDs       json.RawMessage `json:"counterevidence_ids"`
	Provenance               json.RawMessage `json:"provenance"`
}

type UpdateBodyStateHypothesisLifecycleRequest struct {
	ExpectedRevision   *int64          `json:"expected_revision"`
	LifecycleState     string          `json:"lifecycle_state" binding:"required"`
	CounterevidenceIDs json.RawMessage `json:"counterevidence_ids"`
}

type ResolveBodyStateSafetyRequest struct {
	ExpectedRevision *int64 `json:"expected_revision"`
	Resolution       string `json:"resolution" binding:"required"`
	Note             string `json:"note"`
}
