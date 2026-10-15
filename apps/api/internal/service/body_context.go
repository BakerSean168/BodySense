package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// User-reported current context (body metrics, injury history and the six
// lifestyle sections) is stored only in the V2 BodyFact/BodyObservation
// ledger. Every edit appends a new user statement; the newest statement per
// key decides the current value, and an explicit clear withdraws it. Older
// statements stay as history and are never overwritten.
const (
	bodyContextObservationHeight = "height"
	bodyContextObservationWeight = "weight"
	bodyContextHistoryInjury     = "injury_summary"
	bodyContextMaxPages          = 50
)

var bodyContextLifestyleSections = []string{"activity", "sleep", "exercise", "nutrition", "substances", "recovery"}

var bodyContextLegacyLifestyleKind = map[string]string{
	"activity":   model.BodyStateFactKindLifestyleActivity,
	"sleep":      model.BodyStateFactKindLifestyleSleep,
	"exercise":   model.BodyStateFactKindLifestyleExercise,
	"nutrition":  model.BodyStateFactKindLifestyleNutrition,
	"substances": model.BodyStateFactKindLifestyleSubstances,
	"recovery":   model.BodyStateFactKindLifestyleRecovery,
}

type bodyContextLedger interface {
	ListFacts(ctx context.Context, userID uuid.UUID, beforeRevision int64) (*model.BodyFactPage, error)
	RecordDirectFacts(ctx context.Context, userID uuid.UUID, command model.RecordBodyFactsCommand) (*model.BodyFactCommit, error)
	WithdrawDirectFact(ctx context.Context, owner, target uuid.UUID, command model.AmendBodyFactCommand) (*model.BodyFactCommit, error)
	ListDirectObservations(ctx context.Context, owner uuid.UUID, beforeRevision int64) (*model.BodyObservationPage, error)
	RecordDirectObservation(ctx context.Context, owner uuid.UUID, command model.RecordBodyObservationCommand) (*model.BodyObservationCommit, error)
	ListSafetyScreens(ctx context.Context, owner uuid.UUID) ([]model.SafetyScreen, error)
}

// BodyContext is the current projection of the user's reported context.
type BodyContext struct {
	Revision  int64
	Height    *model.BodyObservationRecord
	Weight    *model.BodyObservationRecord
	Injury    *model.BodyFact
	Lifestyle map[string]*model.BodyFact
	// Symptoms are all currently asserted V2 symptom statements, newest first.
	Symptoms []model.BodyFact
	// SafetyScreens are the owner's structured safety screens, newest first.
	SafetyScreens []model.SafetyScreen
}

// BodyContextUpdate is one semantic edit. Nil fields are untouched; an empty
// string clears the current statement (withdrawal, not erasure).
type BodyContextUpdate struct {
	HeightCm  *float64
	WeightKg  *float64
	Injury    *string
	Lifestyle map[string]*LifestyleSectionInput
	Context   string // measurement context label, e.g. onboarding
}

