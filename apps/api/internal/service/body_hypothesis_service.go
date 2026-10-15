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
	ListActiveBodyHypotheses(context.Context, uuid.UUID) (*model.BodyHypothesisList, error)
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

func (s *BodyHypothesisService) ListActiveBodyHypotheses(ctx context.Context, owner uuid.UUID) (*model.BodyHypothesisList, error) {
	if owner == uuid.Nil {
		return nil, fmt.Errorf("%w: authenticated owner required", model.ErrInvalidBodyHypothesis)
	}
	return s.ledger.ListActiveBodyHypotheses(ctx, owner)
}
