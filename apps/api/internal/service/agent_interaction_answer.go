package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// ResumeInteraction marks an interaction as answered. It is idempotent for
// repeated submissions with the same answer and rejects conflicting answers.
func (s *AgentInteractionService) ResumeInteraction(
	ctx context.Context,
	interactionID uuid.UUID,
	answer datatypes.JSON,
) error {
	interaction, err := s.repo.GetByID(ctx, interactionID)
	if err != nil {
		return fmt.Errorf("get interaction: %w", err)
	}
	if interaction == nil {
		return ErrInteractionNotFound
	}
	if interaction.Status == model.AgentInteractionAnswered {
		if jsonEqual(interaction.Answer, answer) {
			return nil
		}
		return ErrInteractionConflict
	}
	if interaction.Status == model.AgentInteractionExpired {
		return ErrInteractionExpired
	}
	if interaction.Status != model.AgentInteractionPending {
		return fmt.Errorf("%w: %s", ErrInteractionClosed, interaction.Status)
	}
	// Soft-expire if the TTL elapsed but the sweeper has not yet run.
	if interaction.ExpiresAt != nil && !interaction.ExpiresAt.After(time.Now().UTC()) {
		if won, expErr := s.repo.ExpirePending(ctx, interactionID); expErr != nil {
			log.Printf("failed to expire interaction %s: %v", interactionID, expErr)
		} else if won {
			s.settled(ctx, *interaction)
		}
		return ErrInteractionExpired
	}

	updated, err := s.repo.MarkAnswered(ctx, interactionID, answer)
	if err != nil {
		return fmt.Errorf("mark answered: %w", err)
	}
	if !updated {
		latest, latestErr := s.repo.GetByID(ctx, interactionID)
		if latestErr != nil {
			return fmt.Errorf("reload interaction after answer race: %w", latestErr)
		}
		if latest != nil && latest.Status == model.AgentInteractionAnswered && jsonEqual(latest.Answer, answer) {
			return nil
		}
		return ErrInteractionConflict
	}

	return nil
}

// ResumeInteractionWithEvent is the production answer transition. The
// pending->answered CAS and state.interaction.answered milestone share one
// transaction on the original interrupted run.
func (s *AgentInteractionService) ResumeInteractionWithEvent(
	ctx context.Context,
	interactionID uuid.UUID,
	answer datatypes.JSON,
) error {
	interaction, err := s.repo.GetByID(ctx, interactionID)
	if err != nil {
		return fmt.Errorf("get interaction: %w", err)
	}
	if interaction == nil {
		return ErrInteractionNotFound
	}
	if interaction.Status == model.AgentInteractionAnswered {
		if jsonEqual(interaction.Answer, answer) {
			return nil
		}
		return ErrInteractionConflict
	}
	if interaction.Status == model.AgentInteractionExpired {
		return ErrInteractionExpired
	}
	if interaction.Status != model.AgentInteractionPending {
		return fmt.Errorf("%w: %s", ErrInteractionClosed, interaction.Status)
	}
	if interaction.ExpiresAt != nil && !interaction.ExpiresAt.After(time.Now().UTC()) {
		if _, expErr := s.expireInteractionWithEvent(ctx, *interaction); expErr != nil {
			return fmt.Errorf("expire overdue interaction: %w", expErr)
		}
		return ErrInteractionExpired
	}
	if s.transactions == nil || s.lifecycleEvents == nil {
		return errors.New("answer interaction: lifecycle coordinator is not configured")
	}
	if err := s.lifecycleEvents.Flush(ctx); err != nil {
		return fmt.Errorf("flush runtime events before interaction answer: %w", err)
	}

	updated := false
	err = s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		won, err := s.repo.MarkAnswered(txCtx, interactionID, answer)
		if err != nil {
			return fmt.Errorf("mark answered: %w", err)
		}
		if !won {
			return nil
		}
		updated = true
		_, err = s.lifecycleEvents.PersistOutOfBandMilestone(
			txCtx,
			interaction.ConversationID,
			interaction.RunID,
			nil,
			"state",
			"state.interaction.answered",
			dto.StreamEventIDs{
				ConversationID: interaction.ConversationID.String(),
				RunID:          interaction.RunID.String(),
				InteractionID:  interaction.ID.String(),
				ToolCallID:     interaction.ToolCallID,
			},
			map[string]any{
				"interaction_id": interaction.ID.String(),
				"answer":         json.RawMessage(answer),
			},
		)
		return err
	})
	if err != nil {
		return fmt.Errorf("answer interaction with lifecycle event: %w", err)
	}
	if updated {
		return nil
	}
	latest, latestErr := s.repo.GetByID(ctx, interactionID)
	if latestErr != nil {
		return fmt.Errorf("reload interaction after answer race: %w", latestErr)
	}
	if latest != nil && latest.Status == model.AgentInteractionAnswered && jsonEqual(latest.Answer, answer) {
		return nil
	}
	return ErrInteractionConflict
}

// CancelInteraction marks a pending interaction as cancelled.
func (s *AgentInteractionService) CancelInteraction(ctx context.Context, interactionID uuid.UUID) error {
	interaction, err := s.repo.GetByID(ctx, interactionID)
	if err != nil {
		return fmt.Errorf("get interaction: %w", err)
	}
	if interaction == nil {
		return ErrInteractionNotFound
	}
	if interaction.Status == model.AgentInteractionCancelled {
		return nil
	}
	if interaction.Status != model.AgentInteractionPending {
		return fmt.Errorf("%w: %s", ErrInteractionClosed, interaction.Status)
	}
	updated, err := s.repo.CancelPending(ctx, interactionID)
	if err != nil {
		return fmt.Errorf("cancel interaction: %w", err)
	}
	if !updated {
		return ErrInteractionClosed
	}
	s.settled(ctx, *interaction)
	return nil
}
