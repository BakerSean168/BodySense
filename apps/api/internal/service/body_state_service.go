package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// bodyStateRepository is deliberately consumer-owned: the application service
// declares only the persistence capabilities required by BodyState workflows.
type bodyStateRepository interface {
	bodyContextLedger
	GetCurrent(ctx context.Context, userID uuid.UUID) (*model.BodyState, error)
	ListRecentRevisions(ctx context.Context, userID uuid.UUID, limit int) ([]model.BodyStateRevision, error)
	ListReviewableObservations(ctx context.Context, userID uuid.UUID, limit int) ([]model.BodyStateObservation, error)
	ListReviewableFacts(ctx context.Context, userID uuid.UUID, limit int) ([]model.BodyStateFact, error)
	ListRevisionsAfter(ctx context.Context, userID uuid.UUID, afterRevision int64, limit int) ([]model.BodyStateRevision, error)
	UpsertFact(ctx context.Context, userID uuid.UUID, expectedRevision *int64, fact model.BodyStateFact, source string) (*model.BodyStateFact, *model.BodyStateRevision, error)
	CorrectFact(ctx context.Context, userID uuid.UUID, expectedRevision *int64, targetFactID uuid.UUID, replacement model.BodyStateFact, source string) (*model.BodyStateFact, *model.BodyStateRevision, error)
	TransitionFact(ctx context.Context, userID uuid.UUID, expectedRevision *int64, targetFactID uuid.UUID, replacement model.BodyStateFact, effectiveAt time.Time, source string) (*model.BodyStateFact, *model.BodyStateRevision, error)
	AcceptCurrentFactCandidate(ctx context.Context, userID uuid.UUID, expectedRevision *int64, candidateID uuid.UUID, effectiveAt time.Time, source string) (*model.BodyStateFact, *model.BodyStateRevision, error)
	UpdateFactTemporal(ctx context.Context, userID uuid.UUID, expectedRevision *int64, factID uuid.UUID, lifecycleState, trend string, validUntil *time.Time, source string) (*model.BodyStateFact, *model.BodyStateRevision, error)
	UpdateFactReviewState(ctx context.Context, userID uuid.UUID, expectedRevision *int64, factID uuid.UUID, reviewState, source string) (*model.BodyStateFact, *model.BodyStateRevision, error)
	UpsertObservation(ctx context.Context, userID uuid.UUID, expectedRevision *int64, observation model.BodyStateObservation, source string) (*model.BodyStateObservation, *model.BodyStateRevision, error)
	TransitionObservation(ctx context.Context, userID uuid.UUID, expectedRevision *int64, targetObservationID uuid.UUID, replacement model.BodyStateObservation, source string) (*model.BodyStateObservation, *model.BodyStateRevision, error)
	ApplyCurrentContextPatch(ctx context.Context, userID uuid.UUID, expectedRevision *int64, patch model.BodyStateCurrentContextPatch, source string) (*model.BodyStateRevision, error)
	UpdateObservationReviewState(ctx context.Context, userID uuid.UUID, expectedRevision *int64, observationID uuid.UUID, reviewState, source string) (*model.BodyStateObservation, *model.BodyStateRevision, error)
	SetSafetyState(ctx context.Context, userID uuid.UUID, expectedRevision *int64, safetyState datatypes.JSON, source string) (*model.BodyStateRevision, error)
	UpsertEvidence(ctx context.Context, userID uuid.UUID, evidence model.BodyStateEvidence) (*model.BodyStateEvidence, error)
	ListEvidence(ctx context.Context, userID uuid.UUID, limit int) ([]model.BodyStateEvidence, error)
	GetEvidenceByIDs(ctx context.Context, userID uuid.UUID, ids []uuid.UUID) ([]model.BodyStateEvidence, error)
	AddHypothesis(ctx context.Context, userID uuid.UUID, expectedRevision *int64, hypothesis model.BodyStateHypothesis, source string) (*model.BodyStateHypothesis, *model.BodyStateRevision, error)
	UpdateHypothesisLifecycle(ctx context.Context, userID uuid.UUID, expectedRevision *int64, hypothesisID uuid.UUID, lifecycleState string, counterevidenceIDs datatypes.JSON, source string) (*model.BodyStateHypothesis, *model.BodyStateRevision, error)
}

