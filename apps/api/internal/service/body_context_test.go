package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
)

func TestBodyMetricsRecordsNewMeasurementsAndProjectsLatest(t *testing.T) {
	store := &memoryBodyContextStore{}
	svc := NewBodyMetricsService(store)
	owner := uuid.New()
	height, weight := 178.5, 75.0
	got, err := svc.Update(context.Background(), owner, UpdateBodyMetricsRequest{HeightCm: &height, WeightKg: &weight})
	if err != nil {
		t.Fatal(err)
	}
	if got.Height == nil || got.Height.Value != 178.5 || got.Height.Unit != "cm" || got.Weight == nil || got.Weight.Value != 75 || got.BMI == nil {
		t.Fatalf("unexpected metrics: %#v", got)
	}
	newWeight := 73.2
	revision := got.CurrentRevision
	got, err = svc.Update(context.Background(), owner, UpdateBodyMetricsRequest{ExpectedRevision: &revision, WeightKg: &newWeight, HeightCm: &height})
	if err != nil {
		t.Fatal(err)
	}
	if got.Weight.Value != 73.2 || got.Height.Value != 178.5 {
		t.Fatalf("latest measurement not projected: %#v", got)
	}
	if len(store.ledger.observations) != 3 {
		t.Fatalf("unchanged height must not be re-recorded; history must be kept: %d", len(store.ledger.observations))
	}
	stale := int64(0)
	if _, err := svc.Update(context.Background(), owner, UpdateBodyMetricsRequest{ExpectedRevision: &stale, WeightKg: &weight}); !errors.Is(err, repository.ErrBodyStateRevisionConflict) {
		t.Fatalf("stale revision must conflict: %v", err)
	}
}

func TestBodyMetricsRejectsOutOfRangeValuesBeforeMutation(t *testing.T) {
	store := &memoryBodyContextStore{}
	height := 300.0
	if _, err := NewBodyMetricsService(store).Update(context.Background(), uuid.New(), UpdateBodyMetricsRequest{HeightCm: &height}); err == nil {
		t.Fatal("expected validation error")
	}
	if len(store.updates) != 0 {
		t.Fatal("invalid measurement must not reach BodyState")
	}
}

func TestInjuryHistoryRecordsCorrectsAndClearsByWithdrawal(t *testing.T) {
	store := &memoryBodyContextStore{}
	svc := NewHealthHistoryService(store)
	owner := uuid.New()
	got, err := svc.UpdateInjuryHistory(context.Background(), owner, UpdateInjuryHistoryRequest{Summary: "2024 年左膝拉伤"})
	if err != nil || got.Summary != "2024 年左膝拉伤" || got.FactID == nil {
		t.Fatalf("record injury history: %#v %v", got, err)
	}
	got, err = svc.UpdateInjuryHistory(context.Background(), owner, UpdateInjuryHistoryRequest{Summary: "  "})
	if err != nil || got.Summary != "" || got.FactID != nil {
		t.Fatalf("clear must withdraw the current statement: %#v %v", got, err)
	}
	if len(store.ledger.facts) != 1 || store.ledger.facts[0].AssertionStatus != "withdrawn" {
		t.Fatalf("withdrawal keeps history instead of deleting: %#v", store.ledger.facts)
	}
}

func TestLifestyleUpdateIsPatchLikeAndKeepsHistory(t *testing.T) {
	store := &memoryBodyContextStore{}
	svc := NewLifestyleService(store)
	owner := uuid.New()
	got, err := svc.Update(context.Background(), owner, UpdateLifestyleRequest{
		Sleep:    &LifestyleSectionInput{Summary: "平均睡 6 小时"},
		Exercise: &LifestyleSectionInput{Summary: "每周力量训练 3 次", Details: []byte(`{"frequency":"3/week","unknown":"dropped"}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Sleep.Summary != "平均睡 6 小时" || got.Exercise.Summary != "每周力量训练 3 次" || got.Activity.Summary != "" {
		t.Fatalf("unexpected lifestyle: %#v", got)
	}
	if string(got.Exercise.Details) != `{"habit_type":"exercise","frequency":"3/week"}` {
		t.Fatalf("details must keep only approved lifestyle fields: %s", got.Exercise.Details)
	}
	got, err = svc.Update(context.Background(), owner, UpdateLifestyleRequest{Sleep: &LifestyleSectionInput{Summary: "平均睡 7 小时"}})
	if err != nil || got.Sleep.Summary != "平均睡 7 小时" || got.Exercise.Summary != "每周力量训练 3 次" {
		t.Fatalf("patch must change only sleep: %#v %v", got, err)
	}
	if len(store.ledger.facts) != 3 || store.ledger.facts[0].AssertionStatus != "asserted" {
		t.Fatalf("a real habit change appends history: %#v", store.ledger.facts)
	}
}
