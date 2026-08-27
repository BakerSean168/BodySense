package service

import (
	"context"
	"testing"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type fakeBodyMetricsBodyState struct {
	snapshot BodyStateSnapshot
	patches  []model.BodyStateCurrentContextPatch
}

func (f *fakeBodyMetricsBodyState) GetSnapshot(context.Context, uuid.UUID, int) (*BodyStateSnapshot, error) {
	copy := f.snapshot
	return &copy, nil
}

func (f *fakeBodyMetricsBodyState) ApplyCurrentContextPatch(_ context.Context, _ uuid.UUID, _ *int64, patch model.BodyStateCurrentContextPatch, _ string) (*model.BodyStateRevision, error) {
	f.patches = append(f.patches, patch)
	f.snapshot.CurrentRevision++
	return &model.BodyStateRevision{Revision: f.snapshot.CurrentRevision}, nil
}

func TestBodyMetricsUpdateBatchesHeightAndWeightIntoOneBodyStateMutation(t *testing.T) {
	bodyState := &fakeBodyMetricsBodyState{snapshot: BodyStateSnapshot{CurrentRevision: 4}}
	svc := NewBodyMetricsService(bodyState)
	height := 178.5
	weight := 75.0
	revision := int64(4)
	_, err := svc.Update(context.Background(), uuid.New(), dto.UpdateBodyMetricsRequest{
		ExpectedRevision: &revision,
		HeightCm:         &height,
		WeightKg:         &weight,
	})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if len(bodyState.patches) != 1 {
		t.Fatalf("one measurement save must create one BodyState patch, got %d", len(bodyState.patches))
	}
	patch := bodyState.patches[0]
	if len(patch.Observations) != 2 {
		t.Fatalf("patch observations = %d, want 2", len(patch.Observations))
	}
	if patch.Observations[0].Kind != model.BodyStateObservationKindHeight || patch.Observations[1].Kind != model.BodyStateObservationKindWeight {
		t.Fatalf("unexpected metric patch kinds: %#v", patch.Observations)
	}
}

func TestBodyMetricsRejectsOutOfRangeValuesBeforeMutation(t *testing.T) {
	bodyState := &fakeBodyMetricsBodyState{}
	svc := NewBodyMetricsService(bodyState)
	height := 300.0
	_, err := svc.Update(context.Background(), uuid.New(), dto.UpdateBodyMetricsRequest{HeightCm: &height})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if len(bodyState.patches) != 0 {
		t.Fatal("invalid measurement must not reach BodyState")
	}
}