// BodyStateSnapshot is the stable business context shared with Consultation and
// Diagnosis. Revisions are newest-first and bounded so context remains finite.
type BodyStateSnapshot struct {
	UserID          uuid.UUID                    `json:"user_id"`
	CurrentRevision int64                        `json:"current_revision"`
	SafetyState     json.RawMessage              `json:"safety_state"`
	Facts           []model.BodyStateFact        `json:"facts"`
	Observations    []model.BodyStateObservation `json:"observations"`
	Hypotheses      []model.BodyStateHypothesis  `json:"hypotheses"`
	RecentRevisions []model.BodyStateRevision    `json:"recent_revisions,omitempty"`
	// SafetyScreens feed the Go Safety envelope only; they are not sent to
	// agents or browsers as raw rows.
	SafetyScreens []model.SafetyScreen `json:"-"`
}

// BodyStateService is the application boundary around the user-scoped aggregate.
// Producers (chat, workbench, posture analysis) call this service; they never own
// BodyState persistence themselves.
type BodyStateService struct {
	repo                  bodyStateRepository
	bodyRegionIDValidator BodyRegionIDValidator
}

func NewBodyStateService(repo bodyStateRepository) *BodyStateService {
	return &BodyStateService{repo: repo}
}

// WithBodyRegionIDValidator wires the canonical ontology authority without
// moving ontology ownership into the durable lane. Anatomically localized writes
// fail closed unless they resolve to a canonical region identity.
func (s *BodyStateService) WithBodyRegionIDValidator(validator BodyRegionIDValidator) *BodyStateService {
	s.bodyRegionIDValidator = validator
	return s
}

func (s *BodyStateService) GetSnapshot(ctx context.Context, userID uuid.UUID, historyLimit int) (*BodyStateSnapshot, error) {
	state, err := s.repo.GetCurrent(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get current body state: %w", err)
	}
	revisions, err := s.repo.ListRecentRevisions(ctx, userID, historyLimit)
	if err != nil {
		return nil, fmt.Errorf("list body state revisions: %w", err)
	}
	safety := json.RawMessage(state.SafetyState)
	if len(safety) == 0 {
		safety = json.RawMessage(`{}`)
	}
	facts := state.Facts
	if facts == nil {
		facts = []model.BodyStateFact{}
	}
	observations := state.Observations
	if observations == nil {
		observations = []model.BodyStateObservation{}
	}
	hypotheses := state.Hypotheses
	if hypotheses == nil {
		hypotheses = []model.BodyStateHypothesis{}
	}
	if revisions == nil {
		revisions = []model.BodyStateRevision{}
	}
	snapshot := &BodyStateSnapshot{
		UserID:          userID,
		CurrentRevision: state.CurrentRevision,
		SafetyState:     safety,
		Facts:           facts,
		Observations:    observations,
		Hypotheses:      hypotheses,
		RecentRevisions: revisions,
	}
	current, err := loadBodyContext(ctx, s.repo, userID)
	if err != nil {
		return nil, err
	}
	projectBodyContextIntoSnapshot(snapshot, current)
	return snapshot, nil
}

// CurrentBodyContext returns the user's reported metrics, injury history and
// lifestyle from the V2 ledger.
func (s *BodyStateService) CurrentBodyContext(ctx context.Context, userID uuid.UUID) (*BodyContext, error) {
	return loadBodyContext(ctx, s.repo, userID)
}

