package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/bodysense/api/internal/domain/safety"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// TrainerTimeZone accepts only a loadable IANA zone name. "Local" and empty
// names would silently follow the server clock and are rejected.
func TrainerTimeZone(name string) (*time.Location, bool) {
	if name == "" || len(name) > 64 || strings.TrimSpace(name) != name || name == "Local" {
		return nil, false
	}
	loc, err := time.LoadLocation(name)
	return loc, err == nil
}

// TrainingDate is the user's local calendar date of instant in zone; it is
// fixed at session start and never recomputed (TP-D6).
func TrainingDate(instant time.Time, zone *time.Location) model.DateOnly {
	local := instant.In(zone)
	return model.DateOnly(time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC))
}

// admit share-locks the owner's Safety head, requires it to be exactly the
// revision the user saw, and requires the Go TrainerGate to permit action at
// that revision. Unavailable or unscreened Safety never permits.
func (s *TrainerSessionService) admit(ctx context.Context, owner uuid.UUID, expected int64, action safety.Action) (safety.Decision, error) {
	current, err := s.repo.SafetyRevision(ctx, owner)
	if err != nil {
		return safety.Decision{}, err
	}
	if current != expected {
		return safety.Decision{}, ErrTrainerSafetyStale
	}
	decision := s.body.EvaluateTrainerGate(ctx, owner, action, s.now())
	if !decision.Permits() {
		return decision, &TrainerGateError{Decision: decision}
	}
	if decision.SafetyRevision != expected {
		return decision, ErrTrainerSafetyStale
	}
	return decision, nil
}

// pinnedTemplate reads the session template from one immutable revision.
func (s *TrainerSessionService) pinnedTemplate(ctx context.Context, owner, plan, revision uuid.UUID, templateID string) (*model.PlanSessionTemplate, error) {
	rev, err := s.plans.Revision(ctx, owner, plan, revision)
	if err != nil {
		return nil, trainerError(err)
	}
	var draft model.TrainingPlanDraft
	if err := json.Unmarshal(rev.Snapshot, &draft); err != nil {
		return nil, fmt.Errorf("training plan revision snapshot: %w", err)
	}
	for i := range draft.Content.Sessions {
		if draft.Content.Sessions[i].TemplateID == templateID {
			return &draft.Content.Sessions[i], nil
		}
	}
	return nil, ErrTrainerInvalid
}

func gateExercise(e model.PlanExercise) safety.Exercise {
	return safety.Exercise{ExerciseID: e.ExerciseID, Custom: strings.HasPrefix(e.ExerciseID, "custom:")}
}

// templateGateAction is a session-scope gate action over every exercise of
// the template (start and resume).
func templateGateAction(kind safety.ActionKind, plan uuid.UUID, template *model.PlanSessionTemplate) safety.Action {
	action := safety.Action{Kind: kind, PlanID: plan}
	for _, stage := range template.Stages {
		for _, exercise := range stage.Exercises {
			action.Exercises = append(action.Exercises, gateExercise(exercise))
		}
	}
	return action
}
