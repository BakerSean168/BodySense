package repository

import (
	"context"
	"strings"
	"time"

	"github.com/bodysense/api/internal/database"
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
func (r *ConsultationRepository) UpdatePhase(ctx context.Context, conversationID uuid.UUID, phase model.ConsultationPhase) error {
	return database.FromContext(ctx, r.db).
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

// GetLatestByUserID returns the consultation session of the owner's single
// Conversation without creating it, or nil when the owner has not chatted yet.
func (r *ConsultationRepository) GetLatestByUserID(ctx context.Context, userID uuid.UUID) (*model.ConsultationSession, error) {
	var session model.ConsultationSession
	err := r.db.WithContext(ctx).
		Joins("JOIN conversations ON conversations.id = consultation_sessions.conversation_id").
		Where("conversations.user_id = ?", userID).
		First(&session).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &session, nil
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
		// Serialize run creation per user (PC-D5). The singleton Conversation and
		// idx_runs_one_active_per_user are the database backstops.
		var owner model.User
		if err := tx.WithContext(ctx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", userID).
			First(&owner).Error; err != nil {
			return err
		}
		// Request IDs cannot be rebound from an accepted answer to a new turn.
		// Owner locking serializes this check with ClaimInteractionResume.
		var reserved int64
		if err := tx.Model(&model.InteractionResume{}).Where("user_id = ? AND request_id = ?", userID, requestID).Count(&reserved).Error; err != nil {
			return err
		}
		if reserved != 0 {
			return model.ErrInteractionResumeConflict
		}
		resolvedConversationID, resolvedSession, err := r.resolveRunConversation(ctx, tx, userID, conversationID)
		if err != nil {
			return err
		}
		session = resolvedSession

		var lockedConversation model.Conversation
		if err := tx.WithContext(ctx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", resolvedConversationID, userID).
			First(&lockedConversation).Error; err != nil {
			return err
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
			if isActiveRunPerUserViolation(result.Error) {
				return model.ErrConversationRunInProgress
			}
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
			Where("id = ? AND user_id = ?", resolvedConversationID, userID).
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

// isActiveRunPerUserViolation reports a unique violation of
// idx_runs_one_active_per_user. Owner locking normally prevents it; the index
// is the fail-closed backstop for any path that bypasses the lock.
func isActiveRunPerUserViolation(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "idx_runs_one_active_per_user") && strings.Contains(message, "23505")
}

// ensureUserRunAvailable enforces one active Run per user (PC-D5) before a new
// Run is admitted. A live running Run, a Run blocked on the user
// (waiting_user) or a pending Ask User interaction keeps the user busy and
// returns model.ErrConversationRunInProgress. A running Run whose lease has
// expired is reclaimed as failed. The Conversation's active-run pointer is
// cleared once no Run is in progress.
func (r *ConsultationRepository) ensureUserRunAvailable(
	ctx context.Context,
	tx *gorm.DB,
	conversation *model.Conversation,
) error {
	var active []model.Run
	if err := tx.WithContext(ctx).
		Where("user_id = ? AND status IN ?", conversation.UserID, []string{"running", "waiting_user"}).
		Find(&active).Error; err != nil {
		return err
	}
	now := time.Now()
	for _, run := range active {
		switch run.Status {
		case "waiting_user":
			// Blocked on user input is not lease-bound; keep it in progress.
			return model.ErrConversationRunInProgress
		case "running":
			if run.LeaseExpiresAt != nil && now.Before(*run.LeaseExpiresAt) {
				return model.ErrConversationRunInProgress
			}
			// A current running run must own a lease. A missing or expired lease is
			// non-live state and is reclaimed rather than treated as indefinitely active.
			if err := tx.WithContext(ctx).Model(&model.Run{}).
				Where("id = ? AND user_id = ? AND status = ?", run.ID, conversation.UserID, "running").
				Updates(map[string]any{
					"status":       "failed",
					"error":        datatypes.JSON(`{"message":"run lease expired; execution reclaimed"}`),
					"completed_at": now,
				}).Error; err != nil {
				return err
			}
		}
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

	if conversation.ActiveRunID != nil || conversation.ActiveStreamID != "" {
		if err := tx.WithContext(ctx).Model(&model.Conversation{}).
			Where("id = ? AND user_id = ?", conversation.ID, conversation.UserID).
			Updates(map[string]any{"active_run_id": nil, "active_stream_id": ""}).Error; err != nil {
			return err
		}
		conversation.ActiveRunID = nil
		conversation.ActiveStreamID = ""
	}
	return nil
}

// resolveRunConversation resolves (lazily creating) the owner's single
// Conversation and locks it. A caller-supplied conversation ID must be that
// Conversation; any other ID is owner-masked as not found.
func (r *ConsultationRepository) resolveRunConversation(
	ctx context.Context,
	tx *gorm.DB,
	userID uuid.UUID,
	conversationID *uuid.UUID,
) (uuid.UUID, *model.ConsultationSession, error) {
	singleton, session, _, err := getOrCreateSingletonConversation(ctx, tx, userID)
	if err != nil {
		return uuid.Nil, nil, err
	}
	if conversationID != nil && *conversationID != uuid.Nil && *conversationID != singleton.ID {
		return uuid.Nil, nil, gorm.ErrRecordNotFound
	}
	var conversation model.Conversation
	if err := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND user_id = ?", singleton.ID, userID).
		First(&conversation).Error; err != nil {
		return uuid.Nil, nil, err
	}
	if err := r.ensureUserRunAvailable(ctx, tx, &conversation); err != nil {
		return uuid.Nil, nil, err
	}
	return conversation.ID, session, nil
}
