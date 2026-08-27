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
	SetSafetyState(ctx context.Context, userID uuid.UUID, safetyState datatypes.JSON, source string) (*model.BodyStateRevision, error)
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
// moving ontology ownership into the durable lane. Until this is configured,
// existing free-text-only BodyState writes continue to work while new canonical
// IDs fail closed instead of polluting durable state with unverified identities.
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
	return &BodyStateSnapshot{
		UserID:          userID,
		CurrentRevision: state.CurrentRevision,
		SafetyState:     safety,
		Facts:           facts,
		Observations:    observations,
		Hypotheses:      hypotheses,
		RecentRevisions: revisions,
	}, nil
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

// UpsertExtractedSymptom converts the existing Consultation extraction event into
// a durable Fact without making ConsultationSession the new truth source.
func (s *BodyStateService) UpsertExtractedSymptom(
	ctx context.Context,
	userID uuid.UUID,
	runID uuid.UUID,
	info json.RawMessage,
) error {
	var raw map[string]any
	if len(info) == 0 || json.Unmarshal(info, &raw) != nil {
		return nil
	}
	bodyRegion := bodyStateString(raw["body_part"])
	symptom := bodyStateString(raw["symptom_type"])
	if bodyRegion == "" && symptom == "" {
		return nil
	}

	details := map[string]any{}
	for _, key := range []string{"duration", "trigger", "relief", "severity", "additional_notes"} {
		if value := bodyStateString(raw[key]); value != "" {
			details[key] = value
		}
	}
	detailsJSON, _ := json.Marshal(details)
	provenanceJSON, _ := json.Marshal(map[string]any{
		"source_type": "consultation_extraction",
		"run_id":      runID,
		"raw":         raw,
	})

	// The extraction tool may replay after SSE reconnect/retry. A deterministic
	// source key makes that replay idempotent while still allowing a later turn to
	// update the same normalized claim when its content genuinely changes.
	sourceKey := "consultation:" + runID.String() + ":symptom:" + bodyStateHash(bodyRegion+"|"+symptom)
	_, _, err := s.repo.UpsertFact(ctx, userID, nil, model.BodyStateFact{
		ConcernKey:     bodyStateConcernKey(bodyRegion),
		Kind:           "discomfort",
		BodyRegion:     bodyRegion,
		Value:          symptom,
		Details:        datatypes.JSON(detailsJSON),
		Origin:         "ai_extracted",
		ReviewState:    "unverified",
		LifecycleState: "active",
		Trend:          "unknown",
		SourceKey:      sourceKey,
		Provenance:     datatypes.JSON(provenanceJSON),
	}, "consultation")
	return err
}

