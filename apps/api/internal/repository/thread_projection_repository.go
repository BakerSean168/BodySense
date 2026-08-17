package repository

import (
	"context"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ThreadProjectionRepository handles durable thread projection persistence and queries.
type ThreadProjectionRepository struct {
	db *gorm.DB
}

// NewThreadProjectionRepository creates a new ThreadProjectionRepository.
func NewThreadProjectionRepository(db *gorm.DB) *ThreadProjectionRepository {
	return &ThreadProjectionRepository{db: db}
}

// UpsertSnapshot replaces the stored thread snapshot, projected messages, and tool timeline atomically.
func (r *ThreadProjectionRepository) UpsertSnapshot(
	ctx context.Context,
	projection *model.ThreadProjection,
	messages []model.ThreadProjectionMessage,
	toolCalls []model.ThreadProjectionToolCall,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "conversation_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"user_id",
				"title",
				"title_status",
				"status",
				"pinned",
				"pinned_at",
				"default_model",
				"active_run_id",
				"last_message_at",
				"metadata",
				"phase",
				"extracted_info",
				"pending_interactions",
				"interaction_history",
				"conversation_created_at",
				"conversation_updated_at",
				"session_created_at",
				"session_updated_at",
				"ended_at",
				"refreshed_at",
			}),
		}).Create(projection).Error; err != nil {
			return err
		}

		if err := tx.Where("conversation_id = ?", projection.ConversationID).Delete(&model.ThreadProjectionMessage{}).Error; err != nil {
			return err
		}
		if len(messages) > 0 {
			if err := tx.Create(&messages).Error; err != nil {
				return err
			}
		}

		if err := tx.Where("conversation_id = ?", projection.ConversationID).Delete(&model.ThreadProjectionToolCall{}).Error; err != nil {
			return err
		}
		if len(toolCalls) > 0 {
			if err := tx.Create(&toolCalls).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// GetByConversationID returns the durable thread snapshot, projected messages, and tool timeline for the owner.
func (r *ThreadProjectionRepository) GetByConversationID(
	ctx context.Context,
	conversationID, userID uuid.UUID,
) (*model.ThreadProjection, []model.ThreadProjectionMessage, []model.ThreadProjectionToolCall, error) {
	var projection model.ThreadProjection
	err := r.db.WithContext(ctx).
		Where("conversation_id = ? AND user_id = ?", conversationID, userID).
		First(&projection).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil, nil, nil
	}
	if err != nil {
		return nil, nil, nil, err
	}

	var messages []model.ThreadProjectionMessage
	err = r.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Order("seq ASC").
		Find(&messages).Error
	if err != nil {
		return nil, nil, nil, err
	}

	var toolCalls []model.ThreadProjectionToolCall
	err = r.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Order("created_at ASC").
		Find(&toolCalls).Error
	if err != nil {
		return nil, nil, nil, err
	}

	return &projection, messages, toolCalls, nil
}
