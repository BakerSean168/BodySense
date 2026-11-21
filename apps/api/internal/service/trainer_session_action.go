package service

import (
	"context"

	"github.com/bodysense/api/internal/domain/safety"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// RecordSetAction appends one user-originated receipt at the next session
// revision after the TrainerGate admits that exercise. Only an in-progress
// session accepts actions; a paused one must be resumed first.
func (s *TrainerSessionService) RecordSetAction(ctx context.Context, owner uuid.UUID, cmd TrainerSetActionCommand) (*model.TrainingSetAction, error) {
	if owner == uuid.Nil || !cmd.valid() {
		return nil, ErrTrainerInvalid
	}
	fingerprint := planDigest(struct {
		Operation string
		Command   TrainerSetActionCommand
	}{"trainer_set_action", cmd})
	var result *model.TrainingSetAction
	err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := s.lock(ctx, owner); err != nil {
			return err
		}
		existing, err := s.repo.ActionByKey(ctx, owner, cmd.IdempotencyKey)
		if err != nil {
			return err
		}
		if existing != nil {
			if existing.RequestFingerprint != fingerprint {
				return ErrTrainerConflict
			}
			result = existing
			return nil
		}
		session, err := s.repo.Session(ctx, owner, cmd.SessionID, true)
		if err != nil {
			return trainerError(err)
		}
		if err := trainerLiveState(session, TrainerSessionInProgress); err != nil {
			return err
		}
		if session.Revision != cmd.ExpectedSessionRevision {
			return ErrTrainerConflict
		}
		template, err := s.pinnedTemplate(ctx, owner, session.PlanID, session.PlanRevisionID, session.TemplateID)
		if err != nil {
			return err
		}
		var exercise *model.PlanExercise
		for _, stage := range template.Stages {
			for i := range stage.Exercises {
				if stage.Exercises[i].PlanExerciseID == cmd.PlanExerciseID {
					exercise = &stage.Exercises[i]
				}
			}
		}
		if exercise == nil || cmd.SetNumber > exercise.Sets {
			return ErrTrainerInvalid
		}
		prior, err := s.repo.ExerciseActions(ctx, owner, session.ID, cmd.PlanExerciseID)
		if err != nil {
			return err
		}
		if err := checkTrainerTarget(prior, cmd, exercise.Sets); err != nil {
			return err
		}
		action := safety.Action{Kind: safety.ActionPerformSet, PlanID: session.PlanID, Exercises: []safety.Exercise{gateExercise(*exercise)}}
		decision, err := s.admit(ctx, owner, cmd.ExpectedSafetyRevision, action)
		if err != nil {
			return err
		}
		row := model.TrainingSetAction{
			ID: uuid.New(), UserID: owner, SessionID: session.ID, PlanExerciseID: exercise.PlanExerciseID, ExerciseID: exercise.ExerciseID,
			Scope: cmd.Scope, Kind: cmd.Kind, ActualReps: cmd.ActualReps, ActualDurationSeconds: cmd.ActualDurationSeconds,
			SessionRevision: session.Revision + 1, IdempotencyKey: cmd.IdempotencyKey, RequestFingerprint: fingerprint,
			SafetyRevision: decision.SafetyRevision, GateResult: string(decision.Result), GatePolicyRevision: decision.PolicyRevision, RecordedAt: s.now(),
		}
		if cmd.Scope == TrainerScopeSet {
			number := cmd.SetNumber
			row.SetNumber = &number
		}
		if cmd.Kind == TrainerSetCorrected {
			target := cmd.CorrectsActionID
			row.CorrectsActionID = &target
		}
		if err := s.repo.AppendAction(ctx, &row); err != nil {
			return trainerError(err)
		}
		result = &row
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
