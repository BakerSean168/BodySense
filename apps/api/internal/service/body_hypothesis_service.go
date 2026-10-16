package service

import (
	"context"
	"fmt"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// bodyHypothesisLedger is the V2 user-authored BodyHypothesis boundary. It is
// NOT the legacy body_state_hypotheses AI reasoning model (BodyStateHypothesis),
// and AI/Diagnosis hypotheses never pass through it.
type bodyHypothesisLedger interface {
	RecordBodyHypothesis(context.Context, uuid.UUID, model.RecordBodyHypothesisCommand) (*model.BodyHypothesisCommit, error)
	WithdrawBodyHypothesis(context.Context, uuid.UUID, uuid.UUID, model.WithdrawBodyHypothesisCommand) (*model.BodyHypothesisCommit, error)
	CorrectBodyHypothesis(context.Context, uuid.UUID, uuid.UUID, model.CorrectBodyHypothesisCommand) (*model.BodyHypothesisCommit, error)
	ListActiveBodyHypotheses(context.Context, uuid.UUID) (*model.BodyHypothesisList, error)
	ReadConcernGroupSnapshot(context.Context, uuid.UUID) (*model.ConcernGroupSnapshot, error)
}

type BodyHypothesisService struct{ ledger bodyHypothesisLedger }

func NewBodyHypothesisService(ledger bodyHypothesisLedger) *BodyHypothesisService {
	return &BodyHypothesisService{ledger: ledger}
}

func (s *BodyHypothesisService) RecordBodyHypothesis(ctx context.Context, owner uuid.UUID, cmd model.RecordBodyHypothesisCommand) (*model.BodyHypothesisCommit, error) {
	if owner == uuid.Nil {
		return nil, fmt.Errorf("%w: authenticated owner required", model.ErrInvalidBodyHypothesis)
	}
	if err := cmd.Validate(); err != nil {
		return nil, err
	}
	return s.ledger.RecordBodyHypothesis(ctx, owner, cmd)
}

func (s *BodyHypothesisService) WithdrawBodyHypothesis(ctx context.Context, owner, target uuid.UUID, cmd model.WithdrawBodyHypothesisCommand) (*model.BodyHypothesisCommit, error) {
	if owner == uuid.Nil || target == uuid.Nil {
		return nil, fmt.Errorf("%w: owner and exact target required", model.ErrInvalidBodyHypothesis)
	}
	if err := cmd.Validate(); err != nil {
		return nil, err
	}
	return s.ledger.WithdrawBodyHypothesis(ctx, owner, target, cmd)
}

// CorrectBodyHypothesis supersedes exactly one of the user's own current
// guesses with a complete replacement in the same lineage (direct edit only).
func (s *BodyHypothesisService) CorrectBodyHypothesis(ctx context.Context, owner, target uuid.UUID, cmd model.CorrectBodyHypothesisCommand) (*model.BodyHypothesisCommit, error) {
	if owner == uuid.Nil || target == uuid.Nil {
		return nil, fmt.Errorf("%w: owner and exact target required", model.ErrInvalidBodyHypothesis)
	}
	if err := cmd.Validate(); err != nil {
		return nil, err
	}
	return s.ledger.CorrectBodyHypothesis(ctx, owner, target, cmd)
}

func (s *BodyHypothesisService) ListActiveBodyHypotheses(ctx context.Context, owner uuid.UUID) (*model.BodyHypothesisList, error) {
	if owner == uuid.Nil {
		return nil, fmt.Errorf("%w: authenticated owner required", model.ErrInvalidBodyHypothesis)
	}
	return s.ledger.ListActiveBodyHypotheses(ctx, owner)
}

// ListDerivedConcernGroups computes the read-only ConcernGroup organisation
// from one owner-scoped BodyState revision. Nothing is persisted and the
// result carries no clinical authority.
func (s *BodyHypothesisService) ListDerivedConcernGroups(ctx context.Context, owner uuid.UUID) (*model.DerivedConcernGroupList, error) {
	if owner == uuid.Nil {
		return nil, fmt.Errorf("%w: authenticated owner required", model.ErrInvalidBodyHypothesis)
	}
	snap, err := s.ledger.ReadConcernGroupSnapshot(ctx, owner)
	if err != nil {
		return nil, err
	}
	if snap == nil || snap.UserID != owner {
		return nil, fmt.Errorf("%w: concern group snapshot owner mismatch", model.ErrInvalidBodyHypothesis)
	}
	return model.DeriveConcernGroups(*snap), nil
}
