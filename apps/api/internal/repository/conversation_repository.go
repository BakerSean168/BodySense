package repository

import (
	"context"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ConversationRepository handles database operations for conversations.
type ConversationRepository struct {
	db *gorm.DB
}

// NewConversationRepository creates a new ConversationRepository.
func NewConversationRepository(db *gorm.DB) *ConversationRepository {
	return &ConversationRepository{db: db}
}

// Create creates a new conversation.
func (r *ConversationRepository) Create(ctx context.Context, conversation *model.Conversation) error {
	return r.db.WithContext(ctx).Create(conversation).Error
}

// GetByID retrieves a conversation by ID with ownership check.
func (r *ConversationRepository) GetByID(ctx context.Context, id, userID uuid.UUID) (*model.Conversation, error) {
	var conversation model.Conversation
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", id, userID).
		First(&conversation).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &conversation, nil
}

// GetByIDNoUser retrieves a conversation by ID without ownership check.
func (r *ConversationRepository) GetByIDNoUser(ctx context.Context, id uuid.UUID) (*model.Conversation, error) {
	var conversation model.Conversation
	err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&conversation).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &conversation, nil
}

// ListByUserID retrieves conversations for a user with cursor-based pagination.
// When cursor is empty, returns the most recent conversations.
// When cursor is non-empty, returns conversations older than the cursor timestamp.
// Returns limit+1 items to determine hasMore; the extra item is not included in the result.
func (r *ConversationRepository) ListByUserID(ctx context.Context, userID uuid.UUID, cursor *time.Time, limit int) ([]model.Conversation, bool, error) {
	var conversations []model.Conversation

	query := r.db.WithContext(ctx).
		Where("user_id = ? AND deleted_at IS NULL AND last_message_at IS NOT NULL", userID)

	if cursor != nil {
		query = query.Where("last_message_at < ?", *cursor)
	}

	err := query.
		Order("last_message_at DESC").
		Limit(limit + 1).
		Find(&conversations).Error
	if err != nil {
		return nil, false, err
	}

	hasMore := len(conversations) > limit
	if hasMore {
		conversations = conversations[:limit]
	}

	return conversations, hasMore, nil
}

// Update updates a conversation.
func (r *ConversationRepository) Update(ctx context.Context, conversation *model.Conversation) error {
	return r.db.WithContext(ctx).Save(conversation).Error
}

// SoftDelete soft-deletes a conversation by ID with ownership check.
func (r *ConversationRepository) SoftDelete(ctx context.Context, id, userID uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&model.Conversation{}).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", id, userID).
		Update("deleted_at", now).Error
}

// UpdatePinned updates the pinned status of a conversation with ownership check.
func (r *ConversationRepository) UpdatePinned(ctx context.Context, id, userID uuid.UUID, pinned bool) error {
	updates := map[string]any{
		"pinned": pinned,
	}
	if pinned {
		now := time.Now()
		updates["pinned_at"] = now
	} else {
		updates["pinned_at"] = nil
	}

	return r.db.WithContext(ctx).
		Model(&model.Conversation{}).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", id, userID).
		Updates(updates).Error
}

// UpdateTitle updates the title of a conversation with ownership check.
func (r *ConversationRepository) UpdateTitle(ctx context.Context, id, userID uuid.UUID, title string) error {
	return r.db.WithContext(ctx).
		Model(&model.Conversation{}).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", id, userID).
		Update("title", title).Error
}

// UpdateTitleStatus updates the title_status of a conversation with ownership check.
func (r *ConversationRepository) UpdateTitleStatus(ctx context.Context, id, userID uuid.UUID, status string) error {
	return r.db.WithContext(ctx).
		Model(&model.Conversation{}).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", id, userID).
		Update("title_status", status).Error
}

// UpdateLastMessageAt updates the last_message_at of a conversation to now with ownership check.
func (r *ConversationRepository) UpdateLastMessageAt(ctx context.Context, id, userID uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&model.Conversation{}).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", id, userID).
		Update("last_message_at", now).Error
}

// UpdateActiveRunID sets or clears the active_run_id and active_stream_id on a conversation.
func (r *ConversationRepository) UpdateActiveRunID(ctx context.Context, id, userID uuid.UUID, runID *uuid.UUID, streamID string) error {
	return r.db.WithContext(ctx).
		Model(&model.Conversation{}).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", id, userID).
		Updates(map[string]any{
			"active_run_id":    runID,
			"active_stream_id": streamID,
		}).Error
}

// UpdateStatus updates the status of a conversation with ownership check.
func (r *ConversationRepository) UpdateStatus(ctx context.Context, id, userID uuid.UUID, status string) error {
	return r.db.WithContext(ctx).
		Model(&model.Conversation{}).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", id, userID).
		Update("status", status).Error
}

// GetLastEmptyConversation retrieves the most recent empty conversation for a user.
func (r *ConversationRepository) GetLastEmptyConversation(ctx context.Context, userID uuid.UUID) (*model.Conversation, error) {
	var conversation model.Conversation
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND status = ? AND last_message_at IS NULL AND deleted_at IS NULL", userID, "active").
		Order("created_at DESC").
		First(&conversation).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &conversation, nil
}