func loadBodyContext(ctx context.Context, ledger bodyContextLedger, owner uuid.UUID) (*BodyContext, error) {
	out := &BodyContext{Lifestyle: map[string]*model.BodyFact{}}
	seen := map[string]bool{}
	before := int64(0)
	for page := 0; page < bodyContextMaxPages; page++ {
		facts, err := ledger.ListFacts(ctx, owner, before)
		if err != nil {
			return nil, fmt.Errorf("list body facts: %w", err)
		}
		if page == 0 {
			out.Revision = facts.CurrentRevision
		}
		for index := range facts.Facts {
			fact := facts.Facts[index]
			if fact.Kind == "symptom" && fact.AssertionStatus == "asserted" {
				out.Symptoms = append(out.Symptoms, fact)
				continue
			}
			key := bodyContextFactKey(fact)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			if fact.AssertionStatus != "asserted" {
				continue
			}
			if key == "history:"+bodyContextHistoryInjury {
				out.Injury = &fact
			} else {
				out.Lifestyle[strings.TrimPrefix(key, "lifestyle:")] = &fact
			}
		}
		if !facts.HasMore || facts.NextBeforeRevision == nil {
			break
		}
		before = *facts.NextBeforeRevision
	}
	before = 0
	for page := 0; page < bodyContextMaxPages && (out.Height == nil || out.Weight == nil); page++ {
		observations, err := ledger.ListDirectObservations(ctx, owner, before)
		if err != nil {
			return nil, fmt.Errorf("list body observations: %w", err)
		}
		if observations.CurrentRevision > out.Revision {
			out.Revision = observations.CurrentRevision
		}
		for index := range observations.Observations {
			observation := observations.Observations[index]
			if seen["obs:"+observation.Kind] {
				continue
			}
			switch observation.Kind {
			case bodyContextObservationHeight, bodyContextObservationWeight:
			default:
				continue
			}
			seen["obs:"+observation.Kind] = true
			if observation.AssertionStatus != "asserted" {
				continue
			}
			if observation.Kind == bodyContextObservationHeight {
				out.Height = &observation
			} else {
				out.Weight = &observation
			}
		}
		if !observations.HasMore || observations.NextBeforeRevision == nil {
			break
		}
		before = *observations.NextBeforeRevision
	}
	screens, err := ledger.ListSafetyScreens(ctx, owner)
	if err != nil {
		return nil, fmt.Errorf("list safety screens: %w", err)
	}
	out.SafetyScreens = screens
	return out, nil
}

func bodyContextFactKey(fact model.BodyFact) string {
	switch fact.Kind {
	case "health_history":
		var details model.HistoryFactDetails
		if json.Unmarshal(fact.Details, &details) == nil && details.HistoryKind == bodyContextHistoryInjury {
			return "history:" + bodyContextHistoryInjury
		}
	case "lifestyle":
		var details model.LifestyleFactDetails
		if json.Unmarshal(fact.Details, &details) == nil {
			if _, ok := bodyContextLegacyLifestyleKind[details.HabitType]; ok {
				return "lifestyle:" + details.HabitType
			}
		}
	}
	return ""
}

// applyBodyContextUpdate commits the edit as at most one fact batch, one
// observation per changed metric and one withdrawal per cleared statement.
// expectedRevision guards the first command; later commands chain on the
// revision returned by the previous commit.
func applyBodyContextUpdate(ctx context.Context, ledger bodyContextLedger, owner uuid.UUID, expectedRevision *int64, update BodyContextUpdate, now time.Time) (int64, error) {
	current, err := loadBodyContext(ctx, ledger, owner)
	if err != nil {
		return 0, err
	}
	revision := current.Revision
	if expectedRevision != nil {
		revision = *expectedRevision
	}
	var inputs []model.BodyFactInput
	var withdrawals []uuid.UUID
	today := now.UTC().Format("2006-01-02")
	if update.Injury != nil {
		summary := strings.TrimSpace(*update.Injury)
		switch {
		case summary == "" && current.Injury != nil:
			withdrawals = append(withdrawals, current.Injury.ID)
		case summary != "" && (current.Injury == nil || current.Injury.Statement != summary):
			details, _ := json.Marshal(model.HistoryFactDetails{HistoryKind: bodyContextHistoryInjury})
			inputs = append(inputs, model.BodyFactInput{
				Kind: "health_history", Statement: summary, BodyRegionIDs: []string{},
				Temporal: model.FactTemporal{Precision: "unknown"}, Details: details,
			})
		}
	}
	for _, section := range bodyContextLifestyleSections {
		input, ok := update.Lifestyle[section]
		if !ok || input == nil {
			continue
		}
		existing := current.Lifestyle[section]
		summary := strings.TrimSpace(input.Summary)
		details := bodyContextLifestyleDetails(section, input.Details)
		switch {
		case summary == "" && existing != nil:
			withdrawals = append(withdrawals, existing.ID)
		case summary != "" && (existing == nil || existing.Statement != summary || !bodyContextJSONEqual(existing.Details, details)):
			inputs = append(inputs, model.BodyFactInput{
				Kind: "lifestyle", Statement: summary, BodyRegionIDs: []string{},
				Temporal: model.FactTemporal{Start: today, Precision: "day"}, Details: details,
			})
		}
	}
	if len(inputs) > 0 {
		commit, err := ledger.RecordDirectFacts(ctx, owner, model.RecordBodyFactsCommand{
			IdempotencyKey: uuid.NewString(), ExpectedRevision: revision, Facts: inputs,
		})
		if err != nil {
			return 0, err
		}
		revision = commit.Revision
	}
	for _, target := range withdrawals {
		commit, err := ledger.WithdrawDirectFact(ctx, owner, target, model.AmendBodyFactCommand{
			IdempotencyKey: uuid.NewString(), ExpectedRevision: revision,
		})
		if err != nil {
			return 0, err
		}
		revision = commit.Revision
	}
	metrics := []struct {
		value   *float64
		kind    string
		unit    string
		label   string
		current *model.BodyObservationRecord
	}{
		{update.HeightCm, bodyContextObservationHeight, "cm", "身高", current.Height},
		{update.WeightKg, bodyContextObservationWeight, "kg", "体重", current.Weight},
	}
	for _, metric := range metrics {
		if metric.value == nil {
			continue
		}
		raw := strconv.FormatFloat(*metric.value, 'f', -1, 64)
		if metric.current != nil && metric.current.RawValue == raw && metric.current.RawUnit == metric.unit {
			continue
		}
		commit, err := ledger.RecordDirectObservation(ctx, owner, model.RecordBodyObservationCommand{
			IdempotencyKey: uuid.NewString(), ExpectedRevision: revision,
			Observation: model.RecordBodyObservationInput{
				Kind: metric.kind, Statement: metric.label + " " + raw + " " + metric.unit,
				RawValue: raw, RawUnit: metric.unit, MeasurementContext: update.Context,
				ObservedAt: model.FactTemporal{Start: now.UTC().Format(time.RFC3339), Precision: "exact"},
			},
		})
		if err != nil {
			return 0, err
		}
		revision = commit.Revision
	}
	return revision, nil
}