// UpdateBodyContext appends the user's edited context statements to the V2
// ledger and returns the resulting BodyState revision.
func (s *BodyStateService) UpdateBodyContext(ctx context.Context, userID uuid.UUID, expectedRevision *int64, update BodyContextUpdate) (int64, error) {
	return applyBodyContextUpdate(ctx, s.repo, userID, expectedRevision, update, time.Now())
}

func (s *BodyStateService) ListReviewableObservations(
	ctx context.Context,
	userID uuid.UUID,
	limit int,
) ([]model.BodyStateObservation, error) {
	items, err := s.repo.ListReviewableObservations(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.BodyStateObservation{}
	}
	return items, nil
}

// ApplyCurrentContextPatch is the application boundary for a semantically
// coherent multi-field current-context save. The repository commits it under
// one aggregate lock and at most one BodyStateRevision.
func (s *BodyStateService) ApplyCurrentContextPatch(
	ctx context.Context,
	userID uuid.UUID,
	expectedRevision *int64,
	patch model.BodyStateCurrentContextPatch,
	source string,
) (*model.BodyStateRevision, error) {
	seenFacts := map[string]struct{}{}
	for index := range patch.Facts {
		kind := strings.TrimSpace(patch.Facts[index].Kind)
		if kind == "" {
			return nil, errors.New("body state fact kind is required")
		}
		if _, exists := seenFacts[kind]; exists {
			return nil, fmt.Errorf("duplicate current fact mutation for kind %q", kind)
		}
		seenFacts[kind] = struct{}{}
		patch.Facts[index].Kind = kind
		if patch.Facts[index].Replacement != nil {
			patch.Facts[index].Replacement.Kind = kind
			patch.Facts[index].Replacement.Value = strings.TrimSpace(patch.Facts[index].Replacement.Value)
			regionID, err := s.normalizeBodyRegionID(
				patch.Facts[index].Replacement.BodyRegionID,
				patch.Facts[index].Replacement.BodyRegion,
			)
			if err != nil {
				return nil, err
			}
			patch.Facts[index].Replacement.BodyRegionID = regionID
		}
	}
	seenObservations := map[string]struct{}{}
	for index := range patch.Observations {
		kind := strings.TrimSpace(patch.Observations[index].Kind)
		if kind == "" {
			return nil, errors.New("body state observation kind is required")
		}
		if _, exists := seenObservations[kind]; exists {
			return nil, fmt.Errorf("duplicate current observation mutation for kind %q", kind)
		}
		seenObservations[kind] = struct{}{}
		patch.Observations[index].Kind = kind
		if patch.Observations[index].Replacement != nil {
			patch.Observations[index].Replacement.Kind = kind
			regionID, err := s.normalizeBodyRegionID(
				patch.Observations[index].Replacement.BodyRegionID,
				patch.Observations[index].Replacement.BodyRegion,
			)
			if err != nil {
				return nil, err
			}
			patch.Observations[index].Replacement.BodyRegionID = regionID
		}
	}
	return s.repo.ApplyCurrentContextPatch(ctx, userID, expectedRevision, patch, source)
}

func (s *BodyStateService) ListReviewableFacts(ctx context.Context, userID uuid.UUID, limit int) ([]model.BodyStateFact, error) {
	items, err := s.repo.ListReviewableFacts(ctx, userID, limit)
	if items == nil {
		items = []model.BodyStateFact{}
	}
	return items, err
}

func (s *BodyStateService) AcceptCurrentFactCandidate(
	ctx context.Context,
	userID uuid.UUID,
	expectedRevision *int64,
	candidateID uuid.UUID,
	effectiveAt time.Time,
) (*model.BodyStateFact, *model.BodyStateRevision, error) {
	return s.repo.AcceptCurrentFactCandidate(
		ctx, userID, expectedRevision, candidateID, effectiveAt, "user_review",
	)
}

