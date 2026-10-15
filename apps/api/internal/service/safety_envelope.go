package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

const (
	SafetyEnvelopeSchemaV2   = "body-state-safety-envelope-v2"
	SafetyEnvelopePolicyV1   = "body-state-safety-policy-v1"
	SafetyCoverageRevisionV1 = "body-state-safety-coverage-v1"
	SafetyCaptureRevisionV1  = "body-state-safety-capture-v1"
)

type SafetyConceptV1 string

const (
	SafetyTrauma          SafetyConceptV1 = "trauma"
	SafetyRadiatingPain   SafetyConceptV1 = "radiating_pain"
	SafetyNumbness        SafetyConceptV1 = "numbness"
	SafetyWeakness        SafetyConceptV1 = "weakness"
	SafetyDizziness       SafetyConceptV1 = "dizziness"
	SafetyGaitInstability SafetyConceptV1 = "gait_instability"
	SafetySeverePain      SafetyConceptV1 = "severe_pain"
	SafetyWorsening       SafetyConceptV1 = "worsening"
	SafetyInfection       SafetyConceptV1 = "infection"
	SafetySystemic        SafetyConceptV1 = "systemic"
	SafetyUnknownRedFlag  SafetyConceptV1 = "unknown_red_flag"
)

type SafetyPolarityV1 string

const (
	SafetyPresent   SafetyPolarityV1 = "present"
	SafetyAbsent    SafetyPolarityV1 = "absent"
	SafetyUncertain SafetyPolarityV1 = "uncertain"
)

type SafetyTemporalityV1 string

const (
	SafetyCurrent    SafetyTemporalityV1 = "current"
	SafetyHistorical SafetyTemporalityV1 = "historical"
	SafetyResolved   SafetyTemporalityV1 = "resolved"
)

type SafetyReviewStateV1 string

const (
	SafetyConfirmed  SafetyReviewStateV1 = "confirmed"
	SafetyUnverified SafetyReviewStateV1 = "unverified"
)

type SafetySourceKindV1 string

const (
	SafetyStructuredScreen SafetySourceKindV1 = "safety_screen"
	SafetyLegacyStateFact  SafetySourceKindV1 = "legacy_safety_state"
)

type SafetyBlockerReasonV1 string

const (
	SafetyStructuredCurrentSignal SafetyBlockerReasonV1 = "structured_current_signal"
	SafetyLegacyActiveState       SafetyBlockerReasonV1 = "legacy_active_state"
)

type SafetyBlockerV1 struct {
	Concept    SafetyConceptV1       `json:"concept"`
	SourceRef  string                `json:"source_ref"`
	SourceKind SafetySourceKindV1    `json:"source_kind"`
	Reason     SafetyBlockerReasonV1 `json:"reason"`
}

type SafetyAssertionV1 struct {
	Concept     SafetyConceptV1            `json:"concept"`
	Polarity    SafetyPolarityV1           `json:"polarity"`
	Temporality SafetyTemporalityV1        `json:"temporality"`
	ReviewState SafetyReviewStateV1        `json:"review_state"`
	SourceRef   string                     `json:"source_ref"`
	SourceKind  SafetySourceKindV1         `json:"source_kind"`
	ObservedAt  *time.Time                 `json:"observed_at,omitempty"`
	Provenance  map[string]json.RawMessage `json:"provenance,omitempty"`
}

type SafetyEnvelopeV2 struct {
	SchemaRevision     string              `json:"schema_revision"`
	PolicyRevision     string              `json:"policy_revision"`
	BodyStateRevision  int64               `json:"body_state_revision"`
	Assertions         []SafetyAssertionV1 `json:"assertions"`
	ActiveBlockers     []SafetyBlockerV1   `json:"active_blockers"`
	RequiresReview     bool                `json:"requires_review"`
	LegacyStatePresent bool                `json:"legacy_state_present"`
	Coverage           SafetyCoverageV1    `json:"coverage"`
}

type SafetyCoverageV1 struct {
	Revision             string   `json:"revision"`
	CaptureRevision      string   `json:"capture_revision"`
	RequiredConcepts     []string `json:"required_concepts"`
	CoveredSourceRefs    []string `json:"covered_source_refs"`
	IncompleteSourceRefs []string `json:"incomplete_source_refs"`
	Complete             bool     `json:"complete"`
}

var structuredSafetyDetails = []struct {
	key     string
	concept SafetyConceptV1
}{
	{"trauma", SafetyTrauma}, {"radiating_pain", SafetyRadiatingPain},
	{"numbness", SafetyNumbness}, {"weakness", SafetyWeakness}, {"dizziness", SafetyDizziness},
}

