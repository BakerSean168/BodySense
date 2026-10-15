package service

import (
	"context"

	"github.com/google/uuid"
)

// HealthHistoryService projects the user's injury-history statement from the
// V2 BodyFact ledger (health_history, history_kind=injury_summary).
type HealthHistoryService struct {
	bodyState bodyContextStore
}

func NewHealthHistoryService(bodyState bodyContextStore) *HealthHistoryService {
	return &HealthHistoryService{bodyState: bodyState}
}

func (s *HealthHistoryService) GetInjuryHistory(ctx context.Context, userID uuid.UUID) (*InjuryHistorySnapshot, error) {
	current, err := s.bodyState.CurrentBodyContext(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := &InjuryHistorySnapshot{CurrentRevision: current.Revision}
	if fact := current.Injury; fact != nil {
		id := fact.ID
		recordedAt := fact.RecordedAt
		result.FactID = &id
		result.Summary = fact.Statement
		result.ValidFrom = bodyContextFactTemporalStart(fact)
		result.UpdatedAt = &recordedAt
	}
	return result, nil
}

func (s *HealthHistoryService) UpdateInjuryHistory(
	ctx context.Context,
	userID uuid.UUID,
	request UpdateInjuryHistoryRequest,
) (*InjuryHistorySnapshot, error) {
	summary := request.Summary
	if _, err := s.bodyState.UpdateBodyContext(ctx, userID, request.ExpectedRevision, BodyContextUpdate{
		Injury: &summary, Context: "health_history_editor",
	}); err != nil {
		return nil, err
	}
	return s.GetInjuryHistory(ctx, userID)
}