// RecordInteractionAnswer preserves the structured user answer independently from
// the chat transcript. It also creates a negative finding when the question is a
// simple safety/symptom yes/no question answered negatively.
func (s *BodyStateService) RecordInteractionAnswer(
	ctx context.Context,
	userID uuid.UUID,
	interactionID uuid.UUID,
	question datatypes.JSON,
	answer json.RawMessage,
) error {
	questionText, questionContext := bodyStateQuestion(question)
	answerText := bodyStateAnswerText(answer)
	if questionText == "" || answerText == "" {
		return nil
	}

	provenanceJSON, _ := json.Marshal(map[string]any{
		"source_type":    "ask_user",
		"interaction_id": interactionID,
		"question":       questionText,
		"context":        questionContext,
		"answer":         json.RawMessage(answer),
	})
	_, _, err := s.repo.UpsertFact(ctx, userID, nil, model.BodyStateFact{
		Kind:           "user_answer",
		Value:          answerText,
		Details:        datatypes.JSON(bodyStateMustJSON(map[string]any{"question": questionText, "context": questionContext})),
		Origin:         "structured_answer",
		ReviewState:    "confirmed",
		LifecycleState: "active",
		Trend:          "unknown",
		SourceKey:      "interaction:" + interactionID.String() + ":answer",
		Provenance:     datatypes.JSON(provenanceJSON),
	}, "consultation")
	if err != nil {
		return err
	}

	if bodyStateNegativeAnswer(answerText) && bodyStateContainsAny(questionText+questionContext, "不适", "疼痛", "酸痛", "麻木", "无力", "发热", "外伤", "放射") {
		_, _, err = s.repo.UpsertFact(ctx, userID, nil, model.BodyStateFact{
			Kind:           "negative_finding",
			Value:          "未报告：" + strings.TrimSpace(questionText),
			Origin:         "structured_answer",
			ReviewState:    "confirmed",
			LifecycleState: "active",
			Trend:          "unknown",
			SourceKey:      "interaction:" + interactionID.String() + ":negative",
			Provenance:     datatypes.JSON(provenanceJSON),
		}, "consultation")
	}
	return err
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

// RecordLifestyleContext accepts only explicit user-reported lifestyle context
// normalized by the consultation runtime. It never infers a lifestyle fact from
// symptoms or external knowledge.
func (s *BodyStateService) RecordLifestyleContext(
	ctx context.Context,
	userID uuid.UUID,
	runID uuid.UUID,
	payload json.RawMessage,
) error {
	var raw struct {
		Section string         `json:"section"`
		Summary string         `json:"summary"`
		Details map[string]any `json:"details"`
	}
	if len(payload) == 0 || json.Unmarshal(payload, &raw) != nil {
		return nil
	}
	section := strings.TrimSpace(raw.Section)
	kind, ok := lifestyleFactKind(section)
	if !ok || strings.TrimSpace(raw.Summary) == "" {
		return nil
	}
	details, _ := json.Marshal(raw.Details)
	provenance, _ := json.Marshal(map[string]any{
		"source_type": "consultation_lifestyle_extraction",
		"run_id":      runID,
		"raw":         json.RawMessage(payload),
	})
	// This is model-mediated extraction, not a deterministic structured answer.
	// Persist it durably but keep it out of current reasoning until the user
	// accepts it from the Lifestyle projection.
	_, _, err := s.repo.UpsertFact(ctx, userID, nil, model.BodyStateFact{
		ConcernKey:            "lifestyle:" + section,
		Kind:                  kind,
		Value:                 strings.TrimSpace(raw.Summary),
		Details:               datatypes.JSON(bodyStateRawOr(details, `{}`)),
		Origin:                "ai_extracted",
		ReviewState:           "unverified",
		LifecycleState:        "active",
		Trend:                 "unknown",
		SourceKey:             "consultation:" + runID.String() + ":lifestyle:" + section,
		Provenance:            datatypes.JSON(provenance),
		ExcludedFromReasoning: true,
	}, "consultation")
	return err
}

func bodyStateRawOr(value []byte, fallback string) []byte {
	if len(value) == 0 || string(value) == "null" {
		return []byte(fallback)
	}
	return value
}

func lifestyleFactKind(section string) (string, bool) {
	switch section {
	case "activity":
		return model.BodyStateFactKindLifestyleActivity, true
	case "sleep":
		return model.BodyStateFactKindLifestyleSleep, true
	case "exercise":
		return model.BodyStateFactKindLifestyleExercise, true
	case "nutrition":
		return model.BodyStateFactKindLifestyleNutrition, true
	case "substances":
		return model.BodyStateFactKindLifestyleSubstances, true
	case "recovery":
		return model.BodyStateFactKindLifestyleRecovery, true
	default:
		return "", false
	}
}

func (s *BodyStateService) UpsertFact(ctx context.Context, userID uuid.UUID, expectedRevision *int64, fact model.BodyStateFact) (*model.BodyStateFact, *model.BodyStateRevision, error) {
	regionID, err := s.normalizeBodyRegionID(fact.BodyRegionID)
	if err != nil {
		return nil, nil, err
	}
	fact.BodyRegionID = regionID
	return s.repo.UpsertFact(ctx, userID, expectedRevision, fact, "user_edit")
}

func (s *BodyStateService) CorrectFact(ctx context.Context, userID uuid.UUID, expectedRevision *int64, factID uuid.UUID, replacement model.BodyStateFact) (*model.BodyStateFact, *model.BodyStateRevision, error) {
	regionID, err := s.normalizeBodyRegionID(replacement.BodyRegionID)
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
	regionID, err := s.normalizeBodyRegionID(observation.BodyRegionID)
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
	regionID, err := s.normalizeBodyRegionID(observation.BodyRegionID)
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
		details := map[string]any{
			"outcome_kind": outcome.Kind,
			"notes":        outcome.Notes,
			"value":        value,
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
			ConcernKey:     bodyStateDefault(outcome.ConcernKey, "general"),
			Kind:           "discomfort",
			BodyRegion:     outcome.BodyRegion,
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
	observation, revision, err := s.repo.UpsertObservation(ctx, userID, nil, model.BodyStateObservation{
		ConcernKey:            bodyStateDefault(outcome.ConcernKey, "general"),
		Kind:                  observationKind,
		BodyRegion:            outcome.BodyRegion,
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
func (s *BodyStateService) ResolveSafetyState(ctx context.Context, userID uuid.UUID, resolution, note string) (*model.BodyStateRevision, error) {
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
	return s.repo.SetSafetyState(ctx, userID, state, "safety_review")
}

func (s *BodyStateService) RecordSafetyEvent(ctx context.Context, userID uuid.UUID, payload json.RawMessage) error {
	var event struct {
		HasRedFlags bool            `json:"has_red_flags"`
		Flags       json.RawMessage `json:"flags"`
	}
	if len(payload) == 0 || json.Unmarshal(payload, &event) != nil || !event.HasRedFlags {
		return nil
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
	_, err := s.repo.SetSafetyState(ctx, userID, state, "consultation")
	return err
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

func bodyStateQuestion(raw datatypes.JSON) (string, string) {
	var parsed map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &parsed) != nil {
		return "", ""
	}
	return bodyStateString(parsed["question"]), bodyStateString(parsed["context"])
}

func bodyStateAnswerText(raw json.RawMessage) string {
	var parsed any
	if len(raw) == 0 || json.Unmarshal(raw, &parsed) != nil {
		return strings.Trim(string(raw), `"`)
	}
	switch value := parsed.(type) {
	case string:
		return strings.TrimSpace(value)
	case map[string]any:
		for _, key := range []string{"text", "value"} {
			if text := bodyStateString(value[key]); text != "" {
				return text
			}
		}
		if fields, ok := value["fields"].(map[string]any); ok {
			return bodyStateString(fields)
		}
	}
	return bodyStateString(parsed)
}

func bodyStateNegativeAnswer(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "无", "没有", "否", "no", "none", "false":
		return true
	default:
		return false
	}
}

func bodyStateContainsAny(value string, words ...string) bool {
	for _, word := range words {
		if strings.Contains(value, word) {
			return true
		}
	}
	return false
}

func bodyStateMetadataIdentity(item map[string]any) (uuid.UUID, string) {
	metadata, _ := item["metadata"].(map[string]any)
	id, err := uuid.Parse(bodyStateString(metadata["body_state_item_id"]))
	if err != nil {
		return uuid.Nil, ""
	}
	return id, bodyStateString(metadata["body_state_item_type"])
}

func bodyStateMetadataJSON(item map[string]any, key, fallback string) []byte {
	metadata, _ := item["metadata"].(map[string]any)
	value, ok := metadata[key]
	if !ok || value == nil {
		return []byte(fallback)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return []byte(fallback)
	}
	return encoded
}

func bodyStateJSONMap(raw datatypes.JSON) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil || value == nil {
		return map[string]any{}
	}
	return value
}

func bodyStateLegacyDetails(details map[string]any) string {
	parts := make([]string, 0, 4)
	for _, key := range []string{"duration", "trigger", "relief", "additional_notes"} {
		if value := bodyStateString(details[key]); value != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, "，")
}

func bodyStateFactKind(category string) string {
	switch category {
	case "discomforts":
		return "discomfort"
	case "negative_findings":
		return "negative_finding"
	case "red_flags":
		return "red_flags"
	case "user_answers":
		return "user_answer"
	default:
		return ""
	}
}

func bodyStateDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func bodyStateReviewState(item map[string]any) string {
	confirmed, _ := item["confirmed"].(bool)
	if confirmed {
		return "confirmed"
	}
	return "unverified"
}
