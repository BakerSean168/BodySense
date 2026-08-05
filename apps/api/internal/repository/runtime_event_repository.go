package repository

import (
	"context"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RuntimeEventRepository handles database operations for durable runtime events.
type RuntimeEventRepository struct {
	db *gorm.DB
}

// NewRuntimeEventRepository creates a new RuntimeEventRepository.
func NewRuntimeEventRepository(db *gorm.DB) *RuntimeEventRepository {
	return &RuntimeEventRepository{db: db}
}

// Create appends a new runtime event.
func (r *RuntimeEventRepository) Create(ctx context.Context, event *model.RuntimeEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}

// CreateBatch appends multiple runtime events in one statement.
// Used by the text.delta write buffer to cut write amplification while keeping
// one row per event (replay equivalence with the live stream).
func (r *RuntimeEventRepository) CreateBatch(ctx context.Context, events []*model.RuntimeEvent) error {
	if len(events) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&events).Error
}

// ListByRunID returns events for a run after the provided sequence, ordered by seq ascending.
// Returns limit+1 rows internally to determine hasMore.
func (r *RuntimeEventRepository) ListByRunID(
	ctx context.Context,
	conversationID, runID uuid.UUID,
	afterSeq, limit int,
) ([]model.RuntimeEvent, bool, error) {
	var events []model.RuntimeEvent

	err := r.db.WithContext(ctx).
		Where("conversation_id = ? AND run_id = ? AND seq > ?", conversationID, runID, afterSeq).
		Order("seq ASC").
		Limit(limit + 1).
		Find(&events).Error
	if err != nil {
		return nil, false, err
	}

	hasMore := len(events) > limit
	if hasMore {
		events = events[:limit]
	}

	return events, hasMore, nil
}

// ListByConversationID returns all durable events for a conversation ordered for timeline projection.
func (r *RuntimeEventRepository) ListByConversationID(
	ctx context.Context,
	conversationID uuid.UUID,
) ([]model.RuntimeEvent, error) {
	var events []model.RuntimeEvent
	err := r.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Order("created_at ASC, seq ASC").
		Find(&events).Error
	return events, err
}
