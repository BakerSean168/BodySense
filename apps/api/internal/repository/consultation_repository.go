package repository

import (
	"context"
	"errors"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ConsultationRepository handles database operations for consultation sessions.
type ConsultationRepository struct {
	db         *gorm.DB
	leaseOwner string
}

// NewConsultationRepository creates a new ConsultationRepository.
func NewConsultationRepository(db *gorm.DB, leaseOwners ...string) *ConsultationRepository {
	owner := uuid.NewString()
	if len(leaseOwners) > 0 && leaseOwners[0] != "" {
		owner = leaseOwners[0]
	}
	return &ConsultationRepository{db: db, leaseOwner: owner}
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

// UpdatePhase updates the workflow phase of a consultation session.
func (r *ConsultationRepository) UpdatePhase(ctx context.Context, conversationID uuid.UUID, phase string) error {
	return r.db.WithContext(ctx).
		Model(&model.ConsultationSession{}).
		Where("conversation_id = ?", conversationID).
		Update("phase", phase).Error
}

// Delete removes a consultation session by conversation ID.
func (r *ConsultationRepository) Delete(ctx context.Context, conversationID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Delete(&model.ConsultationSession{}).Error
}

// GetLatestByUserID resolves the canonical long-lived consultation for a user.
// Historical conversations are retained, but new product flows reuse the most
// recently active consultation instead of requiring users to create another one.
func (r *ConsultationRepository) GetLatestByUserID(ctx context.Context, userID uuid.UUID) (*model.ConsultationSession, error) {
	var session model.ConsultationSession
	err := r.db.WithContext(ctx).
		Joins("JOIN conversations ON conversations.id = consultation_sessions.conversation_id").
		Where("conversations.user_id = ? AND conversations.status = ? AND conversations.deleted_at IS NULL", userID, "active").
		Order("COALESCE(conversations.last_message_at, conversations.created_at) DESC").
		First(&session).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &session, nil
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
		// Serialize run creation per user. This also prevents two concurrent
		// conversation-less requests from creating separate "long-lived" sessions.
		var owner model.User
		if err := tx.WithContext(ctx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", userID).
			First(&owner).Error; err != nil {
			return err
		}
		resolvedConversationID, resolvedSession, err := r.resolveRunConversation(ctx, tx, userID, conversationID)
		if err != nil {
			return err
		}
		session = resolvedSession

		var lockedConversation model.Conversation
		if err := tx.WithContext(ctx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ? AND deleted_at IS NULL", resolvedConversationID, userID).
			First(&lockedConversation).Error; err != nil {
			return err
		}
		if lockedConversation.ActiveRunID != nil {
			if err := r.handleActiveRun(ctx, tx, resolvedConversationID, *lockedConversation.ActiveRunID); err != nil {
				return err
			}
		}

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
			LeaseOwner:     r.leaseOwner,
			LeaseExpiresAt: runLeaseExpiry(),
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

// runLeaseDuration bounds how long a running run may sit before it is
// reclaimed. Runtime heartbeats renew the lease via the SSE loop, whose
// timeout is well below this so live runs never expire their lease.
const runLeaseDuration = 30 * time.Minute

// runLeaseExpiry returns the lease deadline for a newly-created run.
func runLeaseExpiry() *time.Time {
	expires := time.Now().Add(runLeaseDuration)
	return &expires
}

// handleActiveRun inspects the conversation's active run and decides whether a
// new run may proceed. It returns nil when the pointer may be cleared and the
// caller may proceed (terminal run, or a stale running run whose lease has
// expired and is reclaimed as failed), or model.ErrConversationRunInProgress
// when a live run or a run blocked on user input is still in progress.
func (r *ConsultationRepository) handleActiveRun(
	ctx context.Context,
	tx *gorm.DB,
	conversationID uuid.UUID,
	activeRunID uuid.UUID,
) error {
	var active model.Run
	err := tx.WithContext(ctx).
		Where("id = ? AND conversation_id = ?", activeRunID, conversationID).
		First(&active).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Orphaned pointer: clear and proceed.
		return r.clearActiveRun(ctx, tx, conversationID)
	}

	switch active.Status {
	case "waiting_user":
		// Blocked on user input is not lease-bound; keep it in progress.
		return model.ErrConversationRunInProgress
	case "running":
		if active.LeaseExpiresAt == nil || time.Now().Before(*active.LeaseExpiresAt) {
			// Live run (or legacy run without a lease): keep it in progress.
			return model.ErrConversationRunInProgress
		}
		// Lease expired: assume the owning process died and reclaim the run.
		now := time.Now()
		if err := tx.WithContext(ctx).Model(&model.Run{}).
			Where("id = ?", activeRunID).
			Updates(map[string]any{
				"status":       "failed",
				"error":        datatypes.JSON(`{"message":"run lease expired; execution reclaimed"}`),
				"completed_at": now,
			}).Error; err != nil {
			return err
		}
		return r.clearActiveRun(ctx, tx, conversationID)
	default:
		// Terminal state: clear the pointer and proceed.
		return r.clearActiveRun(ctx, tx, conversationID)
	}
}

func (r *ConsultationRepository) clearActiveRun(
	ctx context.Context,
	tx *gorm.DB,
	conversationID uuid.UUID,
) error {
	return tx.WithContext(ctx).Model(&model.Conversation{}).
		Where("id = ?", conversationID).
		Updates(map[string]any{"active_run_id": nil, "active_stream_id": ""}).Error
}

func (r *ConsultationRepository) ensureConversationRunAvailable(
	ctx context.Context,
	tx *gorm.DB,
	conversation *model.Conversation,
) error {
	if conversation.ActiveRunID != nil {
		if err := r.handleActiveRun(ctx, tx, conversation.ID, *conversation.ActiveRunID); err != nil {
			return err
		}
		conversation.ActiveRunID = nil
		conversation.ActiveStreamID = ""
	}

	var pendingInteractions int64
	if err := tx.WithContext(ctx).Model(&model.AgentInteraction{}).
		Where("conversation_id = ? AND status = ?", conversation.ID, "pending").
		Count(&pendingInteractions).Error; err != nil {
		return err
	}
	if pendingInteractions > 0 {
		return model.ErrConversationRunInProgress
	}
	return nil
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
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ? AND deleted_at IS NULL", *conversationID, userID).
			First(&conversation).Error; err != nil {
			return uuid.Nil, nil, err
		}
		if err := r.ensureConversationRunAvailable(ctx, tx, &conversation); err != nil {
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
		Joins("JOIN consultation_sessions ON consultation_sessions.conversation_id = conversations.id").
		Where("conversations.user_id = ? AND conversations.status = ? AND conversations.deleted_at IS NULL", userID, "active").
		Order("COALESCE(conversations.last_message_at, conversations.created_at) DESC").
		First(&conversation).Error
	switch {
	case err == nil:
		if err := tx.WithContext(ctx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ? AND deleted_at IS NULL", conversation.ID, userID).
			First(&conversation).Error; err != nil {
			return uuid.Nil, nil, err
		}
		if err := r.ensureConversationRunAvailable(ctx, tx, &conversation); err != nil {
			return uuid.Nil, nil, err
		}
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
