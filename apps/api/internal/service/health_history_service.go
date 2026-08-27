package service

import (
	"context"
	"time"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type healthHistoryBodyState interface {
	GetSnapshot(ctx context.Context, userID uuid.UUID, historyLimit int) (*BodyStateSnapshot, error)
	ApplyCurrentContextPatch(ctx context.Context, userID uuid.UUID, expectedRevision *int64, patch model.BodyStateCurrentContextPatch, source string) (*model.BodyStateRevision, error)
}

// HealthHistoryService exposes durable health-history projections while
// retaining BodyState as the sole source of truth.
type HealthHistoryService struct {
	bodyState healthHistoryBodyState
}

func NewHealthHistoryService(bodyState healthHistoryBodyState) *HealthHistoryService {
	return &HealthHistoryService{bodyState: bodyState}
}

func (s *HealthHistoryService) GetInjuryHistory(ctx context.Context, userID uuid.UUID) (*dto.InjuryHistorySnapshot, error) {
	snapshot, err := s.bodyState.GetSnapshot(ctx, userID, 0)
	if err != nil {
		return nil, err
	}
	result := &dto.InjuryHistorySnapshot{CurrentRevision: snapshot.CurrentRevision}
	for index := range snapshot.Facts {
		fact := snapshot.Facts[index]
		if fact.Kind != model.BodyStateFactKindInjuryHistory || fact.ReviewState != "confirmed" || fact.LifecycleState != "active" || fact.ExcludedFromReasoning {
			continue
		}
		updatedAt := fact.UpdatedAt
		result.FactID = &fact.ID
		result.Summary = fact.Value
		result.ValidFrom = fact.ValidFrom
		result.UpdatedAt = &updatedAt
		break
	}
	return result, nil
}

func (s *HealthHistoryService) UpdateInjuryHistory(
	ctx context.Context,
	userID uuid.UUID,
	request dto.UpdateInjuryHistoryRequest,
) (*dto.InjuryHistorySnapshot, error) {
	mutation := injuryHistoryMutation(
		request.Summary,
		time.Now().UTC(),
		"user_edited",
		"health_history_editor",
	)
	if _, err := s.bodyState.ApplyCurrentContextPatch(
		ctx,
		userID,
		request.ExpectedRevision,
		model.BodyStateCurrentContextPatch{Facts: []model.BodyStateCurrentFactMutation{mutation}},
		"health_history_editor",
	); err != nil {
		return nil, err
	}
	return s.GetInjuryHistory(ctx, userID)
}
