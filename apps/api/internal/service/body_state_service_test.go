package service

import (
	"context"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type fakeBodyStateRepository struct {
	memoryBodyContextLedger
	current      *model.BodyState
	revisions    []model.BodyStateRevision
	restrictions []model.SafetyRestrictionRecord
	screens      []model.SafetyScreenCommand
}

func (r *fakeBodyStateRepository) Ensure(context.Context, uuid.UUID) error { return nil }
func (r *fakeBodyStateRepository) GetBodyStateHead(_ context.Context, owner uuid.UUID) (*model.BodyStateHead, error) {
	if r.current == nil {
		return &model.BodyStateHead{UserID: owner}, nil
	}
	return &model.BodyStateHead{UserID: owner, CurrentRevision: r.current.CurrentRevision}, nil
}
func (r *fakeBodyStateRepository) ListRecentRevisions(context.Context, uuid.UUID, int) ([]model.BodyStateRevision, error) {
	return r.revisions, nil
}
func (r *fakeBodyStateRepository) ListRevisionsAfter(context.Context, uuid.UUID, int64, int) ([]model.BodyStateRevision, error) {
	return r.revisions, nil
}
func TestRecordOutcomeSymptomRecordsOnlyUserWordsAsV2Symptom(t *testing.T) {
	repo := &fakeBodyStateRepository{}
	svc := NewBodyStateService(repo)
	owner := uuid.New()
	revision, err := svc.RecordOutcomeSymptom(context.Background(), owner, model.Outcome{
		ID: uuid.New(), SourceType: "web_checkin", SourceKey: "k", Kind: "symptom_change", BodyRegion: "右肩",
		Value: datatypes.JSON(`{"description":"  抬手时右肩还是疼  ","trend":"stable"}`), Notes: "not promoted",
	})
	if err != nil || revision == nil || revision.Revision != 1 {
		t.Fatalf("user-authored outcome symptom must commit one V2 revision: rev=%#v err=%v", revision, err)
	}
	if len(repo.facts) != 1 {
		t.Fatalf("expected one native fact, got %d", len(repo.facts))
	}
	fact := repo.facts[0]
	if fact.Kind != "symptom" || fact.Statement != "抬手时右肩还是疼" || fact.UserID != owner || string(fact.Details) != `{}` {
		t.Fatalf("fact must carry the user's exact words without inferred details: %#v", fact)
	}
	if string(fact.BodyRegionIDs) != `["shoulder.right"]` {
		t.Fatalf("canonical region expected, got %s", fact.BodyRegionIDs)
	}
	if len(repo.screens) != 0 {
		t.Fatal("outcome symptom must stay unscreened (Safety fail-closed)")
	}
}

func TestRecordOutcomeSymptomDropsNonUserAuthoredOutcomes(t *testing.T) {
	cases := map[string]model.Outcome{
		"adherence":          {Kind: "training_adherence", Value: datatypes.JSON(`{"checked_in":true}`)},
		"feeling only":       {Kind: "training_feedback", Value: datatypes.JSON(`{"training_feeling":"tired"}`), Notes: "sore"},
		"symptom notes only": {Kind: "symptom_change", Value: datatypes.JSON(`{"trend":"improving"}`), Notes: "better"},
		"blank description":  {Kind: "new_discomfort", Value: datatypes.JSON(`{"description":"   "}`)},
		"non-string":         {Kind: "new_discomfort", Value: datatypes.JSON(`{"description":7}`)},
	}
	for name, outcome := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &fakeBodyStateRepository{}
			outcome.ID = uuid.New()
			revision, err := NewBodyStateService(repo).RecordOutcomeSymptom(context.Background(), uuid.New(), outcome)
			if err != nil || revision != nil || len(repo.facts) != 0 {
				t.Fatalf("no BodyState write expected: rev=%#v err=%v", revision, err)
			}
		})
	}
}

func (r *fakeBodyStateRepository) ListSafetyRestrictions(_ context.Context, owner uuid.UUID) (*model.SafetyRestrictionList, error) {
	revision := r.revision
	if r.current != nil {
		revision = r.current.CurrentRevision
	}
	return &model.SafetyRestrictionList{UserID: owner, CurrentRevision: revision, Restrictions: r.restrictions}, nil
}

func (r *fakeBodyStateRepository) RecordSafetyScreen(_ context.Context, userID uuid.UUID, _ model.AnswerFactSource, command model.SafetyScreenCommand) (*model.SafetyScreen, error) {
	r.screens = append(r.screens, command)
	return &model.SafetyScreen{ID: uuid.New(), UserID: userID, CaptureID: command.CaptureID, BodyFactID: command.BodyFactID}, nil
}
