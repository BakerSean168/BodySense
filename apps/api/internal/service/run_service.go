package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bodysense/api/internal/dto"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

var ErrRunTerminal = errors.New("run is already terminal")

// runLeaseDuration bounds how long a run may sit in the running state before
// the runtime is assumed dead and the run is reclaimed. It is comfortably
// larger than the SSE timeout so live runs never expire their lease.
const runLeaseDuration = 30 * time.Minute

// RunService handles run business logic.
type runLifecycleEventStore interface {
	Flush(ctx context.Context) error
	PersistPreparedMilestone(ctx context.Context, conversationID, runID uuid.UUID, turnID *uuid.UUID, event dto.StreamEvent) error
	PersistOutOfBandMilestone(ctx context.Context, conversationID, runID uuid.UUID, turnID *uuid.UUID, channel, eventType string, ids dto.StreamEventIDs, payload any) (dto.StreamEvent, error)
}

type runTransactionManager interface {
	WithinTransaction(ctx context.Context, fn func(context.Context) error) error
}

type RunService struct {
	runRepo         runRepo
	leaseOwner      string
	lifecycleEvents runLifecycleEventStore
	transactions    runTransactionManager
}

// NewRunService creates a new RunService.
func NewRunService(runRepo runRepo, leaseOwners ...string) *RunService {
	owner := uuid.NewString()
	if len(leaseOwners) > 0 && leaseOwners[0] != "" {
		owner = leaseOwners[0]
	}
	return &RunService{runRepo: runRepo, leaseOwner: owner}
}

// WithLifecycleEvents binds the transactional public lifecycle log. Terminal
// Run mutations use this coordinator so durable state and the authoritative
// run.completed/failed/cancelled event share one commit boundary.
func (s *RunService) WithLifecycleEvents(events runLifecycleEventStore, transactions runTransactionManager) *RunService {
	s.lifecycleEvents = events
	s.transactions = transactions
	return s
}

func (s *RunService) requireLifecycleCoordinator() error {
	if s.lifecycleEvents == nil || s.transactions == nil {
		return errors.New("run lifecycle coordinator is not configured")
	}
	return nil
}

// UpdateAgentConfiguration persists the immutable Agent configuration +
// execution provenance captured from the runtime.agent_configuration event.
func (s *RunService) UpdateAgentConfiguration(
	ctx context.Context,
	id uuid.UUID,
	configurationID string,
	configuration datatypes.JSON,
	provenance datatypes.JSON,
) error {
	if err := s.runRepo.UpdateAgentConfiguration(ctx, id, configurationID, configuration, provenance); err != nil {
		return fmt.Errorf("update run agent configuration: %w", err)
	}
	return nil
}

// leaseExpiry returns the timestamp at which a newly-created run's lease
// expires. The runtime extends the lease as long as the run is actively
// streaming; an un-renewed expiry means the owning process died and the run
// may be reclaimed.
func leaseExpiry() *time.Time {
	expires := time.Now().Add(runLeaseDuration)
	return &expires
}
