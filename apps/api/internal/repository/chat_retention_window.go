package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// runBoundary reports whether no Run has messages on both sides of seq b. A
// Run is the unit LangGraph keeps in runtime_messages, so the context cursor
// must never split one even where its messages span several turns.
func runBoundary(messages []retentionMessage, b int) bool {
	below := map[uuid.UUID]bool{}
	for _, m := range messages {
		if m.RunID != nil && m.Seq <= b {
			below[*m.RunID] = true
		}
	}
	for _, m := range messages {
		if m.RunID != nil && m.Seq > b && below[*m.RunID] {
			return false
		}
	}
	return true
}

// interactionBoundary is a complete turn and Run boundary after seq b.
func interactionBoundary(messages []retentionMessage, b int) bool {
	return turnBoundary(messages, b) && runBoundary(messages, b)
}

// planRetentionWindow computes where P and C should be now.
//
// P moves to the last complete interaction boundary that every message
// before it is settled for; it stops at the first unsettled message (a live
// Run, a pending Ask User or a still-streaming reply).
//
// C moves forward, one complete interaction at a time, while the raw
// messages from C on are estimated above budget. It never passes P + 1, so it
// never leaves an unsettled Run behind, and it never moves backwards. It also
// never passes the start of the newest interaction: when that interaction (or
// the unprocessed tail) alone is above budget, C stops in front of it.
func planRetentionWindow(conversation *model.Conversation, messages []retentionMessage, budget int) (processed, contextStart int) {
	processed = conversation.ProcessedThroughSeq
	for _, m := range messages {
		if m.Seq <= conversation.ProcessedThroughSeq {
			continue
		}
		if !m.settled() {
			break
		}
		if interactionBoundary(messages, m.Seq) {
			processed = m.Seq
		}
	}

	// newest is where the last interaction of the transcript starts. It is
	// always kept whole, so the next turn still sees the exchange it follows
	// even when that exchange alone is above budget.
	newest := conversation.PrunedThroughSeq + 1
	for i, m := range messages {
		if i < len(messages)-1 && interactionBoundary(messages, m.Seq) {
			newest = m.Seq + 1
		}
	}

	contextStart = conversation.ContextStartSeq
	retained := 0
	for _, m := range messages {
		if m.Seq >= contextStart {
			retained += m.tokens()
		}
	}
	// leaving counts the interaction being walked; it leaves the context only
	// once its closing boundary is reached.
	leaving := 0
	for _, m := range messages {
		if m.Seq < conversation.ContextStartSeq {
			continue
		}
		if retained <= budget || m.Seq > processed || m.Seq >= newest {
			break
		}
		leaving += m.tokens()
		if interactionBoundary(messages, m.Seq) {
			retained -= leaving
			leaving = 0
			contextStart = m.Seq + 1
		}
	}
	return processed, contextStart
}

// ReconcileWindow moves P and C to where the settled transcript and the token
// budget put them, in one revision under the Conversation lock. It is safe to
// call on every Run settlement: a call that changes nothing writes nothing.
// moved reports whether a cursor changed.
func (r *ChatRetentionRepository) ReconcileWindow(ctx context.Context, owner uuid.UUID, budget int) (cursor *model.ChatRetentionCursor, moved bool, err error) {
	if budget < model.MinChatContextTokenBudget {
		return nil, false, fmt.Errorf("%w: context token budget %d", model.ErrRetentionCursorInvalid, budget)
	}
	err = database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		conversation, err := r.lock(ctx, tx, owner)
		if err != nil {
			return err
		}
		messages, err := liveMessages(ctx, tx, conversation)
		if err != nil {
			return err
		}
		processed, contextStart := planRetentionWindow(conversation, messages, budget)
		if processed == conversation.ProcessedThroughSeq && contextStart == conversation.ContextStartSeq {
			cursor = cursorOf(conversation)
			return nil
		}
		if processed < conversation.ProcessedThroughSeq || contextStart < conversation.ContextStartSeq ||
			!model.ValidRetentionOrder(processed, contextStart, conversation.PrunedThroughSeq) {
			return fmt.Errorf("%w: planned window P=%d C=%d", model.ErrRetentionCursorInvalid, processed, contextStart)
		}
		next := conversation.RetentionRevision + 1
		update := tx.WithContext(ctx).Model(&model.Conversation{}).
			Where("id = ? AND user_id = ? AND retention_revision = ?", conversation.ID, owner, conversation.RetentionRevision).
			Updates(map[string]any{
				"processed_through_seq": processed,
				"context_start_seq":     contextStart,
				"retention_revision":    next,
				"updated_at":            r.now(),
			})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return model.ErrRetentionRevisionConflict
		}
		if err := tx.WithContext(ctx).Where("id = ?", conversation.ID).First(conversation).Error; err != nil {
			return err
		}
		cursor, moved = cursorOf(conversation), true
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return cursor, moved, nil
}

// ContextWindow returns the raw-history window of the owner's Conversation
// for the AI runtime: C and the Runs owning messages at or after it. A
// conversation that is not the owner's, or a message bound to another
// owner's Run, fails closed.
func (r *ChatRetentionRepository) ContextWindow(ctx context.Context, owner, conversationID uuid.UUID) (*model.ChatContextWindow, error) {
	if owner == uuid.Nil || conversationID == uuid.Nil {
		return nil, model.ErrRetentionNotFound
	}
	db := database.FromContext(ctx, r.db).WithContext(ctx)
	var conversation model.Conversation
	err := db.Where("id = ? AND user_id = ?", conversationID, owner).First(&conversation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, model.ErrRetentionNotFound
	}
	if err != nil {
		return nil, err
	}
	var rows []struct {
		RunID     uuid.UUID
		RunUserID uuid.UUID
		RunConvID uuid.UUID
	}
	err = db.Raw(`
SELECT m.run_id, r.user_id AS run_user_id, r.conversation_id AS run_conv_id
FROM messages m JOIN runs r ON r.id = m.run_id
WHERE m.conversation_id = ? AND m.seq >= ?
GROUP BY m.run_id, r.user_id, r.conversation_id
ORDER BY MIN(m.seq)`, conversation.ID, conversation.ContextStartSeq).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	window := &model.ChatContextWindow{ContextStartSeq: conversation.ContextStartSeq, RunIDs: make([]uuid.UUID, 0, len(rows))}
	for _, row := range rows {
		if row.RunUserID != owner || row.RunConvID != conversation.ID {
			return nil, fmt.Errorf("%w: context message bound to a foreign run", model.ErrRetentionOwnerAnomaly)
		}
		window.RunIDs = append(window.RunIDs, row.RunID)
	}
	return window, nil
}
