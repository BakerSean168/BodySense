package service

// Learning path (Thought Forest note filenames):
//   - go-context.md
//   - go-error-wrapping.md
//   - go-interfaces.md
//   - go-sync-package.md
//   - go-slices.md
//
// This file is a useful example of a small Go interface around persistence,
// explicit error propagation, mutex-protected shared state, and batch I/O.
import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// Default number of buffered text.delta events before an automatic flush.
const defaultDeltaFlushSize = 32

type runtimeEventRepo interface {
	// The service depends on the smallest persistence capability it needs,
	// rather than on a concrete GORM repository. Tests can provide a lightweight
	// fake that satisfies this method set implicitly.
	Create(ctx context.Context, event *model.RuntimeEvent) error
	CreateBatch(ctx context.Context, events []*model.RuntimeEvent) error
	CreateWithNextSequence(ctx context.Context, event *model.RuntimeEvent) error
	ListByRunID(ctx context.Context, conversationID, runID uuid.UUID, afterSeq, limit int) ([]model.RuntimeEvent, bool, error)
	ListByConversationID(ctx context.Context, conversationID uuid.UUID) ([]model.RuntimeEvent, error)
}

// RuntimeEventService persists and queries replayable public runtime events.
//
// High-frequency message.text.delta events are buffered and written in batches
// to reduce write amplification. Milestone events always flush the buffer first
// so durable order stays equivalent to the live stream (same seqs, same rows).
type RuntimeEventService struct {
	repo           runtimeEventRepo
	deltaFlushSize int

	// mu protects deltaBuffer, not the repository. Every read or write of the
	// slice header must happen while holding this lock because append can replace
	// its backing array. Never hold the lock during database I/O.
	mu          sync.Mutex
	deltaBuffer []*model.RuntimeEvent
}

// NewRuntimeEventService creates a new RuntimeEventService.
func NewRuntimeEventService(repo runtimeEventRepo) *RuntimeEventService {
	return &RuntimeEventService{
		repo:           repo,
		deltaFlushSize: defaultDeltaFlushSize,
	}
}

// ShouldPersistEvent reports whether the public event should be stored durably.
func ShouldPersistEvent(eventType string) bool {
	switch eventType {
	case "conversation.created",
		"run.started",
		"run.resumed",
		"run.interrupted",
		"run.completed",
		"run.failed",
		"run.cancelled",
		"message.persisted",
		"message.created",
		"message.text.delta",
		"tool.call",
		"tool.result",
		"state.extracted_info.upsert",
		"state.phase.changed",
		"source.citation.added",
		"source.answer_attribution.added",
		"source.knowledge_gap",
		"safety.red_flag.detected",
		"safety.output_reviewed",
		"safety.output_rejected",
		"state.interaction.required",
		"state.interaction.answered",
		"state.interaction.expired",
		"message.completed",
		"message.failed",
		"title.generated",
		"usage.reported",
		"stream.done",
		"stream.error":
		return true
	default:
		return false
	}
}

func isBufferedDelta(eventType string) bool {
	return eventType == "message.text.delta"
}

// RecordPublicEvent maps a public StreamEvent into a durable runtime event.
func (s *RuntimeEventService) RecordPublicEvent(
	ctx context.Context,
	conversationID uuid.UUID,
	runID uuid.UUID,
	turnID *uuid.UUID,
	event dto.StreamEvent,
) error {
	if !ShouldPersistEvent(event.Type) {
		return nil
	}

	record, err := buildRuntimeEventRecord(conversationID, runID, turnID, event)
	if err != nil {
		return err
	}

	if isBufferedDelta(event.Type) {
		return s.enqueueDelta(ctx, record)
	}

	// Milestone events: flush pending deltas first so seq order in the durable
	// log matches the live stream, then write synchronously.
	if err := s.Flush(ctx); err != nil {
		return err
	}
	if err := s.repo.Create(ctx, record); err != nil {
		return fmt.Errorf("create runtime event: %w", err)
	}
	return nil
}

// Flush writes any buffered text.delta events. Safe to call repeatedly.
func (s *RuntimeEventService) Flush(ctx context.Context) error {
	s.mu.Lock()
	if len(s.deltaBuffer) == 0 {
		s.mu.Unlock()
		return nil
	}
	// Copy the slice header out and detach the shared buffer while locked. The
	// batch elements remain valid because both slices contain pointers.
	batch := s.deltaBuffer
	s.deltaBuffer = nil
	s.mu.Unlock()

	// Database I/O happens after Unlock so slow storage does not block producers
	// from enqueueing new deltas.
	if err := s.repo.CreateBatch(ctx, batch); err != nil {
		// Put failed batch back so a retry can re-flush (best-effort).
		s.mu.Lock()
		s.deltaBuffer = append(batch, s.deltaBuffer...)
		s.mu.Unlock()
		// %w preserves the original error in the chain, allowing callers to use
		// errors.Is/errors.As while still adding operation-specific context.
		return fmt.Errorf("flush runtime event deltas: %w", err)
	}
	return nil
}

