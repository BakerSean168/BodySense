package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// fakeBodyStateRepository keeps these tests at the application boundary. The
// SQL/repository transaction details are intentionally separate from service
// semantics such as projection identity and producer mapping.
type fakeBodyStateRepository struct {
	current                  *model.BodyState
	revisions                []model.BodyStateRevision
	upsertedFacts            []model.BodyStateFact
	transitionedFacts        []model.BodyStateFact
	transitionFactTargets    []uuid.UUID
	upsertedObservations     []model.BodyStateObservation
	transitionedObservations []model.BodyStateObservation
	appliedPatches           []model.BodyStateCurrentContextPatch
	safetyStates             []datatypes.JSON
	returnExistingFact       bool
	returnExistingObs        bool
}

func (r *fakeBodyStateRepository) Ensure(context.Context, uuid.UUID) error { return nil }
func (r *fakeBodyStateRepository) GetCurrent(context.Context, uuid.UUID) (*model.BodyState, error) {
	if r.current == nil {
		return &model.BodyState{SafetyState: datatypes.JSON(`{}`)}, nil
	}
	return r.current, nil
}
func (r *fakeBodyStateRepository) ListRecentRevisions(context.Context, uuid.UUID, int) ([]model.BodyStateRevision, error) {
	return r.revisions, nil
}
func (r *fakeBodyStateRepository) ListReviewableObservations(context.Context, uuid.UUID, int) ([]model.BodyStateObservation, error) {
	result := make([]model.BodyStateObservation, 0)
	for _, observation := range r.upsertedObservations {
		if observation.ReviewState == "unverified" {
			result = append(result, observation)
		}
	}
	return result, nil
}
func (r *fakeBodyStateRepository) ListReviewableFacts(context.Context, uuid.UUID, int) ([]model.BodyStateFact, error) {
	result := make([]model.BodyStateFact, 0)
	for _, fact := range r.upsertedFacts {
		if fact.ReviewState == "unverified" {
			result = append(result, fact)
		}
	}
	return result, nil
}
func (r *fakeBodyStateRepository) ListRevisionsAfter(context.Context, uuid.UUID, int64, int) ([]model.BodyStateRevision, error) {
	return r.revisions, nil
}
func (r *fakeBodyStateRepository) UpsertFact(_ context.Context, userID uuid.UUID, _ *int64, fact model.BodyStateFact, _ string) (*model.BodyStateFact, *model.BodyStateRevision, error) {
	if fact.ID == uuid.Nil {
		fact.ID = uuid.New()
	}
	fact.UserID = userID
	r.upsertedFacts = append(r.upsertedFacts, fact)
	if r.returnExistingFact {
		fact.UpdatedRevision = 17
		return &fact, nil, nil
	}
	return &fact, &model.BodyStateRevision{Revision: int64(len(r.upsertedFacts))}, nil
}
func (r *fakeBodyStateRepository) CorrectFact(context.Context, uuid.UUID, *int64, uuid.UUID, model.BodyStateFact, string) (*model.BodyStateFact, *model.BodyStateRevision, error) {
	return nil, nil, nil
}
func (r *fakeBodyStateRepository) TransitionFact(_ context.Context, userID uuid.UUID, _ *int64, target uuid.UUID, replacement model.BodyStateFact, _ time.Time, _ string) (*model.BodyStateFact, *model.BodyStateRevision, error) {
	replacement.UserID = userID
	r.transitionFactTargets = append(r.transitionFactTargets, target)
	r.transitionedFacts = append(r.transitionedFacts, replacement)
	return &replacement, &model.BodyStateRevision{Revision: 2}, nil
}
func (r *fakeBodyStateRepository) AcceptCurrentFactCandidate(_ context.Context, _ uuid.UUID, _ *int64, candidateID uuid.UUID, _ time.Time, _ string) (*model.BodyStateFact, *model.BodyStateRevision, error) {
	for index := range r.upsertedFacts {
		if r.upsertedFacts[index].ID == candidateID {
			r.upsertedFacts[index].ReviewState = "confirmed"
			r.upsertedFacts[index].ExcludedFromReasoning = false
			fact := r.upsertedFacts[index]
			return &fact, &model.BodyStateRevision{Revision: 4}, nil
		}
	}
	return nil, nil, nil
}
func (r *fakeBodyStateRepository) UpdateFactTemporal(context.Context, uuid.UUID, *int64, uuid.UUID, string, string, *time.Time, string) (*model.BodyStateFact, *model.BodyStateRevision, error) {
	return nil, nil, nil
}
func (r *fakeBodyStateRepository) UpdateFactReviewState(_ context.Context, _ uuid.UUID, _ *int64, factID uuid.UUID, reviewState, _ string) (*model.BodyStateFact, *model.BodyStateRevision, error) {
	return &model.BodyStateFact{ID: factID, ReviewState: reviewState}, &model.BodyStateRevision{Revision: 3}, nil
}

