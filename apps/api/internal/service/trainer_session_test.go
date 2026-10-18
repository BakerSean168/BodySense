package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// The training date is the user's IANA local date at start, not the server
// or UTC date; the same instant falls on different days in different zones.
func TestTrainingDateUsesTimeZoneSnapshot(t *testing.T) {
	instant := time.Date(2026, 10, 13, 16, 30, 0, 0, time.UTC)
	for zone, want := range map[string]string{
		"Asia/Shanghai":       "2026-10-14",
		"UTC":                 "2026-10-13",
		"America/Los_Angeles": "2026-10-13",
		"Pacific/Kiritimati":  "2026-10-14",
	} {
		loc, ok := TrainerTimeZone(zone)
		if !ok {
			t.Fatalf("zone %s rejected", zone)
		}
		if got := TrainingDate(instant, loc).String(); got != want {
			t.Fatalf("%s: got %s want %s", zone, got, want)
		}
	}
	for _, zone := range []string{"", "Local", "Nowhere/City", "Asia/Shanghai "} {
		if _, ok := TrainerTimeZone(zone); ok {
			t.Fatalf("zone %q accepted", zone)
		}
	}
}

// Malformed commands are rejected before any database work.
func TestTrainerCommandsValidateBeforeStorage(t *testing.T) {
	svc := &TrainerSessionService{}
	ctx, owner := context.Background(), uuid.New()
	if _, err := svc.Start(ctx, owner, StartTrainerSessionCommand{PlanID: uuid.New(), ExpectedPlanRevisionID: uuid.New(), TemplateID: "t", TimeZone: "UTC", IdempotencyKey: "k"}); !errors.Is(err, ErrTrainerInvalid) {
		t.Fatalf("start without safety revision: %v", err)
	}
	for _, kind := range []string{"", "completed", "corrected"} {
		cmd := TrainerSetActionCommand{SessionID: uuid.New(), PlanExerciseID: "e", SetNumber: 1, Kind: kind, ExpectedSafetyRevision: 1, IdempotencyKey: "k"}
		if _, err := svc.RecordSetAction(ctx, owner, cmd); !errors.Is(err, ErrTrainerInvalid) {
			t.Fatalf("set kind %q: %v", kind, err)
		}
	}
	if _, err := svc.RecordSetAction(ctx, owner, TrainerSetActionCommand{SessionID: uuid.New(), PlanExerciseID: "e", SetNumber: 0, Kind: TrainerSetPerformed, ExpectedSafetyRevision: 1, IdempotencyKey: "k"}); !errors.Is(err, ErrTrainerInvalid) {
		t.Fatalf("set number 0: %v", err)
	}
	if _, err := svc.Finish(ctx, owner, FinishTrainerSessionCommand{SessionID: uuid.New(), EndedReason: "abandoned", IdempotencyKey: "k"}); !errors.Is(err, ErrTrainerInvalid) {
		t.Fatalf("finish reason: %v", err)
	}
	if _, err := svc.List(ctx, owner, "paused"); !errors.Is(err, ErrTrainerInvalid) {
		t.Fatalf("list state: %v", err)
	}
}
