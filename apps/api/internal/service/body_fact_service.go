package service

import (
	"context"
	"fmt"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type bodyFactLedger interface {
	CorrectDirectFact(context.Context, uuid.UUID, uuid.UUID, model.AmendBodyFactCommand) (*model.BodyFactCommit, error)
	WithdrawDirectFact(context.Context, uuid.UUID, uuid.UUID, model.AmendBodyFactCommand) (*model.BodyFactCommit, error)
	RecordDirectFacts(context.Context, uuid.UUID, model.RecordBodyFactsCommand) (*model.BodyFactCommit, error)
	RecordRunFacts(context.Context, uuid.UUID, uuid.UUID, model.RecordBodyFactsCommand) (*model.BodyFactCommit, error)
	RecordMessageFacts(context.Context, uuid.UUID, model.MessageFactSource, model.RecordBodyFactsCommand) (*model.BodyFactCommit, error)
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

// RecordMessageFacts accepts a consultation run's message identity only after
// validating all fact kinds/regions. The repository independently verifies the
// original Go-owned envelope and exact user-authored excerpts.
func (s *BodyFactService) RecordMessageFacts(ctx context.Context, userID uuid.UUID, source model.MessageFactSource, command model.RecordBodyFactsCommand) (*model.BodyFactCommit, error) {
	if userID == uuid.Nil || source.RunID == uuid.Nil || source.MessageID == uuid.Nil {
		return nil, fmt.Errorf("%w: authenticated run and source message required", model.ErrInvalidBodyFact)
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
	return s.ledger.RecordMessageFacts(ctx, userID, source, command)
}

// RecordRunFacts binds an extraction proposal to a server-owned Run, never
// to a source identity supplied by the model. Repository rechecks in transaction.
func (s *BodyFactService) RecordRunFacts(ctx context.Context, userID, runID uuid.UUID, command model.RecordBodyFactsCommand) (*model.BodyFactCommit, error) {
	if userID == uuid.Nil || runID == uuid.Nil {
		return nil, fmt.Errorf("%w: authenticated run required", model.ErrInvalidBodyFact)
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
	return s.ledger.RecordRunFacts(ctx, userID, runID, command)
}

func (s *BodyFactService) ListFacts(ctx context.Context, userID uuid.UUID, beforeRevision int64) (*model.BodyFactPage, error) {
	if userID == uuid.Nil || beforeRevision < 0 {
		return nil, fmt.Errorf("%w: invalid owner or cursor", model.ErrInvalidBodyFact)
	}
	return s.ledger.ListFacts(ctx, userID, beforeRevision)
}

func (s *BodyFactService) CorrectDirectFact(ctx context.Context, owner, target uuid.UUID, command model.AmendBodyFactCommand) (*model.BodyFactCommit, error) {
	if owner == uuid.Nil || target == uuid.Nil {
		return nil, fmt.Errorf("%w: owner and exact target required", model.ErrInvalidBodyFact)
	}
	if err := command.Validate(true); err != nil {
		return nil, err
	}
	for _, region := range command.Replacement.BodyRegionIDs {
		if s.regions == nil {
			return nil, ErrBodyRegionIDValidationUnavailable
		}
		if !s.regions.IsValidBodyRegionID(region) {
			return nil, ErrUnknownBodyRegionID
		}
	}
	return s.ledger.CorrectDirectFact(ctx, owner, target, command)
}
func (s *BodyFactService) WithdrawDirectFact(ctx context.Context, owner, target uuid.UUID, command model.AmendBodyFactCommand) (*model.BodyFactCommit, error) {
	if owner == uuid.Nil || target == uuid.Nil {
		return nil, fmt.Errorf("%w: owner and exact target required", model.ErrInvalidBodyFact)
	}
	if err := command.Validate(false); err != nil {
		return nil, err
	}
	return s.ledger.WithdrawDirectFact(ctx, owner, target, command)
}

// RecordAnswerFacts accepts only the server resume envelope, never Agent-authored IDs.
func (s *BodyFactService) RecordAnswerFacts(ctx context.Context, owner uuid.UUID, source model.AnswerFactSource, command model.RecordBodyFactsCommand) (*model.BodyFactCommit, error) {
	if owner == uuid.Nil || source.RunID == uuid.Nil || source.InteractionID == uuid.Nil || source.MessageID == uuid.Nil {
		return nil, fmt.Errorf("%w: accepted answer required", model.ErrInvalidBodyFact)
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
	ledger, ok := s.ledger.(interface {
		RecordAnswerFacts(context.Context, uuid.UUID, model.AnswerFactSource, model.RecordBodyFactsCommand) (*model.BodyFactCommit, error)
	})
	if !ok {
		return nil, fmt.Errorf("%w: answer ledger unavailable", model.ErrInvalidBodyFact)
	}
	return ledger.RecordAnswerFacts(ctx, owner, source, command)
}