// SetCurrentFact is the single-item convenience wrapper. Real later changes are
// temporal transitions; corrections continue to use CorrectFact explicitly.
func (s *BodyStateService) SetCurrentFact(
	ctx context.Context,
	userID uuid.UUID,
	expectedRevision *int64,
	kind string,
	replacement *model.BodyStateFact,
	effectiveAt time.Time,
	source string,
) (*model.BodyStateFact, *model.BodyStateRevision, error) {
	revision, err := s.ApplyCurrentContextPatch(ctx, userID, expectedRevision, model.BodyStateCurrentContextPatch{
		Facts: []model.BodyStateCurrentFactMutation{{
			Kind: kind, Replacement: replacement, EffectiveAt: effectiveAt,
		}},
	}, source)
	if err != nil {
		return nil, nil, err
	}
	if replacement == nil || strings.TrimSpace(replacement.Value) == "" {
		return nil, revision, nil
	}
	state, err := s.repo.GetCurrent(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	for index := range state.Facts {
		if state.Facts[index].Kind == strings.TrimSpace(kind) && state.Facts[index].ReviewState == "confirmed" {
			fact := state.Facts[index]
			return &fact, revision, nil
		}
	}
	return nil, revision, nil
}

// SetCurrentObservation is the single-item convenience wrapper for singleton
// current measurements/observations.
func (s *BodyStateService) SetCurrentObservation(
	ctx context.Context,
	userID uuid.UUID,
	expectedRevision *int64,
	kind string,
	replacement model.BodyStateObservation,
	source string,
) (*model.BodyStateObservation, *model.BodyStateRevision, error) {
	revision, err := s.ApplyCurrentContextPatch(ctx, userID, expectedRevision, model.BodyStateCurrentContextPatch{
		Observations: []model.BodyStateCurrentObservationMutation{{
			Kind: kind, Replacement: &replacement,
		}},
	}, source)
	if err != nil {
		return nil, nil, err
	}
	state, err := s.repo.GetCurrent(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	for index := range state.Observations {
		if state.Observations[index].Kind == strings.TrimSpace(kind) {
			observation := state.Observations[index]
			return &observation, revision, nil
		}
	}
	return nil, revision, nil
}

func (s *BodyStateService) UpsertFact(ctx context.Context, userID uuid.UUID, expectedRevision *int64, fact model.BodyStateFact) (*model.BodyStateFact, *model.BodyStateRevision, error) {
	regionID, err := s.normalizeBodyRegionID(fact.BodyRegionID, fact.BodyRegion)
	if err != nil {
		return nil, nil, err
	}
	fact.BodyRegionID = regionID
	return s.repo.UpsertFact(ctx, userID, expectedRevision, fact, "user_edit")
}

func (s *BodyStateService) CorrectFact(ctx context.Context, userID uuid.UUID, expectedRevision *int64, factID uuid.UUID, replacement model.BodyStateFact) (*model.BodyStateFact, *model.BodyStateRevision, error) {
	regionID, err := s.normalizeBodyRegionID(replacement.BodyRegionID, replacement.BodyRegion)
	if err != nil {
		return nil, nil, err
	}
	replacement.BodyRegionID = regionID
	return s.repo.CorrectFact(ctx, userID, expectedRevision, factID, replacement, "user_edit")
}

func (s *BodyStateService) UpdateFactTemporal(ctx context.Context, userID uuid.UUID, expectedRevision *int64, factID uuid.UUID, lifecycleState, trend string, validUntil *time.Time) (*model.BodyStateFact, *model.BodyStateRevision, error) {
	return s.repo.UpdateFactTemporal(ctx, userID, expectedRevision, factID, lifecycleState, trend, validUntil, "user_edit")
}

func (s *BodyStateService) ReviewFact(ctx context.Context, userID uuid.UUID, expectedRevision *int64, factID uuid.UUID, reviewState string) (*model.BodyStateFact, *model.BodyStateRevision, error) {
	return s.repo.UpdateFactReviewState(ctx, userID, expectedRevision, factID, reviewState, "user_review")
}

func (s *BodyStateService) AddObservation(ctx context.Context, userID uuid.UUID, expectedRevision *int64, observation model.BodyStateObservation) (*model.BodyStateObservation, *model.BodyStateRevision, error) {
	regionID, err := s.normalizeBodyRegionID(observation.BodyRegionID, observation.BodyRegion)
	if err != nil {
		return nil, nil, err
	}
	observation.BodyRegionID = regionID
	observation.ReviewState = "confirmed"
	observation.ExcludedFromReasoning = false
	return s.repo.UpsertObservation(ctx, userID, expectedRevision, observation, "user_edit")
}

func (s *BodyStateService) AddAssessmentObservation(
	ctx context.Context,
	userID uuid.UUID,
	observation model.BodyStateObservation,
) (*model.BodyStateObservation, *model.BodyStateRevision, error) {
	regionID, err := s.normalizeBodyRegionID(observation.BodyRegionID, observation.BodyRegion)
	if err != nil {
		return nil, nil, err
	}
	observation.BodyRegionID = regionID
	observation.ReviewState = "unverified"
	observation.ExcludedFromReasoning = true
	return s.repo.UpsertObservation(ctx, userID, nil, observation, "assessment")
}

func (s *BodyStateService) ReviewObservation(
	ctx context.Context,
	userID uuid.UUID,
	expectedRevision *int64,
	observationID uuid.UUID,
	reviewState string,
) (*model.BodyStateObservation, *model.BodyStateRevision, error) {
	return s.repo.UpdateObservationReviewState(
		ctx, userID, expectedRevision, observationID, reviewState, "user_review",
	)
}

// ListRevisionsAfter exposes bounded semantic changes for deterministic freshness
// and treatment-review policies.
func (s *BodyStateService) ListRevisionsAfter(ctx context.Context, userID uuid.UUID, afterRevision int64, limit int) ([]model.BodyStateRevision, error) {
	return s.repo.ListRevisionsAfter(ctx, userID, afterRevision, limit)
}

func (s *BodyStateService) UpsertEvidence(ctx context.Context, userID uuid.UUID, evidence model.BodyStateEvidence) (*model.BodyStateEvidence, error) {
	return s.repo.UpsertEvidence(ctx, userID, evidence)
}

func (s *BodyStateService) ListEvidence(ctx context.Context, userID uuid.UUID, limit int) ([]model.BodyStateEvidence, error) {
	return s.repo.ListEvidence(ctx, userID, limit)
}

func (s *BodyStateService) GetEvidenceByIDs(ctx context.Context, userID uuid.UUID, ids []uuid.UUID) ([]model.BodyStateEvidence, error) {
	return s.repo.GetEvidenceByIDs(ctx, userID, ids)
}

func (s *BodyStateService) AddHypothesis(ctx context.Context, userID uuid.UUID, expectedRevision *int64, hypothesis model.BodyStateHypothesis) (*model.BodyStateHypothesis, *model.BodyStateRevision, error) {
	return s.repo.AddHypothesis(ctx, userID, expectedRevision, hypothesis, "user_review")
}

func (s *BodyStateService) AddDiagnosisHypothesis(ctx context.Context, userID uuid.UUID, hypothesis model.BodyStateHypothesis) (*model.BodyStateHypothesis, *model.BodyStateRevision, error) {
	return s.repo.AddHypothesis(ctx, userID, nil, hypothesis, "diagnosis")
}

func (s *BodyStateService) UpdateHypothesisLifecycle(ctx context.Context, userID uuid.UUID, expectedRevision *int64, hypothesisID uuid.UUID, lifecycleState string, counterevidenceIDs json.RawMessage) (*model.BodyStateHypothesis, *model.BodyStateRevision, error) {
	return s.repo.UpdateHypothesisLifecycle(ctx, userID, expectedRevision, hypothesisID, lifecycleState, datatypes.JSON(counterevidenceIDs), "user_review")
}

// RecordOutcome turns accepted intervention feedback into an explicit BodyState
// mutation. Subjective symptom feedback becomes a Fact; measurements/adherence
// become Observations. The Outcome remains a separate durable record either way.
func (s *BodyStateService) RecordOutcome(ctx context.Context, userID uuid.UUID, outcome model.Outcome) (*model.BodyStateRevision, error) {
	var value map[string]any
	_ = json.Unmarshal(outcome.Value, &value)
	provenance := datatypes.JSON(bodyStateMustJSON(map[string]any{
		"source_type": "outcome",
		"outcome_id":  outcome.ID,
		"source_key":  outcome.SourceKey,
		"occurred_at": outcome.OccurredAt,
		"causality":   outcome.CausalityLevel,
		"association": outcome.AssociationStatement,
	}))

	if outcome.Kind == "symptom_change" || outcome.Kind == "new_discomfort" {
		if factID, err := uuid.Parse(bodyStateString(value["fact_id"])); err == nil {
			lifecycle := bodyStateString(value["lifecycle_state"])
			trend := bodyStateString(value["trend"])
			fact, revision, updateErr := s.repo.UpdateFactTemporal(ctx, userID, nil, factID, lifecycle, trend, nil, "outcome")
			if updateErr != nil || revision != nil {
				return revision, updateErr
			}
			if fact != nil && fact.UpdatedRevision > 0 {
				return &model.BodyStateRevision{Revision: fact.UpdatedRevision}, nil
			}
			return nil, errors.New("existing fact projection has no revision")
		}
		durableRegion, durableRegionID := bodyStateCanonicalRegionProjection(outcome.BodyRegion)
		details := map[string]any{
			"outcome_kind":         outcome.Kind,
			"notes":                outcome.Notes,
			"value":                value,
			"reported_body_region": strings.TrimSpace(outcome.BodyRegion),
		}
		description := bodyStateString(value["description"])
		if description == "" {
			description = bodyStateString(value["symptom"])
		}
		if description == "" {
			description = strings.TrimSpace(outcome.Notes)
		}
		if description == "" {
			description = "干预后的症状变化"
		}
		trend := bodyStateDefault(bodyStateString(value["trend"]), "unknown")
		fact, revision, err := s.repo.UpsertFact(ctx, userID, nil, model.BodyStateFact{
			ConcernKey:     bodyStateDefault(outcome.ConcernKey, bodyStateConcernKey(durableRegion)),
			Kind:           "discomfort",
			BodyRegion:     durableRegion,
			BodyRegionID:   durableRegionID,
			Value:          description,
			Details:        datatypes.JSON(bodyStateMustJSON(details)),
			Origin:         "user_reported",
			ReviewState:    "confirmed",
			LifecycleState: bodyStateDefault(bodyStateString(value["lifecycle_state"]), "active"),
			Trend:          trend,
			SourceKey:      "outcome:" + outcome.ID.String(),
			Provenance:     provenance,
			ObservedAt:     &outcome.OccurredAt,
		}, "outcome")
		if err != nil || revision != nil {
			return revision, err
		}
		if fact != nil && fact.UpdatedRevision > 0 {
			return &model.BodyStateRevision{Revision: fact.UpdatedRevision}, nil
		}
		return nil, errors.New("existing fact projection has no revision")
	}

	observationKind := outcome.Kind
	if observationKind == "" {
		observationKind = "intervention_outcome"
	}
	durableRegion, durableRegionID := bodyStateCanonicalRegionProjection(outcome.BodyRegion)
	observation, revision, err := s.repo.UpsertObservation(ctx, userID, nil, model.BodyStateObservation{
		ConcernKey:            bodyStateDefault(outcome.ConcernKey, bodyStateConcernKey(durableRegion)),
		Kind:                  observationKind,
		BodyRegion:            durableRegion,
		BodyRegionID:          durableRegionID,
		Method:                outcome.SourceType,
		Value:                 outcome.Value,
		Condition:             datatypes.JSON(bodyStateMustJSON(map[string]any{"notes": outcome.Notes})),
		SourceKey:             "outcome:" + outcome.ID.String(),
		Provenance:            provenance,
		ObservedAt:            &outcome.OccurredAt,
		ReviewState:           "confirmed",
		LifecycleState:        "active",
		ExcludedFromReasoning: false,
	}, "outcome")
	if err != nil || revision != nil {
		return revision, err
	}
	if observation != nil && observation.UpdatedRevision > 0 {
		return &model.BodyStateRevision{Revision: observation.UpdatedRevision}, nil
	}
	return nil, errors.New("existing observation projection has no revision")
}

// RecordSafetyEvent promotes a positive runtime safety signal into durable
// BodyState. A later negative detector result does not silently clear it; safety
// resolution needs an explicit business policy/review path.
// ResolveSafetyState is the explicit review path. Negative detector output never
// calls this method automatically.
func (s *BodyStateService) ResolveSafetyState(ctx context.Context, userID uuid.UUID, expectedRevision *int64, resolution, note string) (*model.BodyStateRevision, error) {
	resolution = strings.TrimSpace(resolution)
	if resolution != "resolved" && resolution != "cleared_by_review" && resolution != "monitoring" {
		return nil, fmt.Errorf("invalid safety resolution %q", resolution)
	}
	hasRedFlags := resolution == "monitoring"
	state := datatypes.JSON(bodyStateMustJSON(map[string]any{
		"has_red_flags":   hasRedFlags,
		"flags":           []any{},
		"status":          resolution,
		"resolution_note": strings.TrimSpace(note),
		"resolved_at":     time.Now().UTC(),
	}))
	return s.repo.SetSafetyState(ctx, userID, expectedRevision, state, "safety_review")
}

func (s *BodyStateService) RecordSafetyEvent(ctx context.Context, userID uuid.UUID, payload json.RawMessage) error {
	_, err := s.RecordSafetyEventWithReceipt(ctx, userID, payload)
	return err
}

// RecordSafetyEventWithReceipt exposes only a committed local revision to the
// owning runtime. A negative screen is not authorization to relax a restriction.
func (s *BodyStateService) RecordSafetyEventWithReceipt(ctx context.Context, userID uuid.UUID, payload json.RawMessage) (*model.BodyStateRevision, error) {
	var event struct {
		HasRedFlags bool            `json:"has_red_flags"`
		Flags       json.RawMessage `json:"flags"`
	}
	if len(payload) == 0 || json.Unmarshal(payload, &event) != nil || !event.HasRedFlags {
		return nil, nil
	}
	flags := event.Flags
	if len(flags) == 0 {
		flags = json.RawMessage(`[]`)
	}
	state := datatypes.JSON(bodyStateMustJSON(map[string]any{
		"has_red_flags": true,
		"flags":         json.RawMessage(flags),
		"status":        "requires_review",
	}))
	revision, err := s.repo.SetSafetyState(ctx, userID, nil, state, "consultation")
	if err != nil {
		return nil, err
	}
	return revision, nil
}

func bodyStateCanonicalRegionProjection(display string) (string, *string) {
	display = strings.TrimSpace(display)
	if display == "" {
		return "", nil
	}
	id, ok := ResolveCanonicalBodyRegionID(display)
	if !ok {
		return "", nil
	}
	return display, &id
}

func bodyStateConcernKey(bodyRegion string) string {
	region := strings.TrimSpace(strings.ToLower(bodyRegion))
	if region == "" {
		return "general"
	}
	return "region:" + region
}

func bodyStateHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:8])
}

func bodyStateString(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func bodyStateMustJSON(value any) []byte {
	encoded, err := json.Marshal(value)
	if err != nil {
		return []byte(`{}`)
	}
	return encoded
}

func bodyStateDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
