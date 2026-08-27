package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type fakeLifestyleBodyState struct {
	snapshot    BodyStateSnapshot
	patches     []model.BodyStateCurrentContextPatch
	reviewable  []model.BodyStateFact
	acceptedIDs []uuid.UUID
	rejectedIDs []uuid.UUID
}

func (f *fakeLifestyleBodyState) GetSnapshot(context.Context, uuid.UUID, int) (*BodyStateSnapshot, error) {
	copy := f.snapshot
	return &copy, nil
}

func (f *fakeLifestyleBodyState) ListReviewableFacts(context.Context, uuid.UUID, int) ([]model.BodyStateFact, error) {
	return append([]model.BodyStateFact(nil), f.reviewable...), nil
}

func (f *fakeLifestyleBodyState) AcceptCurrentFactCandidate(_ context.Context, _ uuid.UUID, _ *int64, candidateID uuid.UUID, _ time.Time) (*model.BodyStateFact, *model.BodyStateRevision, error) {
	f.acceptedIDs = append(f.acceptedIDs, candidateID)
	for i, fact := range f.reviewable {
		if fact.ID != candidateID {
			continue
		}
		fact.ReviewState = "confirmed"
		fact.ExcludedFromReasoning = false
		f.reviewable = append(f.reviewable[:i], f.reviewable[i+1:]...)
		replaced := false
		for j := range f.snapshot.Facts {
			if f.snapshot.Facts[j].Kind == fact.Kind && f.snapshot.Facts[j].ReviewState == "confirmed" {
				f.snapshot.Facts[j] = fact
				replaced = true
				break
			}
		}
		if !replaced {
			f.snapshot.Facts = append(f.snapshot.Facts, fact)
		}
		f.snapshot.CurrentRevision++
		return &fact, &model.BodyStateRevision{Revision: f.snapshot.CurrentRevision}, nil
	}
	return nil, nil, nil
}

func (f *fakeLifestyleBodyState) ReviewFact(_ context.Context, _ uuid.UUID, _ *int64, factID uuid.UUID, reviewState string) (*model.BodyStateFact, *model.BodyStateRevision, error) {
	if reviewState == "rejected" {
		f.rejectedIDs = append(f.rejectedIDs, factID)
	}
	for i, fact := range f.reviewable {
		if fact.ID != factID {
			continue
		}
		f.reviewable = append(f.reviewable[:i], f.reviewable[i+1:]...)
		fact.ReviewState = reviewState
		fact.ExcludedFromReasoning = reviewState == "rejected"
		f.snapshot.CurrentRevision++
		return &fact, &model.BodyStateRevision{Revision: f.snapshot.CurrentRevision}, nil
	}
	return nil, nil, nil
}

func (f *fakeLifestyleBodyState) ApplyCurrentContextPatch(_ context.Context, _ uuid.UUID, _ *int64, patch model.BodyStateCurrentContextPatch, _ string) (*model.BodyStateRevision, error) {
	f.patches = append(f.patches, patch)
	f.snapshot.CurrentRevision++
	return &model.BodyStateRevision{Revision: f.snapshot.CurrentRevision}, nil
}

func TestLifestyleUpdateBatchesMultipleSectionsIntoOneBodyStateMutation(t *testing.T) {
	bodyState := &fakeLifestyleBodyState{snapshot: BodyStateSnapshot{CurrentRevision: 10}}
	svc := NewLifestyleService(bodyState)
	revision := int64(10)
	_, err := svc.Update(context.Background(), uuid.New(), dto.UpdateLifestyleRequest{
		ExpectedRevision: &revision,
		Activity:         &dto.LifestyleSectionInput{Summary: "久坐为主"},
		Sleep:            &dto.LifestyleSectionInput{Summary: "轮班，平均睡 6-7 小时"},
	})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if len(bodyState.patches) != 1 {
		t.Fatalf("one user save must create one BodyState patch, got %d", len(bodyState.patches))
	}
	patch := bodyState.patches[0]
	if len(patch.Facts) != 2 {
		t.Fatalf("patch facts = %d, want 2", len(patch.Facts))
	}
	if patch.Facts[0].Kind != model.BodyStateFactKindLifestyleActivity || patch.Facts[1].Kind != model.BodyStateFactKindLifestyleSleep {
		t.Fatalf("unexpected lifestyle patch kinds: %#v", patch.Facts)
	}
}

