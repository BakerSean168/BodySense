package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"gorm.io/datatypes"
)

// ExpireExpiredInteractions closes due pending interactions. When lifecycle
// events are configured (production), interaction.expired and the waiting Run's
// run.failed transition are committed atomically. Tests may still exercise the
// repository-only form by leaving the event store unset.
func (s *AgentInteractionService) ExpireExpiredInteractions(
	ctx context.Context,
	limit int,
) ([]model.AgentInteraction, error) {
	due, err := s.repo.ListExpiredPending(ctx, time.Now().UTC(), limit)
	if err != nil {
		return nil, fmt.Errorf("list expired interactions: %w", err)
	}
	expired := make([]model.AgentInteraction, 0, len(due))
	for _, item := range due {
		var updated bool
		var expErr error
		if s.lifecycleEvents != nil {
			updated, expErr = s.expireInteractionWithEvent(ctx, item)
		} else {
			updated, expErr = s.repo.ExpirePending(ctx, item.ID)
			if expErr == nil && updated {
				s.settled(ctx, item)
			}
		}
		if expErr != nil {
			slog.Default().Error("expire_interaction_failed", "interaction_id", item.ID, "error", expErr)
			continue
		}
		if updated {
			item.Status = model.AgentInteractionExpired
			expired = append(expired, item)
		}
	}
	return expired, nil
}

func (s *AgentInteractionService) expireInteractionWithEvent(ctx context.Context, interaction model.AgentInteraction) (bool, error) {
	if s.transactions == nil || s.lifecycleEvents == nil {
		return false, errors.New("expire interaction: lifecycle coordinator is not configured")
	}
	if err := s.lifecycleEvents.Flush(ctx); err != nil {
		return false, fmt.Errorf("flush runtime events before interaction expiry: %w", err)
	}
	expiredAt := time.Now().UTC()
	updated := false
	err := s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		won, err := s.repo.ExpirePending(txCtx, interaction.ID)
		if err != nil {
			return err
		}
		if !won {
			return nil
		}
		updated = true
		runFailed, err := s.runLifecycle.FailWaitingUser(
			txCtx,
			interaction.RunID,
			datatypes.JSON(`{"message":"interaction expired"}`),
		)
		if err != nil {
			return err
		}
		_, err = s.lifecycleEvents.PersistOutOfBandMilestone(
			txCtx,
			interaction.ConversationID,
			interaction.RunID,
			nil,
			"state",
			"state.interaction.expired",
			dto.StreamEventIDs{
				ConversationID: interaction.ConversationID.String(),
				RunID:          interaction.RunID.String(),
				InteractionID:  interaction.ID.String(),
				ToolCallID:     interaction.ToolCallID,
			},
			map[string]any{
				"interaction_id": interaction.ID.String(),
				"expired_at":     expiredAt.Format(time.RFC3339Nano),
				"reason":         "ttl_elapsed",
			},
		)
		if err != nil {
			return err
		}
		if runFailed {
			_, err = s.lifecycleEvents.PersistOutOfBandMilestone(
				txCtx,
				interaction.ConversationID,
				interaction.RunID,
				nil,
				"run",
				"run.failed",
				dto.StreamEventIDs{
					ConversationID: interaction.ConversationID.String(),
					RunID:          interaction.RunID.String(),
					InteractionID:  interaction.ID.String(),
				},
				map[string]any{"status": "failed", "reason": "interaction_expired"},
			)
		}
		return err
	})
	if err != nil {
		return false, fmt.Errorf("expire interaction with lifecycle events: %w", err)
	}
	if updated {
		s.settled(ctx, interaction)
	}
	return updated, nil
}

// InteractionExpiredHandler is called after an interaction is marked expired.
// Typically records state.interaction.expired into the durable event log.
type InteractionExpiredHandler func(ctx context.Context, interaction model.AgentInteraction)

// StartInteractionExpiryWorker periodically sweeps expired pending interactions.
// onExpired is optional; when set it receives each newly expired interaction.
func (s *AgentInteractionService) StartInteractionExpiryWorker(
	ctx context.Context,
	interval time.Duration,
	onExpired InteractionExpiredHandler,
) {
	if interval <= 0 {
		interval = time.Minute
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				expired, err := s.ExpireExpiredInteractions(ctx, 100)
				if err != nil {
					slog.Default().Error("interaction_expiry_sweep_failed", "error", err)
					continue
				}
				if len(expired) == 0 {
					continue
				}
				slog.Default().Info("interaction_expiry_sweep_expired", "expired_count", len(expired))
				if onExpired == nil {
					continue
				}
				for _, item := range expired {
					onExpired(ctx, item)
				}
			}
		}
	}()
}
