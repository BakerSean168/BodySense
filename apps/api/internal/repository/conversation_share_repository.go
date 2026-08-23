package repository

import (
	"context"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ConversationShareRepository handles database operations for conversation shares.
type ConversationShareRepository struct {
	db *gorm.DB
}

// NewConversationShareRepository creates a new ConversationShareRepository.
func NewConversationShareRepository(db *gorm.DB) *ConversationShareRepository {
	return &ConversationShareRepository{db: db}
}

// Create creates a new conversation share.
func (r *ConversationShareRepository) Create(ctx context.Context, share *model.ConversationShare) error {
	return r.db.WithContext(ctx).Create(share).Error
}

// GetByToken retrieves a share by its token.
func (r *ConversationShareRepository) GetByToken(ctx context.Context, token string) (*model.ConversationShare, error) {
	var share model.ConversationShare
	err := r.db.WithContext(ctx).
		Where("share_token = ?", token).
		First(&share).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &share, nil
}

// GetByConversationID retrieves a share by conversation ID.
func (r *ConversationShareRepository) GetByConversationID(ctx context.Context, conversationID uuid.UUID) (*model.ConversationShare, error) {
	var share model.ConversationShare
	err := r.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		First(&share).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &share, nil
}

// Delete deletes a share by ID.
func (r *ConversationShareRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&model.ConversationShare{}).Error
}

// DeleteByConversationID deletes a share by conversation ID.
func (r *ConversationShareRepository) DeleteByConversationID(ctx context.Context, conversationID uuid.UUID) error {
	return database.FromContext(ctx, r.db).
		Where("conversation_id = ?", conversationID).
		Delete(&model.ConversationShare{}).Error
}
