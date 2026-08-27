package service

import (
	"context"
	"testing"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type fakeHealthHistoryBodyState struct {
	snapshot BodyStateSnapshot
	patches  []model.BodyStateCurrentContextPatch
}

func (f *fakeHealthHistoryBodyState) GetSnapshot(context.Context, uuid.UUID, int) (*BodyStateSnapshot, error) {
	copy := f.snapshot
	return &copy, nil
}

func (f *fakeHealthHistoryBodyState) ApplyCurrentContextPatch(_ context.Context, _ uuid.UUID, _ *int64, patch model.BodyStateCurrentContextPatch, _ string) (*model.BodyStateRevision, error) {
	f.patches = append(f.patches, patch)
	return &model.BodyStateRevision{Revision: f.snapshot.CurrentRevision + 1}, nil
}

func TestHealthHistoryProjectionIgnoresUnverifiedInjuryCandidate(t *testing.T) {
	bodyState := &fakeHealthHistoryBodyState{snapshot: BodyStateSnapshot{
		CurrentRevision: 4,
		Facts: []model.BodyStateFact{
			{ID: uuid.New(), Kind: model.BodyStateFactKindInjuryHistory, Value: "AI 候选旧伤", ReviewState: "unverified", LifecycleState: "active"},
			{ID: uuid.New(), Kind: model.BodyStateFactKindInjuryHistory, Value: "用户确认的左膝旧伤", ReviewState: "confirmed", LifecycleState: "active"},
		},
	}}
	svc := NewHealthHistoryService(bodyState)
	result, err := svc.GetInjuryHistory(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("GetInjuryHistory returned error: %v", err)
	}
	if result.Summary != "用户确认的左膝旧伤" {
		t.Fatalf("summary=%q, want confirmed fact", result.Summary)
	}
}

func TestHealthHistoryUpdateUsesOneBodyStateMutation(t *testing.T) {
	bodyState := &fakeHealthHistoryBodyState{snapshot: BodyStateSnapshot{CurrentRevision: 8}}
	svc := NewHealthHistoryService(bodyState)
	revision := int64(8)
	_, err := svc.UpdateInjuryHistory(context.Background(), uuid.New(), dto.UpdateInjuryHistoryRequest{
		ExpectedRevision: &revision,
		Summary:          "2024 年左膝拉伤",
	})
	if err != nil {
		t.Fatalf("UpdateInjuryHistory returned error: %v", err)
	}
	if len(bodyState.patches) != 1 || len(bodyState.patches[0].Facts) != 1 {
		t.Fatalf("expected one health-history patch: %#v", bodyState.patches)
	}
	mutation := bodyState.patches[0].Facts[0]
	if mutation.Kind != model.BodyStateFactKindInjuryHistory || mutation.Replacement == nil || mutation.Replacement.Value != "2024 年左膝拉伤" {
		t.Fatalf("unexpected injury mutation: %#v", mutation)
	}
}
