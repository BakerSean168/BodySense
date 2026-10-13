package service

import (
	"context"
	"fmt"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// A separate BodyObservation aggregate boundary, not the legacy
// body_state_observations review/lifecycle or the old AI reasoning model.
type bodyObservationLedger interface {
	RecordDirectObservation(context.Context, uuid.UUID, model.RecordBodyObservationCommand) (*model.BodyObservationCommit, error)
	ListDirectObservations(context.Context, uuid.UUID, int64) (*model.BodyObservationPage, error)
}

type BodyObservationService struct{ ledger bodyObservationLedger }

func NewBodyObservationService(ledger bodyObservationLedger) *BodyObservationService {
	return &BodyObservationService{ledger: ledger}
}
func (s *BodyObservationService) RecordDirectObservation(ctx context.Context, owner uuid.UUID, cmd model.RecordBodyObservationCommand) (*model.BodyObservationCommit, error) {
	if owner == uuid.Nil {
		return nil, fmt.Errorf("%w: authenticated owner required", model.ErrInvalidBodyObservation)
	}
	if err := cmd.Validate(); err != nil {
		return nil, err
	}
	return s.ledger.RecordDirectObservation(ctx, owner, cmd)
}
func (s *BodyObservationService) ListDirectObservations(ctx context.Context, owner uuid.UUID, before int64) (*model.BodyObservationPage, error) {
	if owner == uuid.Nil || before < 0 {
		return nil, fmt.Errorf("%w: invalid owner or cursor", model.ErrInvalidBodyObservation)
	}
	return s.ledger.ListDirectObservations(ctx, owner, before)
}
