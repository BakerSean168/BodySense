package service

import (
	"context"
	"encoding/json"
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
	current              *model.BodyState
	revisions            []model.BodyStateRevision
	upsertedFacts        []model.BodyStateFact
	upsertedObservations []model.BodyStateObservation
	safetyStates         []datatypes.JSON
	returnExistingFact   bool
	returnExistingObs    bool
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
func (r *fakeBodyStateRepository) UpdateObservationReviewState(_ context.Context, _ uuid.UUID, _ *int64, observationID uuid.UUID, reviewState, _ string) (*model.BodyStateObservation, *model.BodyStateRevision, error) {
	return &model.BodyStateObservation{
		ID: observationID, ReviewState: reviewState,
		ExcludedFromReasoning: reviewState != "confirmed",
	}, &model.BodyStateRevision{Revision: 4}, nil
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