func bodyContextLifestyleDetails(section string, raw json.RawMessage) json.RawMessage {
	details := model.LifestyleFactDetails{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &details)
	}
	details.HabitType = section
	encoded, _ := json.Marshal(details)
	return encoded
}

func bodyContextJSONEqual(left datatypes.JSON, right json.RawMessage) bool {
	var a, b any
	if json.Unmarshal(left, &a) != nil || json.Unmarshal(right, &b) != nil {
		return false
	}
	ea, _ := json.Marshal(a)
	eb, _ := json.Marshal(b)
	return string(ea) == string(eb)
}

func bodyContextMetric(record *model.BodyObservationRecord) *BodyMetricValue {
	if record == nil {
		return nil
	}
	value, err := strconv.ParseFloat(record.RawValue, 64)
	if err != nil || value <= 0 {
		return nil
	}
	result := &BodyMetricValue{Value: value, Unit: record.RawUnit}
	if observed, err := time.Parse(time.RFC3339, record.ObservedAt.Start); err == nil {
		result.ObservedAt = &observed
	}
	return result
}

func bodyContextFactTemporalStart(fact *model.BodyFact) *time.Time {
	var temporal model.FactTemporal
	if json.Unmarshal(fact.Temporal, &temporal) != nil || temporal.Start == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if parsed, err := time.Parse(layout, temporal.Start); err == nil {
			return &parsed
		}
	}
	return nil
}