// ProjectSafetyEnvelopeV2 is a pure projection over one pinned BodyState snapshot.
//
// Coverage: every active asserted symptom (V2 symptom statements projected as
// discomfort facts; presence=absent is not a complaint) is covered iff a
// complete safety screen was recorded at a revision >= the symptom's created
// revision. No symptom means no coverage (fail-closed).
//
// Assertions: each safety screen contributes one confirmed current assertion
// per concept. A positive concept is an active blocker until an explicit Go
// safety resolution (ResolveSafetyState) is recorded after that screen.
func ProjectSafetyEnvelopeV2(snapshot *BodyStateSnapshot) (SafetyEnvelopeV2, error) {
	envelope := SafetyEnvelopeV2{SchemaRevision: SafetyEnvelopeSchemaV2, PolicyRevision: SafetyEnvelopePolicyV1, Assertions: []SafetyAssertionV1{}, ActiveBlockers: []SafetyBlockerV1{}}
	envelope.Coverage = SafetyCoverageV1{Revision: SafetyCoverageRevisionV1, CaptureRevision: SafetyCaptureRevisionV1, RequiredConcepts: []string{"trauma", "radiating_pain", "numbness", "weakness", "dizziness"}, CoveredSourceRefs: []string{}, IncompleteSourceRefs: []string{}}
	if snapshot == nil || snapshot.CurrentRevision <= 0 {
		return envelope, fmt.Errorf("safety projection requires a pinned BodyState revision")
	}
	envelope.BodyStateRevision = snapshot.CurrentRevision
	legacyAssertions, legacyActive, legacyPresent, err := adaptLegacySafetyState(snapshot.SafetyState)
	if err != nil {
		return envelope, err
	}
	envelope.LegacyStatePresent = legacyPresent
	envelope.Assertions = append(envelope.Assertions, legacyAssertions...)
	if legacyActive {
		for _, assertion := range legacyAssertions {
			envelope.ActiveBlockers = append(envelope.ActiveBlockers, SafetyBlockerV1{Concept: assertion.Concept, SourceRef: assertion.SourceRef, SourceKind: assertion.SourceKind, Reason: SafetyLegacyActiveState})
		}
	}
	resolvedAt := legacySafetyResolvedAt(snapshot.SafetyState)

	screens := make([]model.SafetyScreen, 0, len(snapshot.SafetyScreens))
	latestScreenRevision := int64(0)
	for _, screen := range snapshot.SafetyScreens {
		if screen.ID == uuid.Nil || screen.UserID != snapshot.UserID || screen.BodyStateRevision <= 0 || screen.BodyStateRevision > snapshot.CurrentRevision {
			continue
		}
		screens = append(screens, screen)
		if screen.BodyStateRevision > latestScreenRevision {
			latestScreenRevision = screen.BodyStateRevision
		}
	}
	sort.Slice(screens, func(i, j int) bool { return screens[i].ID.String() < screens[j].ID.String() })

	facts := append([]model.BodyStateFact(nil), snapshot.Facts...)
	sort.Slice(facts, func(i, j int) bool { return facts[i].ID.String() < facts[j].ID.String() })
	for _, fact := range facts {
		if fact.Kind != "discomfort" || fact.LifecycleState != "active" || fact.ExcludedFromReasoning || (fact.ReviewState != "confirmed" && fact.ReviewState != "unverified") {
			continue
		}
		var details struct {
			Presence string `json:"presence"`
		}
		if len(fact.Details) > 0 && json.Unmarshal(fact.Details, &details) == nil && details.Presence == "absent" {
			continue
		}
		ref := "body-state:fact:" + fact.ID.String()
		if fact.CreatedRevision > 0 && latestScreenRevision >= fact.CreatedRevision {
			envelope.Coverage.CoveredSourceRefs = append(envelope.Coverage.CoveredSourceRefs, ref)
		} else {
			envelope.Coverage.IncompleteSourceRefs = append(envelope.Coverage.IncompleteSourceRefs, ref)
		}
	}

	for _, screen := range screens {
		ref := "body-state:safety-screen:" + screen.ID.String()
		recordedAt := screen.RecordedAt
		values := map[SafetyConceptV1]bool{
			SafetyTrauma: screen.Trauma, SafetyRadiatingPain: screen.RadiatingPain, SafetyNumbness: screen.Numbness,
			SafetyWeakness: screen.Weakness, SafetyDizziness: screen.Dizziness,
		}
		for _, field := range structuredSafetyDetails {
			polarity, temporality := SafetyAbsent, SafetyCurrent
			if values[field.concept] {
				polarity = SafetyPresent
				if resolvedAt != nil && resolvedAt.After(recordedAt) {
					temporality = SafetyResolved
				}
			}
			envelope.Assertions = append(envelope.Assertions, SafetyAssertionV1{Concept: field.concept, Polarity: polarity, Temporality: temporality, ReviewState: SafetyConfirmed, SourceRef: ref, SourceKind: SafetyStructuredScreen, ObservedAt: &recordedAt})
			if polarity == SafetyPresent && temporality == SafetyCurrent {
				envelope.ActiveBlockers = append(envelope.ActiveBlockers, SafetyBlockerV1{Concept: field.concept, SourceRef: ref, SourceKind: SafetyStructuredScreen, Reason: SafetyStructuredCurrentSignal})
			}
		}
	}
	envelope.Coverage.Complete = len(envelope.Coverage.CoveredSourceRefs) > 0 && len(envelope.Coverage.IncompleteSourceRefs) == 0
	envelope.RequiresReview = len(envelope.ActiveBlockers) > 0
	return envelope, nil
}

