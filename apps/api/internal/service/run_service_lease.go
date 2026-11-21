package service

import (
	"context"
	"fmt"
	"time"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

const runLeaseHeartbeatInterval = 1 * time.Minute
const runExecutionLostErrorJSON = `{"message":"run execution lost; lease expired"}`

// StartLeaseHeartbeat renews a running execution until the context ends. The
// owner-bound update makes a stale process unable to resurrect a reclaimed run.
func (s *RunService) StartLeaseHeartbeat(ctx context.Context, id, userID uuid.UUID) func() {
	heartbeatCtx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(runLeaseHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case now := <-ticker.C:
				expires := now.Add(runLeaseDuration)
				alive, err := s.runRepo.RenewLease(heartbeatCtx, id, userID, s.leaseOwner, expires, now)
				if err != nil || !alive {
					return
				}
			}
		}
	}()
	return cancel
}

// ReconcileExpiredRuns atomically commits execution_lost and run.failed for
// every expired Run. Listing is advisory; FailExpiredRun is the terminal CAS,
// so concurrent API instances cannot both win.
func (s *RunService) ReconcileExpiredRuns(ctx context.Context, limit int) ([]model.Run, error) {
	if err := s.requireLifecycleCoordinator(); err != nil {
		return nil, err
	}
	if err := s.lifecycleEvents.Flush(ctx); err != nil {
		return nil, fmt.Errorf("flush runtime events before lease reconciliation: %w", err)
	}
	now := time.Now()
	candidates, err := s.runRepo.ListExpiredRuns(ctx, now, limit)
	if err != nil {
		return nil, fmt.Errorf("list expired runs: %w", err)
	}
	reclaimed := make([]model.Run, 0, len(candidates))
	for i := range candidates {
		candidate := candidates[i]
		committed := false
		err := s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
			won, err := s.runRepo.FailExpiredRun(txCtx, candidate.ID, now, datatypes.JSON([]byte(runExecutionLostErrorJSON)))
			if err != nil {
				return err
			}
			if !won {
				return nil
			}
			candidate.Status = model.RunStatusFailed
			candidate.Error = datatypes.JSON([]byte(runExecutionLostErrorJSON))
			candidate.CompletedAt = &now
			candidate.LeaseOwner = ""
			candidate.LeaseExpiresAt = nil
			candidate.LeaseHeartbeatAt = nil
			turnID := candidate.TurnID
			_, err = s.lifecycleEvents.PersistOutOfBandMilestone(
				txCtx, candidate.ConversationID, candidate.ID, &turnID,
				"run", "run.failed",
				dto.StreamEventIDs{ConversationID: candidate.ConversationID.String(), RunID: candidate.ID.String(), TurnID: candidate.TurnID.String()},
				map[string]any{"status": "failed", "reason": "execution_lost"},
			)
			if err != nil {
				return err
			}
			committed = true
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("reconcile expired run %s: %w", candidate.ID, err)
		}
		if committed {
			reclaimed = append(reclaimed, candidate)
		}
	}
	return reclaimed, nil
}