func (r *fakeBodyStateRepository) UpsertObservation(_ context.Context, userID uuid.UUID, _ *int64, observation model.BodyStateObservation, _ string) (*model.BodyStateObservation, *model.BodyStateRevision, error) {
	if observation.ID == uuid.Nil {
		observation.ID = uuid.New()
	}
	observation.UserID = userID
	r.upsertedObservations = append(r.upsertedObservations, observation)
	if r.returnExistingObs {
		observation.UpdatedRevision = 18
		return &observation, nil, nil
	}
	return &observation, &model.BodyStateRevision{Revision: int64(len(r.upsertedObservations))}, nil
}
func (r *fakeBodyStateRepository) TransitionObservation(_ context.Context, userID uuid.UUID, _ *int64, _ uuid.UUID, replacement model.BodyStateObservation, _ string) (*model.BodyStateObservation, *model.BodyStateRevision, error) {
	replacement.UserID = userID
	r.transitionedObservations = append(r.transitionedObservations, replacement)
	return &replacement, &model.BodyStateRevision{Revision: 2}, nil
}
func (r *fakeBodyStateRepository) UpdateObservationReviewState(_ context.Context, _ uuid.UUID, _ *int64, observationID uuid.UUID, reviewState, _ string) (*model.BodyStateObservation, *model.BodyStateRevision, error) {
	return &model.BodyStateObservation{
		ID: observationID, ReviewState: reviewState,
		ExcludedFromReasoning: reviewState != "confirmed",
	}, &model.BodyStateRevision{Revision: 4}, nil
}
func (r *fakeBodyStateRepository) ApplyCurrentContextPatch(_ context.Context, userID uuid.UUID, _ *int64, patch model.BodyStateCurrentContextPatch, _ string) (*model.BodyStateRevision, error) {
	r.appliedPatches = append(r.appliedPatches, patch)
	if r.current == nil {
		r.current = &model.BodyState{UserID: userID, SafetyState: datatypes.JSON(`{}`)}
	}
	changed := false
	for _, mutation := range patch.Facts {
		next := make([]model.BodyStateFact, 0, len(r.current.Facts)+1)
		for _, fact := range r.current.Facts {
			if fact.Kind != mutation.Kind {
				next = append(next, fact)
			}
		}
		if mutation.Replacement != nil && mutation.Replacement.Value != "" {
			fact := *mutation.Replacement
			if fact.ID == uuid.Nil {
				fact.ID = uuid.New()
			}
			fact.UserID = userID
			fact.Kind = mutation.Kind
			fact.LifecycleState = "active"
			next = append(next, fact)
		}
		r.current.Facts = next
		changed = true
	}
	for _, mutation := range patch.Observations {
		next := make([]model.BodyStateObservation, 0, len(r.current.Observations)+1)
		for _, observation := range r.current.Observations {
			if observation.Kind != mutation.Kind {
				next = append(next, observation)
			}
		}
		if mutation.Replacement != nil {
			observation := *mutation.Replacement
			if observation.ID == uuid.Nil {
				observation.ID = uuid.New()
			}
			observation.UserID = userID
			observation.Kind = mutation.Kind
			observation.LifecycleState = "active"
			next = append(next, observation)
		}
		r.current.Observations = next
		changed = true
	}
	if !changed {
		return nil, nil
	}
	r.current.CurrentRevision++
	return &model.BodyStateRevision{Revision: r.current.CurrentRevision}, nil
}