// legacySafetyResolvedAt returns when the current safety state was explicitly
// resolved through ResolveSafetyState, or nil. Detector output never resolves.
func legacySafetyResolvedAt(raw json.RawMessage) *time.Time {
	var state struct {
		Status     string     `json:"status"`
		ResolvedAt *time.Time `json:"resolved_at"`
	}
	if len(bytes.TrimSpace(raw)) == 0 || json.Unmarshal(raw, &state) != nil || state.ResolvedAt == nil {
		return nil
	}
	switch strings.TrimSpace(state.Status) {
	case "resolved", "cleared_by_review", "monitoring":
		return state.ResolvedAt
	default:
		return nil
	}
}

func adaptLegacySafetyState(raw json.RawMessage) ([]SafetyAssertionV1, bool, bool, error) {
	if len(bytes.TrimSpace(raw)) == 0 || string(bytes.TrimSpace(raw)) == "{}" {
		return nil, false, false, nil
	}
	var state map[string]json.RawMessage
	if err := json.Unmarshal(raw, &state); err != nil || state == nil {
		return nil, false, false, fmt.Errorf("invalid legacy safety state")
	}
	if len(state) == 0 {
		return nil, false, false, nil
	}
	var hasRedFlags bool
	if err := json.Unmarshal(state["has_red_flags"], &hasRedFlags); err != nil {
		return nil, false, true, fmt.Errorf("legacy safety state requires boolean has_red_flags")
	}
	status := ""
	if statusRaw, ok := state["status"]; ok {
		if err := json.Unmarshal(statusRaw, &status); err != nil {
			return nil, false, true, fmt.Errorf("invalid legacy safety status")
		}
		status = strings.TrimSpace(status)
	}
	switch status {
	case "", "requires_review", "active", "monitoring", "resolved", "cleared_by_review":
	default:
		return nil, false, true, fmt.Errorf("unknown legacy safety status %q", status)
	}
	if hasRedFlags && (status == "" || status == "resolved" || status == "cleared_by_review") || !hasRedFlags && (status == "requires_review" || status == "active" || status == "monitoring") {
		return nil, false, true, fmt.Errorf("inconsistent legacy safety state")
	}
	active := hasRedFlags && (status == "requires_review" || status == "active")
	if !hasRedFlags {
		return nil, false, true, nil
	}
	concepts := []SafetyConceptV1{}
	if flagRaw, ok := state["flags"]; ok {
		var flags []map[string]json.RawMessage
		if err := json.Unmarshal(flagRaw, &flags); err != nil {
			return nil, false, true, fmt.Errorf("invalid legacy safety flags")
		}
		for _, flag := range flags {
			var category string
			categoryRaw := flag["category"]
			if len(categoryRaw) == 0 {
				categoryRaw = flag["type"]
			}
			if len(categoryRaw) > 0 && json.Unmarshal(categoryRaw, &category) != nil {
				return nil, false, true, fmt.Errorf("invalid legacy safety flag category")
			}
			concepts = append(concepts, legacySafetyConcept(category))
		}
	}
	if len(concepts) == 0 {
		concepts = append(concepts, SafetyUnknownRedFlag)
	}
	assertions := make([]SafetyAssertionV1, 0, len(concepts))
	for _, concept := range concepts {
		assertions = append(assertions, SafetyAssertionV1{Concept: concept, Polarity: SafetyPresent, Temporality: SafetyCurrent, ReviewState: SafetyUnverified, SourceRef: "body-state:legacy-safety-state", SourceKind: SafetyLegacyStateFact})
	}
	return assertions, active, true, nil
}

func legacySafetyConcept(category string) SafetyConceptV1 {
	switch category {
	case "trauma", "radiating_pain", "numbness", "weakness", "dizziness", "gait_instability", "severe_pain", "worsening", "infection", "systemic":
		return SafetyConceptV1(category)
	default:
		return SafetyUnknownRedFlag
	}
}
