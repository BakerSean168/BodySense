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
	valid := TrainerSetActionCommand{SessionID: uuid.New(), PlanExerciseID: "e", Scope: TrainerScopeSet, SetNumber: 1, Kind: TrainerSetPerformed, ExpectedSafetyRevision: 1, IdempotencyKey: "k"}
	if !valid.valid() {
		t.Fatal("valid command rejected")
	}
	zero, five, tooMany, tooLong := 0, 5, 1001, 86401
	invalid := map[string]func(*TrainerSetActionCommand){
		"empty kind":            func(c *TrainerSetActionCommand) { c.Kind = "" },
		"completed kind":        func(c *TrainerSetActionCommand) { c.Kind = "completed" },
		"correction w/o target": func(c *TrainerSetActionCommand) { c.Kind = TrainerSetCorrected },
		"target w/o correction": func(c *TrainerSetActionCommand) { c.CorrectsActionID = uuid.New() },
		"no scope":              func(c *TrainerSetActionCommand) { c.Scope = "" },
		"set number 0":          func(c *TrainerSetActionCommand) { c.SetNumber = 0 },
		"exercise with set":     func(c *TrainerSetActionCommand) { c.Scope, c.Kind = TrainerScopeExercise, TrainerSetSkipped },
		"exercise performed":    func(c *TrainerSetActionCommand) { c.Scope, c.SetNumber = TrainerScopeExercise, 0 },
		"zero reps":             func(c *TrainerSetActionCommand) { c.ActualReps = &zero },
		"too many reps":         func(c *TrainerSetActionCommand) { c.ActualReps = &tooMany },
		"too long":              func(c *TrainerSetActionCommand) { c.ActualDurationSeconds = &tooLong },
		"actuals on a skip":     func(c *TrainerSetActionCommand) { c.Kind, c.ActualReps = TrainerSetSkipped, &five },
	}
	for name, mutate := range invalid {
		cmd := valid
		mutate(&cmd)
		if _, err := svc.RecordSetAction(ctx, owner, cmd); !errors.Is(err, ErrTrainerInvalid) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if _, err := svc.Pause(ctx, owner, TrainerTransitionCommand{SessionID: uuid.New(), ExpectedSafetyRevision: 1, IdempotencyKey: "k"}); !errors.Is(err, ErrTrainerInvalid) {
		t.Fatalf("pause with safety revision: %v", err)
	}
	if _, err := svc.Resume(ctx, owner, TrainerTransitionCommand{SessionID: uuid.New(), IdempotencyKey: "k"}); !errors.Is(err, ErrTrainerInvalid) {
		t.Fatalf("resume without safety revision: %v", err)
	}
	if _, err := svc.Finish(ctx, owner, FinishTrainerSessionCommand{SessionID: uuid.New(), EndedReason: "abandoned", IdempotencyKey: "k"}); !errors.Is(err, ErrTrainerInvalid) {
		t.Fatalf("finish reason: %v", err)
	}
	if _, err := svc.List(ctx, owner, "stopped"); !errors.Is(err, ErrTrainerInvalid) {
		t.Fatalf("list state: %v", err)
	}
}