func (r *fakeBodyStateRepository) SetSafetyState(_ context.Context, _ uuid.UUID, state datatypes.JSON, _ string) (*model.BodyStateRevision, error) {
	r.safetyStates = append(r.safetyStates, state)
	return &model.BodyStateRevision{Revision: int64(len(r.safetyStates))}, nil
}
func (r *fakeBodyStateRepository) UpsertEvidence(_ context.Context, userID uuid.UUID, evidence model.BodyStateEvidence) (*model.BodyStateEvidence, error) {
	evidence.UserID = userID
	if evidence.ID == uuid.Nil {
		evidence.ID = uuid.New()
	}
	return &evidence, nil
}
func (r *fakeBodyStateRepository) ListEvidence(context.Context, uuid.UUID, int) ([]model.BodyStateEvidence, error) {
	return nil, nil
}
func (r *fakeBodyStateRepository) GetEvidenceByIDs(context.Context, uuid.UUID, []uuid.UUID) ([]model.BodyStateEvidence, error) {
	return nil, nil
}
func (r *fakeBodyStateRepository) AddHypothesis(_ context.Context, userID uuid.UUID, _ *int64, hypothesis model.BodyStateHypothesis, _ string) (*model.BodyStateHypothesis, *model.BodyStateRevision, error) {
	hypothesis.ID = uuid.New()
	hypothesis.UserID = userID
	return &hypothesis, &model.BodyStateRevision{Revision: 1}, nil
}
func (r *fakeBodyStateRepository) UpdateHypothesisLifecycle(_ context.Context, userID uuid.UUID, _ *int64, hypothesisID uuid.UUID, lifecycleState string, counterevidenceIDs datatypes.JSON, _ string) (*model.BodyStateHypothesis, *model.BodyStateRevision, error) {
	return &model.BodyStateHypothesis{ID: hypothesisID, UserID: userID, LifecycleState: lifecycleState, CounterevidenceIDs: counterevidenceIDs}, &model.BodyStateRevision{Revision: 2}, nil
}

func TestBodyStateExtractedSymptomCreatesUnverifiedFact(t *testing.T) {
	repo := &fakeBodyStateRepository{}
	svc := NewBodyStateService(repo)
	if err := svc.UpsertExtractedSymptom(
		context.Background(), uuid.New(), uuid.New(),
		json.RawMessage(`{"body_part":"左臀","symptom_type":"酸胀","trigger":"久坐"}`),
	); err != nil {
		t.Fatalf("UpsertExtractedSymptom returned error: %v", err)
	}
	if len(repo.upsertedFacts) != 1 {
		t.Fatalf("expected one fact, got %d", len(repo.upsertedFacts))
	}
	fact := repo.upsertedFacts[0]
	if fact.Kind != "discomfort" || fact.BodyRegion != "左臀" || fact.Value != "酸胀" {
		t.Fatalf("unexpected fact: %#v", fact)
	}
	if fact.Origin != "ai_extracted" || fact.ReviewState != "unverified" {
		t.Fatalf("AI extraction must not become confirmed user truth: %#v", fact)
	}
}

func TestBodyStateSafetyOnlyPersistsPositiveSignals(t *testing.T) {
	repo := &fakeBodyStateRepository{}
	svc := NewBodyStateService(repo)
	userID := uuid.New()

	if err := svc.RecordSafetyEvent(context.Background(), userID, json.RawMessage(`{"has_red_flags":false,"flags":[]}`)); err != nil {
		t.Fatalf("negative safety event returned error: %v", err)
	}
	if len(repo.safetyStates) != 0 {
		t.Fatal("negative detector result must not silently clear/create durable safety state")
	}

	if err := svc.RecordSafetyEvent(context.Background(), userID, json.RawMessage(`{"has_red_flags":true,"flags":[{"type":"weakness"}]}`)); err != nil {
		t.Fatalf("positive safety event returned error: %v", err)
	}
	if len(repo.safetyStates) != 1 {
		t.Fatalf("expected one durable safety state, got %d", len(repo.safetyStates))
	}
}

