package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type fakeLifestyleBodyState struct {
	*memoryBodyContextStore
	reviewable []model.BodyStateFact
	reviewed   map[uuid.UUID]string
}

func newFakeLifestyleBodyState(reviewable ...model.BodyStateFact) *fakeLifestyleBodyState {
	return &fakeLifestyleBodyState{memoryBodyContextStore: &memoryBodyContextStore{}, reviewable: reviewable}
}

func (f *fakeLifestyleBodyState) ListReviewableFacts(context.Context, uuid.UUID, int) ([]model.BodyStateFact, error) {
	return append([]model.BodyStateFact(nil), f.reviewable...), nil
}

func (f *fakeLifestyleBodyState) ReviewFact(_ context.Context, _ uuid.UUID, _ *int64, factID uuid.UUID, reviewState string) (*model.BodyStateFact, *model.BodyStateRevision, error) {
	if f.reviewed == nil {
		f.reviewed = map[uuid.UUID]string{}
	}
	for i, fact := range f.reviewable {
		if fact.ID == factID {
			f.reviewable = append(f.reviewable[:i], f.reviewable[i+1:]...)
			f.reviewed[factID] = reviewState
			fact.ReviewState = reviewState
			return &fact, &model.BodyStateRevision{}, nil
		}
	}
	return nil, nil, nil
}

func lifestyleCandidate(kind, value string) model.BodyStateFact {
	return model.BodyStateFact{ID: uuid.New(), Kind: kind, Value: value, Origin: "ai_extracted",
		ReviewState: "unverified", LifecycleState: "active", ExcludedFromReasoning: true}
}

func TestLifestyleProjectionKeepsUnverifiedCandidatePending(t *testing.T) {
	candidate := lifestyleCandidate(model.BodyStateFactKindLifestyleSleep, "AI 猜测失眠")
	bodyState := newFakeLifestyleBodyState(candidate)
	owner := uuid.New()
	svc := NewLifestyleService(bodyState)
	if _, err := svc.Update(context.Background(), owner, UpdateLifestyleRequest{Sleep: &LifestyleSectionInput{Summary: "轮班，睡 6-7 小时"}}); err != nil {
		t.Fatal(err)
	}
	result, err := svc.Get(context.Background(), owner)
	if err != nil {
		t.Fatal(err)
	}
	if result.Sleep.Summary != "轮班，睡 6-7 小时" {
		t.Fatalf("sleep summary=%q, want the user's own statement", result.Sleep.Summary)
	}
	if len(result.PendingUpdates) != 1 || result.PendingUpdates[0].FactID != candidate.ID {
		t.Fatalf("pending updates=%#v, want reviewable candidate", result.PendingUpdates)
	}
}

func TestLifestyleAcceptCandidateRecordsItAsUserStatement(t *testing.T) {
	candidate := lifestyleCandidate(model.BodyStateFactKindLifestyleActivity, "现在每天走动和站立更多")
	bodyState := newFakeLifestyleBodyState(candidate)
	svc := NewLifestyleService(bodyState)
	owner := uuid.New()
	result, err := svc.AcceptCandidate(context.Background(), owner, nil, candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Activity.Summary != "现在每天走动和站立更多" || len(result.PendingUpdates) != 0 {
		t.Fatalf("unexpected projection after accept: %#v", result)
	}
	if bodyState.reviewed[candidate.ID] != "confirmed" {
		t.Fatalf("accepted candidate must be closed: %#v", bodyState.reviewed)
	}
}

func TestLifestyleRejectCandidateRemovesItFromPendingWithoutChangingCurrent(t *testing.T) {
	candidate := lifestyleCandidate(model.BodyStateFactKindLifestyleSleep, "AI 误提取的夜班")
	bodyState := newFakeLifestyleBodyState(candidate)
	svc := NewLifestyleService(bodyState)
	owner := uuid.New()
	if _, err := svc.Update(context.Background(), owner, UpdateLifestyleRequest{Sleep: &LifestyleSectionInput{Summary: "作息规律"}}); err != nil {
		t.Fatal(err)
	}
	result, err := svc.RejectCandidate(context.Background(), owner, nil, candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	if bodyState.reviewed[candidate.ID] != "rejected" || result.Sleep.Summary != "作息规律" || len(result.PendingUpdates) != 0 {
		t.Fatalf("unexpected projection after reject: %#v", result)
	}
}

func TestLifestyleCandidateReviewFailsClosedForNonLifestyleFact(t *testing.T) {
	candidate := lifestyleCandidate("discomfort", "颈肩酸胀")
	bodyState := newFakeLifestyleBodyState(candidate)
	svc := NewLifestyleService(bodyState)
	if _, err := svc.AcceptCandidate(context.Background(), uuid.New(), nil, candidate.ID); !errors.Is(err, ErrInvalidLifestyleCandidate) {
		t.Fatalf("accept non-lifestyle candidate error=%v", err)
	}
	if _, err := svc.RejectCandidate(context.Background(), uuid.New(), nil, candidate.ID); !errors.Is(err, ErrInvalidLifestyleCandidate) {
		t.Fatalf("reject non-lifestyle candidate error=%v", err)
	}
	if len(bodyState.reviewed) != 0 || len(bodyState.updates) != 0 {
		t.Fatalf("non-lifestyle candidate reached mutation boundary")
	}
}
