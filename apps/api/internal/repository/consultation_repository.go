package repository

import (
	"context"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ConsultationRepository handles database operations for consultation sessions.
type ConsultationRepository struct {
	db *gorm.DB
}

// NewConsultationRepository creates a new ConsultationRepository.
func NewConsultationRepository(db *gorm.DB) *ConsultationRepository {
	return &ConsultationRepository{db: db}
}

// Create creates a new consultation session.
func (r *ConsultationRepository) Create(ctx context.Context, session *model.ConsultationSession) error {
	return r.db.WithContext(ctx).Create(session).Error
}

// GetByConversationID retrieves a consultation session by conversation ID.
func (r *ConsultationRepository) GetByConversationID(ctx context.Context, conversationID uuid.UUID) (*model.ConsultationSession, error) {
	var session model.ConsultationSession
	err := r.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		First(&session).Error

	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// ListByConversationIDs retrieves consultation sessions for a set of conversation IDs.
func (r *ConsultationRepository) ListByConversationIDs(ctx context.Context, conversationIDs []uuid.UUID) ([]model.ConsultationSession, error) {
	var sessions []model.ConsultationSession
	if len(conversationIDs) == 0 {
		return sessions, nil
	}
	err := r.db.WithContext(ctx).
		Where("conversation_id IN ?", conversationIDs).
		Find(&sessions).Error
	return sessions, err
}

// UpdateExtractedInfo updates the extracted_info JSONB field of a session.
func (r *ConsultationRepository) UpdateExtractedInfo(ctx context.Context, conversationID uuid.UUID, extractedInfo any) error {
	return r.db.WithContext(ctx).
		Model(&model.ConsultationSession{}).
		Where("conversation_id = ?", conversationID).
		Update("extracted_info", extractedInfo).Error
}

// UpdatePhase updates the workflow phase of a consultation session.
func (r *ConsultationRepository) UpdatePhase(ctx context.Context, conversationID uuid.UUID, phase string) error {
	return r.db.WithContext(ctx).
		Model(&model.ConsultationSession{}).
		Where("conversation_id = ?", conversationID).
		Update("phase", phase).Error
}

// UpdateDiagnosis updates the diagnosis field of a session.
func (r *ConsultationRepository) UpdateDiagnosis(ctx context.Context, conversationID uuid.UUID, diagnosis any) error {
	return r.db.WithContext(ctx).
		Model(&model.ConsultationSession{}).
		Where("conversation_id = ?", conversationID).
		Update("diagnosis", diagnosis).Error
}

// UpdateTreatmentPlan updates the treatment_plan field of a session.
func (r *ConsultationRepository) UpdateTreatmentPlan(ctx context.Context, conversationID uuid.UUID, treatmentPlan any) error {
	return r.db.WithContext(ctx).
		Model(&model.ConsultationSession{}).
		Where("conversation_id = ?", conversationID).
		Update("treatment_plan", treatmentPlan).Error
}

// Delete removes a consultation session by conversation ID.
func (r *ConsultationRepository) Delete(ctx context.Context, conversationID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Delete(&model.ConsultationSession{}).Error
}

// ListByUserID retrieves consultation sessions for a user via the conversations join.
func (r *ConsultationRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]model.ConsultationSession, error) {
	var sessions []model.ConsultationSession
	err := r.db.WithContext(ctx).
		Joins("JOIN conversations ON conversations.id = consultation_sessions.conversation_id").
		Where("conversations.user_id = ? AND conversations.status = ? AND conversations.last_message_at IS NOT NULL", userID, "active").
		Order("consultation_sessions.created_at DESC").
		Find(&sessions).Error
	return sessions, err
}

// CreateRunEnvelope atomically resolves the conversation/session and creates the run turn envelope.
func (r *ConsultationRepository) CreateRunEnvelope(
	ctx context.Context,
	userID uuid.UUID,
	conversationID *uuid.UUID,
	requestID string,
	userParts datatypes.JSON,
	userMetadata datatypes.JSON,
	modelName string,
) (*model.ConsultationSession, *model.Run, *model.Message, *model.Message, uuid.UUID, bool, error) {
	var session *model.ConsultationSession
	var run *model.Run
	var userMsg *model.Message
	var assistantMsg *model.Message
	var turnID uuid.UUID
	var existed bool

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		resolvedConversationID, resolvedSession, err := r.resolveRunConversation(ctx, tx, userID, conversationID)
		if err != nil {
			return err
		}
		session = resolvedSession

		var userSeq int
		if err := tx.WithContext(ctx).
			Model(&model.Message{}).
			Where("conversation_id = ?", resolvedConversationID).
			Select("COALESCE(MAX(seq), 0) + 1").
			Scan(&userSeq).Error; err != nil {
			return err
		}

		turnID = uuid.New()
		run = &model.Run{
			ID:             uuid.New(),
			ConversationID: resolvedConversationID,
			TurnID:         turnID,
			RequestID:      requestID,
			UserID:         userID,
			Status:         "running",
			Model:          modelName,
		}

		result := tx.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "request_id"}},
			DoNothing: true,
		}).Create(run)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			existed = true
			return tx.WithContext(ctx).
				Where("user_id = ? AND request_id = ?", userID, requestID).
				First(run).Error
		}

		userMsg = &model.Message{
			ID:             uuid.New(),
			ConversationID: resolvedConversationID,
			TurnID:         turnID,
			Role:           "user",
			Status:         "completed",
			Seq:            userSeq,
			Parts:          userParts,
			Metadata:       userMetadata,
		}
		if err := tx.WithContext(ctx).Create(userMsg).Error; err != nil {
			return err
		}

		assistantMsg = &model.Message{
			ID:             uuid.New(),
			ConversationID: resolvedConversationID,
			TurnID:         turnID,
			RunID:          &run.ID,
			Role:           "assistant",
			Status:         "streaming",
			Seq:            userSeq + 1,
			Parts:          datatypes.JSON("[]"),
			Metadata:       datatypes.JSON("{}"),
		}
		if err := tx.WithContext(ctx).Create(assistantMsg).Error; err != nil {
			return err
		}

		lastMessageAt := userMsg.CreatedAt
		if lastMessageAt.IsZero() {
			lastMessageAt = time.Now()
		}
		return tx.WithContext(ctx).
			Model(&model.Conversation{}).
			Where("id = ? AND user_id = ? AND deleted_at IS NULL", resolvedConversationID, userID).
			Updates(map[string]any{
				"active_run_id":    run.ID,
				"active_stream_id": run.ID.String(),
				"last_message_at":  lastMessageAt,
			}).Error
	})
	if err != nil {
		return nil, nil, nil, nil, uuid.Nil, false, err
	}
	return session, run, userMsg, assistantMsg, turnID, existed, nil
}