func TestLifestyleProjectionIgnoresUnverifiedCandidate(t *testing.T) {
	candidateID := uuid.New()
	candidate := model.BodyStateFact{ID: candidateID, Kind: model.BodyStateFactKindLifestyleSleep, Value: "AI 猜测失眠", Origin: "ai_extracted", ReviewState: "unverified", LifecycleState: "active", ExcludedFromReasoning: true}
	bodyState := &fakeLifestyleBodyState{
		snapshot: BodyStateSnapshot{
			CurrentRevision: 3,
			Facts: []model.BodyStateFact{
				{ID: uuid.New(), Kind: model.BodyStateFactKindLifestyleSleep, Value: "用户确认轮班，睡 6-7 小时", ReviewState: "confirmed", LifecycleState: "active"},
			},
		},
		reviewable: []model.BodyStateFact{candidate},
	}
	svc := NewLifestyleService(bodyState)
	result, err := svc.Get(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if result.Sleep.Summary != "用户确认轮班，睡 6-7 小时" {
		t.Fatalf("sleep summary=%q, want confirmed fact", result.Sleep.Summary)
	}
	if len(result.PendingUpdates) != 1 || result.PendingUpdates[0].FactID != candidateID {
		t.Fatalf("pending updates=%#v, want reviewable candidate", result.PendingUpdates)
	}
}

func TestLifestyleAcceptCandidatePromotesPendingUpdate(t *testing.T) {
	candidateID := uuid.New()
	bodyState := &fakeLifestyleBodyState{
		snapshot: BodyStateSnapshot{
			CurrentRevision: 7,
			Facts: []model.BodyStateFact{{
				ID: uuid.New(), Kind: model.BodyStateFactKindLifestyleActivity, Value: "久坐为主",
				ReviewState: "confirmed", LifecycleState: "active",
			}},
		},
		reviewable: []model.BodyStateFact{{
			ID: candidateID, Kind: model.BodyStateFactKindLifestyleActivity, Value: "现在每天走动和站立更多",
			Origin: "ai_extracted", ReviewState: "unverified", LifecycleState: "active", ExcludedFromReasoning: true,
		}},
	}
	svc := NewLifestyleService(bodyState)
	expected := int64(7)
	result, err := svc.AcceptCandidate(context.Background(), uuid.New(), &expected, candidateID)
	if err != nil {
		t.Fatalf("AcceptCandidate returned error: %v", err)
	}
	if len(bodyState.acceptedIDs) != 1 || bodyState.acceptedIDs[0] != candidateID {
		t.Fatalf("accepted ids=%#v", bodyState.acceptedIDs)
	}
	if result.Activity.Summary != "现在每天走动和站立更多" || len(result.PendingUpdates) != 0 {
		t.Fatalf("unexpected projection after accept: %#v", result)
	}
}

func TestLifestyleRejectCandidateRemovesItFromPendingWithoutChangingCurrent(t *testing.T) {
	candidateID := uuid.New()
	bodyState := &fakeLifestyleBodyState{
		snapshot: BodyStateSnapshot{
			CurrentRevision: 9,
			Facts: []model.BodyStateFact{{
				ID: uuid.New(), Kind: model.BodyStateFactKindLifestyleSleep, Value: "作息规律",
				ReviewState: "confirmed", LifecycleState: "active",
			}},
		},
		reviewable: []model.BodyStateFact{{
			ID: candidateID, Kind: model.BodyStateFactKindLifestyleSleep, Value: "AI 误提取的夜班",
			Origin: "ai_extracted", ReviewState: "unverified", LifecycleState: "active", ExcludedFromReasoning: true,
		}},
	}
	svc := NewLifestyleService(bodyState)
	expected := int64(9)
	result, err := svc.RejectCandidate(context.Background(), uuid.New(), &expected, candidateID)
	if err != nil {
		t.Fatalf("RejectCandidate returned error: %v", err)
	}
	if len(bodyState.rejectedIDs) != 1 || bodyState.rejectedIDs[0] != candidateID {
		t.Fatalf("rejected ids=%#v", bodyState.rejectedIDs)
	}
	if result.Sleep.Summary != "作息规律" || len(result.PendingUpdates) != 0 {
		t.Fatalf("unexpected projection after reject: %#v", result)
	}
}

func TestLifestyleCandidateReviewFailsClosedForNonLifestyleFact(t *testing.T) {
	candidateID := uuid.New()
	bodyState := &fakeLifestyleBodyState{
		snapshot: BodyStateSnapshot{CurrentRevision: 4},
		reviewable: []model.BodyStateFact{{
			ID: candidateID, Kind: "discomfort", Value: "颈肩酸胀",
			Origin: "ai_extracted", ReviewState: "unverified", LifecycleState: "active", ExcludedFromReasoning: true,
		}},
	}
	svc := NewLifestyleService(bodyState)
	expected := int64(4)

	if _, err := svc.AcceptCandidate(context.Background(), uuid.New(), &expected, candidateID); !errors.Is(err, ErrInvalidLifestyleCandidate) {
		t.Fatalf("accept non-lifestyle candidate error=%v, want ErrInvalidLifestyleCandidate", err)
	}
	if _, err := svc.RejectCandidate(context.Background(), uuid.New(), &expected, candidateID); !errors.Is(err, ErrInvalidLifestyleCandidate) {
		t.Fatalf("reject non-lifestyle candidate error=%v, want ErrInvalidLifestyleCandidate", err)
	}
	if len(bodyState.acceptedIDs) != 0 || len(bodyState.rejectedIDs) != 0 {
		t.Fatalf("non-lifestyle candidate reached mutation boundary: accepted=%v rejected=%v", bodyState.acceptedIDs, bodyState.rejectedIDs)
	}
}