func TestBodyStateRecordOutcomeReturnsExistingFactRevisionForIdempotentReplay(t *testing.T) {
	repo := &fakeBodyStateRepository{returnExistingFact: true}
	svc := NewBodyStateService(repo)
	revision, err := svc.RecordOutcome(context.Background(), uuid.New(), model.Outcome{
		ID: uuid.New(), SourceType: "training_feedback", SourceKey: "same",
		Kind: "symptom_change", Value: datatypes.JSON(`{"description":"better"}`),
	})
	if err != nil {
		t.Fatalf("RecordOutcome returned error: %v", err)
	}
	if revision == nil || revision.Revision != 17 {
		t.Fatalf("idempotent replay must recover existing fact revision, got %#v", revision)
	}
}

func TestBodyStateRecordOutcomeReturnsExistingObservationRevisionForIdempotentReplay(t *testing.T) {
	repo := &fakeBodyStateRepository{returnExistingObs: true}
	svc := NewBodyStateService(repo)
	revision, err := svc.RecordOutcome(context.Background(), uuid.New(), model.Outcome{
		ID: uuid.New(), SourceType: "training_checkin", SourceKey: "same",
		Kind: "training_adherence", Value: datatypes.JSON(`{"checked_in":true}`),
	})
	if err != nil {
		t.Fatalf("RecordOutcome returned error: %v", err)
	}
	if revision == nil || revision.Revision != 18 {
		t.Fatalf("idempotent replay must recover existing observation revision, got %#v", revision)
	}
}

func TestAssessmentObservationRemainsExcludedUntilUserConfirmation(t *testing.T) {
	repo := &fakeBodyStateRepository{}
	svc := NewBodyStateService(repo)
	stored, revision, err := svc.AddAssessmentObservation(context.Background(), uuid.New(), model.BodyStateObservation{
		Kind: "posture_alignment", BodyRegion: "肩部",
		Value: datatypes.JSON(`{"label":"高低肩倾向"}`),
	})
	if err != nil {
		t.Fatalf("AddAssessmentObservation returned error: %v", err)
	}
	if revision == nil || stored.ReviewState != "unverified" || !stored.ExcludedFromReasoning {
		t.Fatalf("AI observation must be reviewable but excluded: stored=%#v revision=%#v", stored, revision)
	}

	confirmed, reviewRevision, err := svc.ReviewObservation(
		context.Background(), uuid.New(), nil, stored.ID, "confirmed",
	)
	if err != nil {
		t.Fatalf("ReviewObservation returned error: %v", err)
	}
	if reviewRevision == nil || confirmed.ReviewState != "confirmed" || confirmed.ExcludedFromReasoning {
		t.Fatalf("confirmed observation must enter reasoning: %#v", confirmed)
	}
}

func TestUserObservationIsConfirmedAtCreation(t *testing.T) {
	repo := &fakeBodyStateRepository{}
	svc := NewBodyStateService(repo)
	stored, _, err := svc.AddObservation(context.Background(), uuid.New(), nil, model.BodyStateObservation{
		Kind: "self_measurement", Value: datatypes.JSON(`{"value":10}`),
	})
	if err != nil {
		t.Fatalf("AddObservation returned error: %v", err)
	}
	if stored.ReviewState != "confirmed" || stored.ExcludedFromReasoning {
		t.Fatalf("explicit user observation should be confirmed: %#v", stored)
	}
}

func TestBodyStateSnapshotNormalizesEmptyCollections(t *testing.T) {
	repo := &fakeBodyStateRepository{current: &model.BodyState{
		UserID: uuid.New(), SafetyState: datatypes.JSON(`{}`),
	}}
	svc := NewBodyStateService(repo)
	snapshot, err := svc.GetSnapshot(context.Background(), repo.current.UserID, 30)
	if err != nil {
		t.Fatalf("GetSnapshot returned error: %v", err)
	}
	if snapshot.Facts == nil || snapshot.Observations == nil || snapshot.Hypotheses == nil || snapshot.RecentRevisions == nil {
		t.Fatalf("BodyState JSON collections must be empty arrays, not nil: %#v", snapshot)
	}
	pending, err := svc.ListReviewableObservations(context.Background(), repo.current.UserID, 50)
	if err != nil {
		t.Fatalf("ListReviewableObservations returned error: %v", err)
	}
	if pending == nil {
		t.Fatal("pending observations must serialize as an empty array")
	}
}