func (r *ConsultationRepository) resolveRunConversation(
	ctx context.Context,
	tx *gorm.DB,
	userID uuid.UUID,
	conversationID *uuid.UUID,
) (uuid.UUID, *model.ConsultationSession, error) {
	if conversationID != nil && *conversationID != uuid.Nil {
		var conversation model.Conversation
		if err := tx.WithContext(ctx).
			Where("id = ? AND user_id = ? AND deleted_at IS NULL", *conversationID, userID).
			First(&conversation).Error; err != nil {
			return uuid.Nil, nil, err
		}

		var session model.ConsultationSession
		if err := tx.WithContext(ctx).
			Where("conversation_id = ?", conversation.ID).
			First(&session).Error; err != nil {
			return uuid.Nil, nil, err
		}
		return conversation.ID, &session, nil
	}

	var conversation model.Conversation
	err := tx.WithContext(ctx).
		Where("user_id = ? AND status = ? AND last_message_at IS NULL AND deleted_at IS NULL", userID, "active").
		Order("created_at DESC").
		First(&conversation).Error
	switch {
	case err == nil:
		var session model.ConsultationSession
		if err := tx.WithContext(ctx).
			Where("conversation_id = ?", conversation.ID).
			First(&session).Error; err == nil {
			return conversation.ID, &session, nil
		} else if err != gorm.ErrRecordNotFound {
			return uuid.Nil, nil, err
		}

		session = model.ConsultationSession{
			ConversationID: conversation.ID,
			ExtractedInfo:  datatypes.JSON("[]"),
			Phase:          "collecting",
		}
		if err := tx.WithContext(ctx).Create(&session).Error; err != nil {
			return uuid.Nil, nil, err
		}
		return conversation.ID, &session, nil

	case err != gorm.ErrRecordNotFound:
		return uuid.Nil, nil, err
	}

	conversation = model.Conversation{
		ID:          uuid.New(),
		UserID:      userID,
		Status:      "active",
		TitleStatus: "pending",
	}
	if err := tx.WithContext(ctx).Create(&conversation).Error; err != nil {
		return uuid.Nil, nil, err
	}

	session := model.ConsultationSession{
		ConversationID: conversation.ID,
		ExtractedInfo:  datatypes.JSON("[]"),
		Phase:          "collecting",
	}
	if err := tx.WithContext(ctx).Create(&session).Error; err != nil {
		return uuid.Nil, nil, err
	}
	return conversation.ID, &session, nil
}
