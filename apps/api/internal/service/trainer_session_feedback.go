package service

import (
	"context"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// RecordFeedback stores the user's verbatim feedback on one ended session,
// once. It is never gated (the user can always say how a session went) and
// only feeds the plan review projection: no plan, revision, receipt, BodyState
// or Safety row is written. Body symptoms are recorded through BodyState.
func (s *TrainerSessionService) RecordFeedback(ctx context.Context, owner uuid.UUID, cmd TrainerFeedbackCommand) (*TrainerSessionView, error) {
	if owner == uuid.Nil || cmd.SessionID == uuid.Nil || !validTrainerFeedback(cmd.Text) || !validPlanKey(cmd.IdempotencyKey) {
		return nil, ErrTrainerInvalid
	}
	fingerprint := planDigest(struct {
		Operation string
		Command   TrainerFeedbackCommand
	}{"trainer_feedback", cmd})
	var result *TrainerSessionView
	err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := s.lock(ctx, owner); err != nil {
			return err
		}
		existing, err := s.repo.FeedbackByKey(ctx, owner, cmd.IdempotencyKey)
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
		if session.State != TrainerSessionEnded {
			return ErrTrainerSessionNotEnded
		}
		prior, err := s.repo.Feedback(ctx, owner, []uuid.UUID{session.ID})
		if err != nil {
			return err
		}
		if len(prior) > 0 {
			return ErrTrainerFeedbackRecorded
		}
		row := model.TrainingSessionFeedback{
			ID: uuid.New(), UserID: owner, SessionID: session.ID, PlanID: session.PlanID, PlanRevisionID: session.PlanRevisionID,
			Text: cmd.Text, IdempotencyKey: cmd.IdempotencyKey, RequestFingerprint: fingerprint, RecordedAt: s.now(),
		}
		if err := s.repo.InsertFeedback(ctx, &row); err != nil {
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
