package service

import (
	"context"
	"errors"
	"strings"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type outcomeRepository interface {
	CreateOutcome(ctx context.Context, outcome *model.Outcome) (*model.Outcome, bool, error)
	UpdateOutcomeBodyStateRevision(ctx context.Context, outcomeID, userID uuid.UUID, revision int64) error
	ListOutcomes(ctx context.Context, userID uuid.UUID, limit int) ([]model.Outcome, error)
}

type outcomeBodyStateSource interface {
	RecordOutcomeSymptom(ctx context.Context, userID uuid.UUID, outcome model.Outcome) (*model.BodyStateRevision, error)
}

type outcomeUnitOfWork interface {
	WithinTransaction(ctx context.Context, fn func(context.Context) error) error
}

// OutcomeService records the user's longitudinal Outcomes. An Outcome is
// feedback, never a plan change: it is linked to no plan or revision.
type OutcomeService struct {
	repo       outcomeRepository
	bodyState  outcomeBodyStateSource
	unitOfWork outcomeUnitOfWork
}

func NewOutcomeService(repo outcomeRepository, bodyState outcomeBodyStateSource, unitOfWork outcomeUnitOfWork) *OutcomeService {
	return &OutcomeService{repo: repo, bodyState: bodyState, unitOfWork: unitOfWork}
}

func (s *OutcomeService) ListOutcomes(ctx context.Context, userID uuid.UUID, limit int) ([]model.Outcome, error) {
	return s.repo.ListOutcomes(ctx, userID, limit)
}

// RecordOutcome closes the feedback loop with idempotent source identity. The
// Outcome is persisted first; only a user-authored symptom description becomes
// a V2 symptom BodyFact (linked via body_state_revision). Other outcomes write
// no BodyState and leave body_state_revision empty.
func (s *OutcomeService) RecordOutcome(ctx context.Context, userID uuid.UUID, outcome model.Outcome) (*model.Outcome, bool, error) {
	outcome.UserID = userID
	if outcome.SourceType == "" || strings.TrimSpace(outcome.SourceKey) == "" || outcome.Kind == "" {
		return nil, false, errors.New("outcome source_type, source_key and kind are required")
	}
	if outcome.CausalityLevel == "" {
		outcome.CausalityLevel = "association_only"
	}
	if outcome.CausalityLevel == "association_only" && outcome.AssociationStatement == "" {
		outcome.AssociationStatement = "该变化发生在干预之后，表示时间关联，不代表已经证明因果关系。"
	}
	if s.unitOfWork == nil || s.bodyState == nil {
		return nil, false, errors.New("outcome service is not configured")
	}
	var stored *model.Outcome
	var created bool
	err := s.unitOfWork.WithinTransaction(ctx, func(txCtx context.Context) error {
		var createErr error
		stored, created, createErr = s.repo.CreateOutcome(txCtx, &outcome)
		if createErr != nil {
			return createErr
		}
		if stored == nil {
			return errors.New("outcome repository returned no record")
		}
		if !created && stored.BodyStateRevision != nil {
			return nil
		}
		revision, projectionErr := s.bodyState.RecordOutcomeSymptom(txCtx, userID, *stored)
		if projectionErr != nil {
			return projectionErr
		}
		if revision == nil {
			return nil
		}
		if revision.Revision <= 0 {
			return errors.New("outcome symptom fact has no BodyState revision")
		}
		stored.BodyStateRevision = &revision.Revision
		return s.repo.UpdateOutcomeBodyStateRevision(txCtx, stored.ID, userID, revision.Revision)
	})
	if err != nil {
		return nil, false, err
	}
	return stored, created, nil
}
