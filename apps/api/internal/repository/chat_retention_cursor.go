package repository

import (
	"context"
	"fmt"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// liveMessages returns metadata of every message after the pruned cursor, in
// seq order, and fails closed on rows that cross owners or conversations.
func liveMessages(ctx context.Context, tx *gorm.DB, conversation *model.Conversation) ([]retentionMessage, error) {
	var rows []retentionMessage
	err := tx.WithContext(ctx).Raw(`
SELECT m.id, m.seq, m.turn_id, m.run_id, m.status,
       octet_length(COALESCE(m.content_text, '')) AS content_bytes,
       octet_length(m.parts::text) AS parts_bytes,
       r.status AS run_status, r.user_id AS run_user_id, r.conversation_id AS run_conversation_id,
       EXISTS (SELECT 1 FROM agent_interactions ai
               WHERE ai.run_id = m.run_id AND ai.status = 'pending') AS pending_interaction
FROM messages m
LEFT JOIN runs r ON r.id = m.run_id
WHERE m.conversation_id = ? AND m.seq > ?
ORDER BY m.seq`, conversation.ID, conversation.PrunedThroughSeq).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.RunID == nil {
			continue
		}
		if row.RunUserID == nil || *row.RunUserID != conversation.UserID ||
			row.RunConversationID == nil || *row.RunConversationID != conversation.ID {
			return nil, fmt.Errorf("%w: message %d bound to a foreign run", model.ErrRetentionOwnerAnomaly, row.Seq)
		}
	}
	return rows, nil
}

// turnBoundary reports whether no interaction (turn) has messages on both
// sides of seq b, i.e. b is a complete user/assistant/tool boundary.
func turnBoundary(messages []retentionMessage, b int) bool {
	below := map[uuid.UUID]bool{}
	for _, m := range messages {
		if m.Seq <= b {
			below[m.TurnID] = true
		}
	}
	for _, m := range messages {
		if m.Seq > b && below[m.TurnID] {
			return false
		}
	}
	return true
}

// earliestSpanningTurnStart returns the smallest seq of any turn that spans b.
func earliestSpanningTurnStart(messages []retentionMessage, b int) int {
	above := map[uuid.UUID]bool{}
	for _, m := range messages {
		if m.Seq > b {
			above[m.TurnID] = true
		}
	}
	for _, m := range messages {
		if m.Seq <= b && above[m.TurnID] {
			return m.Seq
		}
	}
	return b + 1
}

func (r *ChatRetentionRepository) advance(
	ctx context.Context,
	owner uuid.UUID,
	cmd model.AdvanceRetentionCursorCommand,
	current func(*model.Conversation) int,
	validate func(*model.Conversation, []retentionMessage) error,
	column string,
) (*model.ChatRetentionCursor, error) {
	var result *model.ChatRetentionCursor
	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		conversation, err := r.lock(ctx, tx, owner)
		if err != nil {
			return err
		}
		switch at := current(conversation); {
		case cmd.ThroughSeq < at:
			return model.ErrRetentionCursorRegression
		case cmd.ThroughSeq == at:
			// Already applied: an exact retry is an idempotent no-op.
			result = cursorOf(conversation)
			return nil
		}
		if cmd.ExpectedRevision != conversation.RetentionRevision {
			return model.ErrRetentionRevisionConflict
		}
		messages, err := liveMessages(ctx, tx, conversation)
		if err != nil {
			return err
		}
		if err := validate(conversation, messages); err != nil {
			return err
		}
		next := conversation.RetentionRevision + 1
		update := tx.WithContext(ctx).Model(&model.Conversation{}).
			Where("id = ? AND user_id = ? AND retention_revision = ?", conversation.ID, owner, conversation.RetentionRevision).
			Updates(map[string]any{column: cmd.ThroughSeq, "retention_revision": next, "updated_at": r.now()})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return model.ErrRetentionRevisionConflict
		}
		if err := tx.WithContext(ctx).Where("id = ?", conversation.ID).First(conversation).Error; err != nil {
			return err
		}
		result = cursorOf(conversation)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// AdvanceProcessed moves P forward to an existing message seq. Every message
// it passes must be settled and the target must close a complete interaction.
func (r *ChatRetentionRepository) AdvanceProcessed(ctx context.Context, owner uuid.UUID, cmd model.AdvanceRetentionCursorCommand) (*model.ChatRetentionCursor, error) {
	return r.advance(ctx, owner, cmd,
		func(c *model.Conversation) int { return c.ProcessedThroughSeq },
		func(c *model.Conversation, messages []retentionMessage) error {
			exists := false
			for _, m := range messages {
				if m.Seq <= c.ProcessedThroughSeq || m.Seq > cmd.ThroughSeq {
					continue
				}
				if !m.settled() {
					return fmt.Errorf("%w: message %d is not processed", model.ErrRetentionCursorInvalid, m.Seq)
				}
				exists = exists || m.Seq == cmd.ThroughSeq
			}
			if !exists {
				return fmt.Errorf("%w: no message at seq %d", model.ErrRetentionCursorInvalid, cmd.ThroughSeq)
			}
			if !turnBoundary(messages, cmd.ThroughSeq) {
				return fmt.Errorf("%w: seq %d splits an interaction", model.ErrRetentionCursorInvalid, cmd.ThroughSeq)
			}
			return nil
		}, "processed_through_seq")
}

// AdvanceContextStart moves C forward. Only processed messages may leave the
// AI context (C <= P + 1) and C must start a complete interaction that splits
// neither a turn nor a Run.
func (r *ChatRetentionRepository) AdvanceContextStart(ctx context.Context, owner uuid.UUID, cmd model.AdvanceRetentionCursorCommand) (*model.ChatRetentionCursor, error) {
	return r.advance(ctx, owner, cmd,
		func(c *model.Conversation) int { return c.ContextStartSeq },
		func(c *model.Conversation, messages []retentionMessage) error {
			if cmd.ThroughSeq > c.ProcessedThroughSeq+1 {
				return fmt.Errorf("%w: context would skip unprocessed messages", model.ErrRetentionCursorInvalid)
			}
			if !interactionBoundary(messages, cmd.ThroughSeq-1) {
				return fmt.Errorf("%w: seq %d splits an interaction", model.ErrRetentionCursorInvalid, cmd.ThroughSeq)
			}
			return nil
		}, "context_start_seq")
}
