package service

import (
	"context"

	"github.com/bodysense/api/internal/domain/safety"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// Start opens one session bound to the plan's current accepted revision.
func (s *TrainerSessionService) Start(ctx context.Context, owner uuid.UUID, cmd StartTrainerSessionCommand) (*TrainerSessionView, error) {
	zone, ok := TrainerTimeZone(cmd.TimeZone)
	if owner == uuid.Nil || cmd.PlanID == uuid.Nil || cmd.ExpectedPlanRevisionID == uuid.Nil || !validTrainerText(cmd.TemplateID) ||
		!ok || cmd.ExpectedSafetyRevision <= 0 || !validPlanKey(cmd.IdempotencyKey) {
		return nil, ErrTrainerInvalid
	}
	fingerprint := planDigest(struct {
		Operation string
		Command   StartTrainerSessionCommand
	}{"trainer_start", cmd})
	var result *TrainerSessionView
	err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := s.lock(ctx, owner); err != nil {
			return err
		}
		existing, err := s.repo.SessionByStartKey(ctx, owner, cmd.IdempotencyKey)
		if err != nil {
			return err
		}
		if existing != nil {
			if existing.StartRequestFingerprint != fingerprint {
				return ErrTrainerConflict
			}
			result, err = s.view(ctx, owner, *existing)
			return err
		}
		plan, err := s.plans.Plan(ctx, owner, cmd.PlanID)
		if err != nil {
			return trainerError(err)
		}
		if plan.Status != "active" {
			return ErrTrainerPlanNotActive
		}
		if *plan.CurrentAcceptedRevisionID != cmd.ExpectedPlanRevisionID {
			return ErrTrainerConflict
		}
		template, err := s.pinnedTemplate(ctx, owner, plan.ID, cmd.ExpectedPlanRevisionID, cmd.TemplateID)
		if err != nil {
			return err
		}
		decision, err := s.admit(ctx, owner, cmd.ExpectedSafetyRevision, templateGateAction(safety.ActionStartSession, plan.ID, template))
		if err != nil {
			return err
		}
		now := s.now()
		session := model.TrainingSession{
			ID: uuid.New(), UserID: owner, PlanID: plan.ID, PlanRevisionID: cmd.ExpectedPlanRevisionID, TemplateID: cmd.TemplateID,
			TimeZone: cmd.TimeZone, TrainingDate: TrainingDate(now, zone), State: TrainerSessionInProgress, StartedAt: now,
			StartIdempotencyKey: cmd.IdempotencyKey, StartRequestFingerprint: fingerprint, StartSafetyRevision: decision.SafetyRevision,
			StartGateResult: string(decision.Result), StartGatePolicyRevision: decision.PolicyRevision,
		}
		if err := s.repo.InsertSession(ctx, &session); err != nil {
			return trainerError(err)
		}
		result = &TrainerSessionView{Session: session, Actions: []model.TrainingSetAction{}, Transitions: []model.TrainingSessionTransition{}}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Pause moves an in-progress session to paused. Pausing is never gated and
// leaves every receipt as it is; it never marks a set or ends the session.
func (s *TrainerSessionService) Pause(ctx context.Context, owner uuid.UUID, cmd TrainerTransitionCommand) (*TrainerSessionView, error) {
	return s.transition(ctx, owner, TrainerTransitionPause, cmd)
}

// Resume moves a paused session back to in progress after re-running the
// TrainerGate over the template against exactly ExpectedSafetyRevision, as a
// start does. The plan must still be active; another session in progress
// blocks it.
func (s *TrainerSessionService) Resume(ctx context.Context, owner uuid.UUID, cmd TrainerTransitionCommand) (*TrainerSessionView, error) {
	return s.transition(ctx, owner, TrainerTransitionResume, cmd)
}

func (s *TrainerSessionService) transition(ctx context.Context, owner uuid.UUID, kind string, cmd TrainerTransitionCommand) (*TrainerSessionView, error) {
	resume := kind == TrainerTransitionResume
	if owner == uuid.Nil || cmd.SessionID == uuid.Nil || cmd.ExpectedSessionRevision < 0 || cmd.ExpectedSafetyRevision < 0 ||
		resume != (cmd.ExpectedSafetyRevision > 0) || !validPlanKey(cmd.IdempotencyKey) {
		return nil, ErrTrainerInvalid
	}
	from, to := TrainerSessionInProgress, TrainerSessionPaused
	if resume {
		from, to = TrainerSessionPaused, TrainerSessionInProgress
	}
	fingerprint := planDigest(struct {
		Operation string
		Command   TrainerTransitionCommand
	}{"trainer_" + kind, cmd})
	var result *TrainerSessionView
	err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := s.lock(ctx, owner); err != nil {
			return err
		}
		existing, err := s.repo.TransitionByKey(ctx, owner, cmd.IdempotencyKey)
		if err != nil {
			return err
		}
		if existing != nil {
			if existing.RequestFingerprint != fingerprint {
				return ErrTrainerConflict
			}
			session, err := s.repo.Session(ctx, owner, existing.SessionID, false)
			if err != nil {
				return trainerError(err)
			}
			result, err = s.view(ctx, owner, *session)
			return err
		}
		session, err := s.repo.Session(ctx, owner, cmd.SessionID, true)
		if err != nil {
			return trainerError(err)
		}
		if err := trainerLiveState(session, from); err != nil {
			return err
		}
		if session.Revision != cmd.ExpectedSessionRevision {
			return ErrTrainerConflict
		}
		row := model.TrainingSessionTransition{
			ID: uuid.New(), UserID: owner, SessionID: session.ID, Kind: kind, SessionRevision: session.Revision + 1,
			IdempotencyKey: cmd.IdempotencyKey, RequestFingerprint: fingerprint, RecordedAt: s.now(),
		}
		if resume {
			plan, err := s.plans.Plan(ctx, owner, session.PlanID)
			if err != nil {
				return trainerError(err)
			}
			if plan.Status != "active" {
				return ErrTrainerPlanNotActive
			}
			template, err := s.pinnedTemplate(ctx, owner, session.PlanID, session.PlanRevisionID, session.TemplateID)
			if err != nil {
				return err
			}
			decision, err := s.admit(ctx, owner, cmd.ExpectedSafetyRevision, templateGateAction(safety.ActionResumeSession, session.PlanID, template))
			if err != nil {
				return err
			}
			gate, policy := string(decision.Result), decision.PolicyRevision
			row.SafetyRevision, row.GateResult, row.GatePolicyRevision = &decision.SafetyRevision, &gate, &policy
		}
		if err := s.repo.Transition(ctx, &row, from, to); err != nil {
			return trainerError(err)
		}
		session.State, session.Revision = to, row.SessionRevision
		result, err = s.view(ctx, owner, *session)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Finish ends an in-progress or paused session once, as finished or stopped
// early. Completion is derived from the effective receipts; nothing is
// backfilled.
func (s *TrainerSessionService) Finish(ctx context.Context, owner uuid.UUID, cmd FinishTrainerSessionCommand) (*TrainerSessionView, error) {
	if owner == uuid.Nil || cmd.SessionID == uuid.Nil || (cmd.EndedReason != TrainerEndedFinished && cmd.EndedReason != TrainerEndedStoppedEarly) ||
		cmd.ExpectedSessionRevision < 0 || !validPlanKey(cmd.IdempotencyKey) {
		return nil, ErrTrainerInvalid
	}
	fingerprint := planDigest(struct {
		Operation string
		Command   FinishTrainerSessionCommand
	}{"trainer_finish", cmd})
	var result *TrainerSessionView
	err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := s.lock(ctx, owner); err != nil {
			return err
		}
		existing, err := s.repo.SessionByFinishKey(ctx, owner, cmd.IdempotencyKey)
		if err != nil {
			return err
		}
		if existing != nil {
			if *existing.FinishRequestFingerprint != fingerprint {
				return ErrTrainerConflict
			}
			result, err = s.view(ctx, owner, *existing)
			return err
		}
		session, err := s.repo.Session(ctx, owner, cmd.SessionID, true)
		if err != nil {
			return trainerError(err)
		}
		if session.State == TrainerSessionEnded {
			return ErrTrainerSessionEnded
		}
		if session.Revision != cmd.ExpectedSessionRevision {
			return ErrTrainerConflict
		}
		now := s.now()
		reason, key, digest := cmd.EndedReason, cmd.IdempotencyKey, fingerprint
		session.State, session.EndedReason, session.EndedAt, session.Revision = TrainerSessionEnded, &reason, &now, session.Revision+1
		session.FinishIdempotencyKey, session.FinishRequestFingerprint = &key, &digest
		if err := s.repo.End(ctx, session, cmd.ExpectedSessionRevision); err != nil {
			return trainerError(err)
		}
		result, err = s.view(ctx, owner, *session)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