func TestBodyStateCanonicalRegionIDUsesInjectedAuthority(t *testing.T) {
	repo := &fakeBodyStateRepository{}
	svc := NewBodyStateService(repo).WithBodyRegionIDValidator(BodyRegionIDValidatorFunc(func(id string) bool {
		return id == "shoulder.right"
	}))
	rawRegionID := "  shoulder.right  "
	stored, revision, err := svc.UpsertFact(context.Background(), uuid.New(), nil, model.BodyStateFact{
		Kind:         "discomfort",
		BodyRegion:   "右肩",
		BodyRegionID: &rawRegionID,
		Value:        "抬手疼痛",
	})
	if err != nil {
		t.Fatalf("UpsertFact returned error: %v", err)
	}
	if revision == nil || stored.BodyRegionID == nil || *stored.BodyRegionID != "shoulder.right" {
		t.Fatalf("canonical region id must round-trip after validation: stored=%#v revision=%#v", stored, revision)
	}
	if stored.BodyRegion != "右肩" {
		t.Fatalf("raw/display body_region must be preserved, got %q", stored.BodyRegion)
	}
}

func TestBodyStateUnknownCanonicalRegionIDIsRejectedBeforePersistence(t *testing.T) {
	repo := &fakeBodyStateRepository{}
	svc := NewBodyStateService(repo).WithBodyRegionIDValidator(BodyRegionIDValidatorFunc(func(id string) bool {
		return id == "shoulder.right"
	}))
	unknown := "shoulder.middle"
	_, _, err := svc.UpsertFact(context.Background(), uuid.New(), nil, model.BodyStateFact{
		Kind:         "discomfort",
		BodyRegion:   "肩部",
		BodyRegionID: &unknown,
		Value:        "疼痛",
	})
	if !errors.Is(err, ErrUnknownBodyRegionID) {
		t.Fatalf("unknown canonical region must be rejected, got %v", err)
	}
	if len(repo.upsertedFacts) != 0 {
		t.Fatalf("invalid canonical region must not reach persistence: %#v", repo.upsertedFacts)
	}
}

func TestBodyStateCanonicalRegionRequiresAuthorityButLegacyNullRemainsWritable(t *testing.T) {
	repo := &fakeBodyStateRepository{}
	svc := NewBodyStateService(repo)
	canonical := "shoulder.right"
	_, _, err := svc.UpsertFact(context.Background(), uuid.New(), nil, model.BodyStateFact{
		Kind:         "discomfort",
		BodyRegion:   "右肩",
		BodyRegionID: &canonical,
		Value:        "疼痛",
	})
	if !errors.Is(err, ErrBodyRegionIDValidationUnavailable) {
		t.Fatalf("canonical region without ontology authority must fail closed, got %v", err)
	}

	legacy, _, err := svc.UpsertFact(context.Background(), uuid.New(), nil, model.BodyStateFact{
		Kind:       "discomfort",
		BodyRegion: "肩颈",
		Value:      "紧张",
	})
	if err != nil {
		t.Fatalf("legacy free-text fact must remain writable: %v", err)
	}
	if legacy.BodyRegionID != nil {
		t.Fatalf("ambiguous legacy fact must remain unresolved, got %q", *legacy.BodyRegionID)
	}
}

func TestBodyStateObservationCanonicalRegionRetainsLaterality(t *testing.T) {
	repo := &fakeBodyStateRepository{}
	svc := NewBodyStateService(repo).WithBodyRegionIDValidator(BodyRegionIDValidatorFunc(func(id string) bool {
		return id == "knee.left" || id == "knee.right"
	}))
	left := "knee.left"
	stored, _, err := svc.AddObservation(context.Background(), uuid.New(), nil, model.BodyStateObservation{
		Kind:         "self_measurement",
		BodyRegion:   "左膝",
		BodyRegionID: &left,
		Value:        datatypes.JSON(`{"pain":3}`),
	})
	if err != nil {
		t.Fatalf("AddObservation returned error: %v", err)
	}
	if stored.BodyRegionID == nil || *stored.BodyRegionID != "knee.left" || stored.BodyRegion != "左膝" {
		t.Fatalf("observation laterality must round-trip: %#v", stored)
	}
}

