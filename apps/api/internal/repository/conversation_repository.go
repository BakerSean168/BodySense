package repository

import (
	"context"
	"errors"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ConversationRepository handles database operations for the per-user
// singleton Conversation (PC-D4).
type ConversationRepository struct {
	db *gorm.DB
}

// ErrInvalidConversationOwner prevents a missing auth identity from being
// used for a tenant-scoped lookup or lazy creation.
var ErrInvalidConversationOwner = errors.New("valid conversation owner required")

// NewConversationRepository creates a new ConversationRepository.
func NewConversationRepository(db *gorm.DB) *ConversationRepository {
	return &ConversationRepository{db: db}
}

// GetOrCreateForUser returns the owner's single Conversation, lazily creating
// it (and its consultation session) on first use. The `conversations_user_id_key`
// unique constraint makes creation race-free: concurrent first calls from
// several devices all converge on the same identity. created reports whether
// this call inserted the row.
func (r *ConversationRepository) GetOrCreateForUser(ctx context.Context, userID uuid.UUID) (*model.Conversation, bool, error) {
	if userID == uuid.Nil {
		return nil, false, ErrInvalidConversationOwner
	}
	var conversation *model.Conversation
	var created bool
	err := database.FromContext(ctx, r.db).WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		resolved, _, inserted, err := getOrCreateSingletonConversation(ctx, tx, userID)
		if err != nil {
			return err
		}
		conversation, created = resolved, inserted
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return conversation, created, nil
}

// getOrCreateSingletonConversation is the single creation path of a
// Conversation. It never selects among several rows: the database guarantees
// at most one row per owner.
func getOrCreateSingletonConversation(
	ctx context.Context,
	tx *gorm.DB,
	userID uuid.UUID,
) (*model.Conversation, *model.ConsultationSession, bool, error) {
	if userID == uuid.Nil {
		return nil, nil, false, ErrInvalidConversationOwner
	}
	candidate := model.Conversation{
		ID:          uuid.New(),
		UserID:      userID,
		Status:      model.ConversationStatusActive,
		TitleStatus: "pending",
	}
	inserted := tx.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoNothing: true,
	}).Create(&candidate)
	if inserted.Error != nil {
		return nil, nil, false, inserted.Error
	}
	created := inserted.RowsAffected == 1

	var conversation model.Conversation
	if err := tx.WithContext(ctx).Where("user_id = ?", userID).First(&conversation).Error; err != nil {
		return nil, nil, false, err
	}

	session := model.ConsultationSession{
		ConversationID: conversation.ID,
		ExtractedInfo:  datatypes.JSON("[]"),
		Phase:          model.ConsultationPhaseCollecting,
	}
	if err := tx.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "conversation_id"}},
		DoNothing: true,
	}).Create(&session).Error; err != nil {
		return nil, nil, false, err
	}
	if err := tx.WithContext(ctx).Where("conversation_id = ?", conversation.ID).First(&session).Error; err != nil {
		return nil, nil, false, err
	}
	return &conversation, &session, created, nil
}

// GetByUserID returns the owner's Conversation without creating it, or nil.
func (r *ConversationRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (*model.Conversation, error) {
	if userID == uuid.Nil {
		return nil, ErrInvalidConversationOwner
	}
	var conversation model.Conversation
	err := database.FromContext(ctx, r.db).WithContext(ctx).
		Where("user_id = ?", userID).
		First(&conversation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &conversation, nil
}

// GetByID retrieves a conversation by ID with ownership check.
func (r *ConversationRepository) GetByID(ctx context.Context, id, userID uuid.UUID) (*model.Conversation, error) {
	db := database.FromContext(ctx, r.db)
	var conversation model.Conversation
	err := db.
		Where("id = ? AND user_id = ?", id, userID).
		First(&conversation).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &conversation, nil
}

// UpdateTitleGeneration atomically persists the generated title and its immutable Agent provenance.
func (r *ConversationRepository) UpdateTitleGeneration(ctx context.Context, id, userID uuid.UUID, title, status, configurationID string, configuration, provenance, decisionTrace []byte) error {
	return r.db.WithContext(ctx).
		Model(&model.Conversation{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]any{
			"title":                        title,
			"title_status":                 status,
			"title_agent_configuration_id": configurationID,
			"title_agent_configuration":    configuration,
			"title_execution_provenance":   provenance,
			"title_decision_trace":         decisionTrace,
		}).Error
}

// UpdateTitleStatus updates the title_status of a conversation with ownership check.
func (r *ConversationRepository) UpdateTitleStatus(ctx context.Context, id, userID uuid.UUID, status string) error {
	return r.db.WithContext(ctx).
		Model(&model.Conversation{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("title_status", status).Error
}

// UpdateLastMessageAt updates the last_message_at of a conversation to now with ownership check.
func (r *ConversationRepository) UpdateLastMessageAt(ctx context.Context, id, userID uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&model.Conversation{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("last_message_at", now).Error
}

// UpdateActiveRunID sets or clears the active_run_id and active_stream_id on a conversation.
func (r *ConversationRepository) UpdateActiveRunID(ctx context.Context, id, userID uuid.UUID, runID *uuid.UUID, streamID string) error {
	return database.FromContext(ctx, r.db).
		Model(&model.Conversation{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]any{
			"active_run_id":    runID,
			"active_stream_id": streamID,
		}).Error
}
