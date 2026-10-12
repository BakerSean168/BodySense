package service

import (
	"context"
	"fmt"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type bodyFactLedger interface {
	RecordDirectFacts(context.Context, uuid.UUID, model.RecordBodyFactsCommand) (*model.BodyFactCommit, error)
	ListFacts(context.Context, uuid.UUID, int64) (*model.BodyFactPage, error)
}

// BodyFactService is the replacement assertion boundary. Legacy observations,
// episodes and consumers migrate in subsequent slices, without dual writing.
type BodyFactService struct {
	ledger  bodyFactLedger
	regions BodyRegionIDValidator
}

func NewBodyFactService(ledger bodyFactLedger, regions BodyRegionIDValidator) *BodyFactService {
	return &BodyFactService{ledger: ledger, regions: regions}
}

func (s *BodyFactService) RecordDirectFacts(ctx context.Context, userID uuid.UUID, command model.RecordBodyFactsCommand) (*model.BodyFactCommit, error) {
	if userID == uuid.Nil {
		return nil, fmt.Errorf("%w: authenticated owner required", model.ErrInvalidBodyFact)
	}
	if err := command.Validate(); err != nil {
		return nil, err
	}
	for _, fact := range command.Facts {
		for _, region := range fact.BodyRegionIDs {
			if s.regions == nil {
				return nil, ErrBodyRegionIDValidationUnavailable
			}
			if !s.regions.IsValidBodyRegionID(region) {
				return nil, ErrUnknownBodyRegionID
			}
		}
	}
	return s.ledger.RecordDirectFacts(ctx, userID, command)
}

func (s *BodyFactService) ListFacts(ctx context.Context, userID uuid.UUID, beforeRevision int64) (*model.BodyFactPage, error) {
	if userID == uuid.Nil || beforeRevision < 0 {
		return nil, fmt.Errorf("%w: invalid owner or cursor", model.ErrInvalidBodyFact)
	}
	return s.ledger.ListFacts(ctx, userID, beforeRevision)
}