func (s *RuntimeEventService) enqueueDelta(ctx context.Context, record *model.RuntimeEvent) error {
	s.mu.Lock()
	s.deltaBuffer = append(s.deltaBuffer, record)
	shouldFlush := len(s.deltaBuffer) >= s.deltaFlushSize
	s.mu.Unlock()

	if shouldFlush {
		return s.Flush(ctx)
	}
	return nil
}

func buildRuntimeEventRecord(
	conversationID uuid.UUID,
	runID uuid.UUID,
	turnID *uuid.UUID,
	event dto.StreamEvent,
) (*model.RuntimeEvent, error) {
	idsJSON, err := json.Marshal(event.IDs)
	if err != nil {
		return nil, fmt.Errorf("marshal event ids: %w", err)
	}

	payload := event.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}

	return &model.RuntimeEvent{
		ConversationID: conversationID,
		RunID:          runID,
		TurnID:         turnID,
		Seq:            event.Seq,
		Channel:        event.Channel,
		Type:           event.Type,
		IDs:            datatypes.JSON(idsJSON),
		Payload:        datatypes.JSON(payload),
		Source:         "go",
		Replayable:     true,
	}, nil
}

// ListRunEvents returns durable events for a run.
func (s *RuntimeEventService) ListRunEvents(
	ctx context.Context,
	conversationID, runID uuid.UUID,
	afterSeq, limit int,
) ([]model.RuntimeEvent, bool, error) {
	events, hasMore, err := s.repo.ListByRunID(ctx, conversationID, runID, afterSeq, limit)
	if err != nil {
		return nil, false, fmt.Errorf("list runtime events: %w", err)
	}
	return events, hasMore, nil
}

// ListConversationEvents returns all durable events for a conversation in timeline order.
func (s *RuntimeEventService) ListConversationEvents(
	ctx context.Context,
	conversationID uuid.UUID,
) ([]model.RuntimeEvent, error) {
	events, err := s.repo.ListByConversationID(ctx, conversationID)
	if err != nil {
		return nil, fmt.Errorf("list conversation runtime events: %w", err)
	}
	return events, nil
}

// ListAllRunEvents returns all durable events for a run in ascending seq order.
func (s *RuntimeEventService) ListAllRunEvents(
	ctx context.Context,
	conversationID, runID uuid.UUID,
) ([]model.RuntimeEvent, error) {
	const batchSize = 1000

	all := make([]model.RuntimeEvent, 0, batchSize)
	afterSeq := 0

	for {
		events, hasMore, err := s.ListRunEvents(ctx, conversationID, runID, afterSeq, batchSize)
		if err != nil {
			return nil, err
		}
		if len(events) == 0 {
			return all, nil
		}

		all = append(all, events...)
		afterSeq = events[len(events)-1].Seq
		if !hasMore {
			return all, nil
		}
	}
}

// RecordOutOfBandPublicEvent appends a public event when no live StreamWriter
// owns sequencing (for example an expired HITL interaction while a run waits).
// The repository allocates the next seq transactionally under a per-run row lock.
func (s *RuntimeEventService) RecordOutOfBandPublicEvent(
	ctx context.Context,
	conversationID, runID uuid.UUID,
	turnID *uuid.UUID,
	channel, eventType string,
	ids dto.StreamEventIDs,
	payload any,
) error {
	if !ShouldPersistEvent(eventType) {
		return fmt.Errorf("event %q is not a replayable public runtime event", eventType)
	}
	if err := s.Flush(ctx); err != nil {
		return err
	}
	event, err := dto.NewStreamEvent(1, channel, eventType, ids, payload)
	if err != nil {
		return err
	}
	record, err := buildRuntimeEventRecord(conversationID, runID, turnID, event)
	if err != nil {
		return err
	}
	if err := s.repo.CreateWithNextSequence(ctx, record); err != nil {
		return fmt.Errorf("create out-of-band runtime event: %w", err)
	}
	return nil
}

// RecordInteractionExpired persists a state.interaction.expired public event.
// Used by the interaction expiry sweeper (no live SSE writer available).
func (s *RuntimeEventService) RecordInteractionExpired(
	ctx context.Context,
	interaction *model.AgentInteraction,
) error {
	if interaction == nil {
		return nil
	}
	now := time.Now().UTC()
	return s.RecordOutOfBandPublicEvent(
		ctx,
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
			"expired_at":     now.Format(time.RFC3339Nano),
			"reason":         "ttl_elapsed",
		},
	)
}

// RecordRunCancelled persists cancellation for a waiting run that has no live
// writer left to own the public sequence.
func (s *RuntimeEventService) RecordRunCancelled(ctx context.Context, run *model.Run, reason string) error {
	if run == nil {
		return nil
	}
	if reason == "" {
		reason = "cancelled_by_user"
	}
	turnID := run.TurnID
	return s.RecordOutOfBandPublicEvent(
		ctx,
		run.ConversationID,
		run.ID,
		&turnID,
		"run",
		"run.cancelled",
		dto.StreamEventIDs{
			ConversationID: run.ConversationID.String(),
			RunID:          run.ID.String(),
			TurnID:         run.TurnID.String(),
		},
		map[string]any{"status": "cancelled", "reason": reason},
	)
}