func TestSetCurrentFactRoutesLifestyleChangeThroughAtomicContextPatch(t *testing.T) {
	userID := uuid.New()
	currentID := uuid.New()
	repo := &fakeBodyStateRepository{current: &model.BodyState{
		UserID:          userID,
		CurrentRevision: 7,
		Facts: []model.BodyStateFact{{
			ID: currentID, UserID: userID,
			Kind:           model.BodyStateFactKindLifestyleSleep,
			Value:          "作息规律，通常睡 7-8 小时",
			Details:        datatypes.JSON(`{"regularity":"regular"}`),
			LifecycleState: "active",
		}},
	}}
	svc := NewBodyStateService(repo)
	expected := int64(7)
	_, revision, err := svc.SetCurrentFact(
		context.Background(), userID, &expected,
		model.BodyStateFactKindLifestyleSleep,
		&model.BodyStateFact{
			Value:   "最近换夜班，通常凌晨 5 点睡",
			Details: datatypes.JSON(`{"shift_work":true}`),
			Origin:  "user_reported", ReviewState: "confirmed",
		},
		time.Date(2026, time.August, 27, 10, 0, 0, 0, time.UTC), "test",
	)
	if err != nil {
		t.Fatalf("SetCurrentFact returned error: %v", err)
	}
	if revision == nil || len(repo.appliedPatches) != 1 {
		t.Fatalf("expected one atomic context patch, revision=%v patches=%d", revision, len(repo.appliedPatches))
	}
	patch := repo.appliedPatches[0]
	if len(patch.Facts) != 1 || patch.Facts[0].Kind != model.BodyStateFactKindLifestyleSleep {
		t.Fatalf("unexpected patch: %#v", patch)
	}
	if patch.Facts[0].Replacement == nil || patch.Facts[0].Replacement.Value != "最近换夜班，通常凌晨 5 点睡" {
		t.Fatalf("unexpected replacement: %#v", patch.Facts[0].Replacement)
	}
}

func TestApplyCurrentContextPatchRejectsDuplicateKinds(t *testing.T) {
	repo := &fakeBodyStateRepository{}
	svc := NewBodyStateService(repo)
	_, err := svc.ApplyCurrentContextPatch(context.Background(), uuid.New(), nil, model.BodyStateCurrentContextPatch{
		Facts: []model.BodyStateCurrentFactMutation{
			{Kind: model.BodyStateFactKindLifestyleActivity, Replacement: &model.BodyStateFact{Value: "久坐"}},
			{Kind: model.BodyStateFactKindLifestyleActivity, Replacement: &model.BodyStateFact{Value: "久站"}},
		},
	}, "test")
	if err == nil {
		t.Fatal("duplicate singleton kinds must be rejected before repository mutation")
	}
	if len(repo.appliedPatches) != 0 {
		t.Fatal("invalid patch must not reach repository")
	}
}

func TestRecordLifestyleContextCreatesReviewableExcludedCandidate(t *testing.T) {
	repo := &fakeBodyStateRepository{}
	svc := NewBodyStateService(repo)
	runID := uuid.New()
	err := svc.RecordLifestyleContext(
		context.Background(), uuid.New(), runID,
		json.RawMessage(`{"section":"substances","summary":"每天喝两杯咖啡，不吸烟","details":{"caffeine_cups":2,"smoking":false}}`),
	)
	if err != nil {
		t.Fatalf("RecordLifestyleContext returned error: %v", err)
	}
	if len(repo.upsertedFacts) != 1 {
		t.Fatalf("expected one durable candidate, got %#v", repo.upsertedFacts)
	}
	fact := repo.upsertedFacts[0]
	if fact.Kind != model.BodyStateFactKindLifestyleSubstances || fact.Value != "每天喝两杯咖啡，不吸烟" {
		t.Fatalf("unexpected lifestyle candidate: %#v", fact)
	}
	if fact.ReviewState != "unverified" || fact.Origin != "ai_extracted" || !fact.ExcludedFromReasoning {
		t.Fatalf("model-mediated extraction must remain reviewable and excluded: %#v", fact)
	}
	if fact.SourceKey != "consultation:"+runID.String()+":lifestyle:substances" {
		t.Fatalf("unexpected idempotency key: %q", fact.SourceKey)
	}
	pending, err := svc.ListReviewableFacts(context.Background(), fact.UserID, 50)
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending candidates=%#v err=%v", pending, err)
	}
}
