package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ChatRetentionRepository owns the three retention cursors of the singleton
// Conversation and the controlled prune pass behind them (PC-RET-001). Every
// command locks the owner's Conversation row, which also serializes it with
// turn creation and Ask User resumes (they lock the same row before
// allocating a message seq), so pruning never races a new message.
type ChatRetentionRepository struct {
	db  *gorm.DB
	now func() time.Time
}

func NewChatRetentionRepository(db *gorm.DB) *ChatRetentionRepository {
	return &ChatRetentionRepository{db: db, now: time.Now}
}

// retentionMessage is the metadata the cursor rules need; no message text is
// ever read by retention.
type retentionMessage struct {
	ID                 uuid.UUID
	Seq                int
	TurnID             uuid.UUID
	RunID              *uuid.UUID
	Status             string
	RunStatus          *string
	RunUserID          *uuid.UUID
	RunConversationID  *uuid.UUID
	PendingInteraction bool
	// ContentBytes and PartsBytes are octet lengths only; the token estimate
	// is derived from them (model.EstimateChatTokens), never from the text.
	ContentBytes int
	PartsBytes   int
}

func (m retentionMessage) tokens() int {
	return model.EstimateChatTokens(m.ContentBytes, m.PartsBytes)
}

// settled reports whether semantic processing of the message is finished:
// its Run (if any) is terminal and no Ask User is still waiting on it.
func (m retentionMessage) settled() bool {
	if m.Status == "streaming" || m.PendingInteraction {
		return false
	}
	if m.RunID == nil {
		return true
	}
	if m.RunStatus == nil {
		return false
	}
	switch model.RunStatus(*m.RunStatus) {
	case model.RunStatusCompleted, model.RunStatusFailed, model.RunStatusCancelled:
		return true
	default:
		return false
	}
}

func cursorOf(c *model.Conversation) *model.ChatRetentionCursor {
	return &model.ChatRetentionCursor{
		ConversationID:      c.ID,
		ProcessedThroughSeq: c.ProcessedThroughSeq,
		ContextStartSeq:     c.ContextStartSeq,
		PrunedThroughSeq:    c.PrunedThroughSeq,
		RetentionRevision:   c.RetentionRevision,
	}
}

// Get returns the owner's retention cursor, or ErrRetentionNotFound.
func (r *ChatRetentionRepository) Get(ctx context.Context, owner uuid.UUID) (*model.ChatRetentionCursor, error) {
	if owner == uuid.Nil {
		return nil, model.ErrRetentionNotFound
	}
	var conversation model.Conversation
	err := database.FromContext(ctx, r.db).Where("user_id = ?", owner).First(&conversation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, model.ErrRetentionNotFound
	}
	if err != nil {
		return nil, err
	}
	return cursorOf(&conversation), nil
}

// OwnerOf returns the owner of a Conversation, for triggers that only carry
// the Conversation id.
func (r *ChatRetentionRepository) OwnerOf(ctx context.Context, conversationID uuid.UUID) (uuid.UUID, error) {
	if conversationID == uuid.Nil {
		return uuid.Nil, model.ErrRetentionNotFound
	}
	var conversation model.Conversation
	err := database.FromContext(ctx, r.db).Select("id", "user_id").Where("id = ?", conversationID).First(&conversation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return uuid.Nil, model.ErrRetentionNotFound
	}
	if err != nil {
		return uuid.Nil, err
	}
	return conversation.UserID, nil
}

func (r *ChatRetentionRepository) lock(ctx context.Context, tx *gorm.DB, owner uuid.UUID) (*model.Conversation, error) {
	if owner == uuid.Nil {
		return nil, model.ErrRetentionNotFound
	}
	var conversation model.Conversation
	err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "NO KEY UPDATE"}).
		Where("user_id = ?", owner).First(&conversation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, model.ErrRetentionNotFound
	}
	if err != nil {
		return nil, err
	}
	if !model.ValidRetentionOrder(conversation.ProcessedThroughSeq, conversation.ContextStartSeq, conversation.PrunedThroughSeq) {
		return nil, fmt.Errorf("%w: stored cursors out of order", model.ErrRetentionCursorInvalid)
	}
	return &conversation, nil
}