// projectBodyContextIntoSnapshot replaces the retired legacy rows for these
// context kinds with the ledger projection, so snapshot readers (assessment,
// diagnosis, treatment, workspace) see exactly one current value per key.
func projectBodyContextIntoSnapshot(snapshot *BodyStateSnapshot, current *BodyContext) {
	legacyFactKinds := map[string]bool{model.BodyStateFactKindInjuryHistory: true}
	for _, kind := range bodyContextLegacyLifestyleKind {
		legacyFactKinds[kind] = true
	}
	facts := snapshot.Facts[:0]
	for _, fact := range snapshot.Facts {
		if !legacyFactKinds[fact.Kind] {
			facts = append(facts, fact)
		}
	}
	observations := snapshot.Observations[:0]
	for _, observation := range snapshot.Observations {
		if observation.Kind != model.BodyStateObservationKindHeight && observation.Kind != model.BodyStateObservationKindWeight {
			observations = append(observations, observation)
		}
	}
	appendFact := func(kind, concern string, fact *model.BodyFact) {
		provenance, _ := json.Marshal(map[string]any{"source_type": "body_fact_ledger", "body_fact_id": fact.ID})
		facts = append(facts, model.BodyStateFact{
			ID: fact.ID, UserID: fact.UserID, ConcernKey: concern, Kind: kind, Value: fact.Statement,
			Details: fact.Details, Origin: "user_reported", ReviewState: "confirmed", LifecycleState: "active",
			Trend: "unknown", Provenance: datatypes.JSON(provenance), ValidFrom: bodyContextFactTemporalStart(fact),
			CreatedRevision: fact.CreatedRevision, UpdatedRevision: fact.CreatedRevision,
			CreatedAt: fact.RecordedAt, UpdatedAt: fact.RecordedAt,
		})
	}
	if current.Injury != nil {
		appendFact(model.BodyStateFactKindInjuryHistory, "history:injury", current.Injury)
	}
	for _, section := range bodyContextLifestyleSections {
		if fact := current.Lifestyle[section]; fact != nil {
			appendFact(bodyContextLegacyLifestyleKind[section], "lifestyle:"+section, fact)
		}
	}
	// V2 symptom statements reach legacy snapshot readers (assessment,
	// diagnosis, treatment, workspace) as discomfort facts keyed by their V2
	// fact id. A user-reported absence is not a complaint and is not projected.
	for index := range current.Symptoms {
		fact := current.Symptoms[index]
		var details model.SymptomFactDetails
		if json.Unmarshal(fact.Details, &details) != nil || details.Presence == "absent" {
			continue
		}
		var regions []string
		_ = json.Unmarshal(fact.BodyRegionIDs, &regions)
		concern, region := "general", ""
		var regionID *string
		if len(regions) > 0 {
			region = regions[0]
			regionID = &regions[0]
			concern = "region:" + region
		}
		provenance, _ := json.Marshal(map[string]any{"source_type": "body_fact_ledger", "body_fact_id": fact.ID})
		facts = append(facts, model.BodyStateFact{
			ID: fact.ID, UserID: fact.UserID, ConcernKey: concern, Kind: "discomfort", BodyRegion: region, BodyRegionID: regionID,
			Value: fact.Statement, Details: fact.Details, Origin: "user_reported", ReviewState: "confirmed", LifecycleState: "active",
			Trend: "unknown", Provenance: datatypes.JSON(provenance), ValidFrom: bodyContextFactTemporalStart(&fact),
			CreatedRevision: fact.CreatedRevision, UpdatedRevision: fact.CreatedRevision,
			CreatedAt: fact.RecordedAt, UpdatedAt: fact.RecordedAt,
		})
	}
	snapshot.SafetyScreens = current.SafetyScreens
	if snapshot.SafetyScreens == nil {
		snapshot.SafetyScreens = []model.SafetyScreen{}
	}
	appendObservation := func(kind string, record *model.BodyObservationRecord) {
		metric := bodyContextMetric(record)
		if metric == nil {
			return
		}
		value, _ := json.Marshal(map[string]any{"value": metric.Value, "unit": metric.Unit})
		provenance, _ := json.Marshal(map[string]any{"source_type": "body_observation_ledger", "body_observation_id": record.ID})
		observations = append(observations, model.BodyStateObservation{
			ID: record.ID, UserID: record.UserID, Kind: kind, Method: "self_report",
			Value: datatypes.JSON(value), Condition: datatypes.JSON(`{}`), Provenance: datatypes.JSON(provenance),
			ObservedAt: metric.ObservedAt, ReviewState: "confirmed", LifecycleState: "active",
			CreatedRevision: record.CreatedRevision, UpdatedRevision: record.CreatedRevision,
			CreatedAt: record.RecordedAt, UpdatedAt: record.RecordedAt,
		})
	}
	appendObservation(model.BodyStateObservationKindHeight, current.Height)
	appendObservation(model.BodyStateObservationKindWeight, current.Weight)
	snapshot.Facts = facts
	snapshot.Observations = observations
	if current.Revision > snapshot.CurrentRevision {
		snapshot.CurrentRevision = current.Revision
	}
}
