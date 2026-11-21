package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// InteractionMetrics is a lightweight projection over agent_interactions
// (T0-1 Phase C). Computed on read from the source table — no separate metrics table.
type InteractionMetrics struct {
	Total          int     `json:"total"`
	Answered       int     `json:"answered"`
	Expired        int     `json:"expired"`
	Pending        int     `json:"pending"`
	AnswerRate     float64 `json:"answer_rate"`
	ExpireRate     float64 `json:"expire_rate"`
	AvgWaitSeconds float64 `json:"avg_wait_seconds"`
}

// GetInteractionMetrics returns answer/expire rates and average wait time.
// conversationID nil => all of the user's conversations; otherwise scoped to one
// conversation. Ownership of the conversation is proven before any data is read.
func (s *AgentInteractionService) GetInteractionMetrics(
	ctx context.Context,
	userID uuid.UUID,
	conversationID *uuid.UUID,
) (InteractionMetrics, error) {
	if conversationID != nil && *conversationID != uuid.Nil {
		conversation, err := s.conversationRepo.GetByID(ctx, *conversationID, userID)
		if err != nil {
			return InteractionMetrics{}, fmt.Errorf("verify interaction metrics ownership: %w", err)
		}
		if conversation == nil {
			return InteractionMetrics{}, ErrConversationNotFound
		}
	}
	answered, expired, pending, avgWait, err := s.repo.AggregateInteractionMetrics(ctx, userID, conversationID)
	if err != nil {
		return InteractionMetrics{}, err
	}
	total := answered + expired + pending
	m := InteractionMetrics{
		Total:          total,
		Answered:       answered,
		Expired:        expired,
		Pending:        pending,
		AvgWaitSeconds: avgWait,
	}
	closed := answered + expired
	if closed > 0 {
		m.AnswerRate = float64(answered) / float64(closed)
		m.ExpireRate = float64(expired) / float64(closed)
	}
	return m, nil
}
