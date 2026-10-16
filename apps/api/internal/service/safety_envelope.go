package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/bodysense/api/internal/domain/safety"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

const (
	SafetyEnvelopeSchemaV2   = "body-state-safety-envelope-v2"
	SafetyEnvelopePolicyV1   = "body-state-safety-policy-v2-native-restrictions"
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
)

type SafetyBlockerReasonV1 string

const (
	SafetyStructuredCurrentSignal SafetyBlockerReasonV1 = "structured_current_signal"
)

type SafetyBlockerV1 struct {
	RestrictionID uuid.UUID             `json:"restriction_id"`
	RiskLevel     string                `json:"risk_level"`
	Concept       SafetyConceptV1       `json:"concept"`
	SourceRef     string                `json:"source_ref"`
	SourceKind    SafetySourceKindV1    `json:"source_kind"`
	Reason        SafetyBlockerReasonV1 `json:"reason"`
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
	SchemaRevision    string              `json:"schema_revision"`
	PolicyRevision    string              `json:"policy_revision"`
	BodyStateRevision int64               `json:"body_state_revision"`
	Assertions        []SafetyAssertionV1 `json:"assertions"`
	ActiveBlockers    []SafetyBlockerV1   `json:"active_blockers"`
	RequiresReview    bool                `json:"requires_review"`
	Coverage          SafetyCoverageV1    `json:"coverage"`
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
// Coverage: each asserted positive V2 symptom statement is covered only by a
// complete safety screen bound to that fact, recorded at or after its creation
// revision. An absent symptom is not a complaint. No symptom means no coverage.
//
// Assertions: each safety screen contributes one confirmed current assertion
// per concept. Native restrictions and source-validated per-restriction
// resolutions alone determine current blockers; clearance never rewrites symptoms.
func ProjectSafetyEnvelopeV2(snapshot *BodyStateReadModel) (SafetyEnvelopeV2, error) {
	input := safetyProjectionInput{}
	if snapshot != nil {
		input = safetyProjectionInput{UserID: snapshot.UserID, CurrentRevision: snapshot.CurrentRevision, Restrictions: snapshot.SafetyRestrictions, Screens: snapshot.SafetyScreens}
		for _, fact := range snapshot.Facts {
			if fact.Kind != "symptom" || fact.AssertionStatus != "asserted" {
				continue
			}
			var details struct {
				Presence string `json:"presence"`
			}
			if len(fact.Details) > 0 && json.Unmarshal(fact.Details, &details) == nil && details.Presence == "absent" {
				continue
			}
			input.Symptoms = append(input.Symptoms, safetyCoverageSubject{ID: fact.ID, CreatedRevision: fact.CreatedRevision})
		}
	}
	return projectSafetyEnvelope(input)
}

// safetyCoverageSubject is one currently asserted symptom complaint that a
// structured safety screen must cover.
type safetyCoverageSubject struct {
	ID              uuid.UUID
	CreatedRevision int64
}

// safetyProjectionInput is everything the Go Safety envelope reads, pinned to
// one BodyState revision. Both the native read model and the V2 agent view
// project through it so they cannot disagree.
type safetyProjectionInput struct {
	UserID          uuid.UUID
	CurrentRevision int64
	Restrictions    []model.SafetyRestrictionRecord
	Symptoms        []safetyCoverageSubject
	Screens         []model.SafetyScreen
}

func projectSafetyEnvelope(snapshot safetyProjectionInput) (SafetyEnvelopeV2, error) {
	envelope := SafetyEnvelopeV2{SchemaRevision: SafetyEnvelopeSchemaV2, PolicyRevision: SafetyEnvelopePolicyV1, Assertions: []SafetyAssertionV1{}, ActiveBlockers: []SafetyBlockerV1{}}
	envelope.Coverage = SafetyCoverageV1{Revision: SafetyCoverageRevisionV1, CaptureRevision: SafetyCaptureRevisionV1, RequiredConcepts: []string{"trauma", "radiating_pain", "numbness", "weakness", "dizziness"}, CoveredSourceRefs: []string{}, IncompleteSourceRefs: []string{}}
	if snapshot.CurrentRevision <= 0 {
		return envelope, fmt.Errorf("safety projection requires a pinned BodyState revision")
	}
	envelope.BodyStateRevision = snapshot.CurrentRevision

	screens := make([]model.SafetyScreen, 0, len(snapshot.Screens))
	latestScreenRevision := map[uuid.UUID]int64{}
	for _, screen := range snapshot.Screens {
		if screen.UserID != snapshot.UserID {
			return envelope, fmt.Errorf("safety screen owner mismatch")
		}
		if screen.ID == uuid.Nil || screen.BodyStateRevision <= 0 || screen.BodyStateRevision > snapshot.CurrentRevision {
			return envelope, fmt.Errorf("invalid safety screen revision")
		}
		screens = append(screens, screen)
		if screen.BodyStateRevision > latestScreenRevision[screen.BodyFactID] {
			latestScreenRevision[screen.BodyFactID] = screen.BodyStateRevision
		}
	}
	sort.Slice(screens, func(i, j int) bool { return screens[i].ID.String() < screens[j].ID.String() })

	symptoms := append([]safetyCoverageSubject(nil), snapshot.Symptoms...)
	sort.Slice(symptoms, func(i, j int) bool { return symptoms[i].ID.String() < symptoms[j].ID.String() })
	for _, symptom := range symptoms {
		ref := "body-state:fact:" + symptom.ID.String()
		if symptom.CreatedRevision > 0 && latestScreenRevision[symptom.ID] >= symptom.CreatedRevision {
			envelope.Coverage.CoveredSourceRefs = append(envelope.Coverage.CoveredSourceRefs, ref)
		} else {
			envelope.Coverage.IncompleteSourceRefs = append(envelope.Coverage.IncompleteSourceRefs, ref)
		}
	}

	positive := map[string]bool{}
	for _, screen := range screens {
		ref := "body-state:safety-screen:" + screen.ID.String()
		recordedAt := screen.RecordedAt
		values := map[SafetyConceptV1]bool{SafetyTrauma: screen.Trauma, SafetyRadiatingPain: screen.RadiatingPain,
			SafetyNumbness: screen.Numbness, SafetyWeakness: screen.Weakness, SafetyDizziness: screen.Dizziness}
		for _, field := range structuredSafetyDetails {
			polarity := SafetyAbsent
			if values[field.concept] {
				polarity = SafetyPresent
				positive[screen.ID.String()+":"+string(field.concept)] = true
			}
			envelope.Assertions = append(envelope.Assertions, SafetyAssertionV1{Concept: field.concept, Polarity: polarity, Temporality: SafetyCurrent, ReviewState: SafetyConfirmed, SourceRef: ref, SourceKind: SafetyStructuredScreen, ObservedAt: &recordedAt})
		}
	}
	restrictions := append([]model.SafetyRestrictionRecord(nil), snapshot.Restrictions...)
	sort.Slice(restrictions, func(i, j int) bool { return restrictions[i].ID.String() < restrictions[j].ID.String() })
	for _, row := range restrictions {
		key := row.SafetyScreenID.String() + ":" + row.Concept
		if row.UserID != snapshot.UserID || row.CreatedRevision > snapshot.CurrentRevision || !positive[key] || row.PolicyRevision != model.SafetyScreenRestrictionPolicyV1 || !safety.ValidateRestriction(gateRestriction(row)) {
			return envelope, fmt.Errorf("invalid native safety restriction")
		}
		if row.Active != (row.Resolution == nil || !row.ClearanceAvailable) {
			return envelope, fmt.Errorf("inconsistent native safety resolution")
		}
		if row.Resolution != nil && (row.Resolution.UserID != snapshot.UserID || row.Resolution.RestrictionID != row.ID || row.Resolution.Revision <= row.CreatedRevision || row.Resolution.Revision > snapshot.CurrentRevision) {
			return envelope, fmt.Errorf("invalid native safety resolution")
		}
		delete(positive, key)
		if row.Active {
			envelope.ActiveBlockers = append(envelope.ActiveBlockers, SafetyBlockerV1{RestrictionID: row.ID, RiskLevel: row.RiskLevel, Concept: SafetyConceptV1(row.Concept), SourceRef: "body-state:safety-screen:" + row.SafetyScreenID.String(), SourceKind: SafetyStructuredScreen, Reason: SafetyStructuredCurrentSignal})
		}
	}
	if len(positive) > 0 {
		return envelope, fmt.Errorf("positive safety screen lacks its native restriction")
	}

	envelope.Coverage.Complete = len(envelope.Coverage.CoveredSourceRefs) > 0 && len(envelope.Coverage.IncompleteSourceRefs) == 0
	envelope.RequiresReview = len(envelope.ActiveBlockers) > 0
	return envelope, nil
}
