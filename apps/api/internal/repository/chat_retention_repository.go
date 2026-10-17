package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
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

// liveMessages returns metadata of every message after the pruned cursor, in
// seq order, and fails closed on rows that cross owners or conversations.
func liveMessages(ctx context.Context, tx *gorm.DB, conversation *model.Conversation) ([]retentionMessage, error) {
	var rows []retentionMessage
	err := tx.WithContext(ctx).Raw(`
SELECT m.id, m.seq, m.turn_id, m.run_id, m.status,
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
// AI context (C <= P + 1) and C must start a complete interaction.
func (r *ChatRetentionRepository) AdvanceContextStart(ctx context.Context, owner uuid.UUID, cmd model.AdvanceRetentionCursorCommand) (*model.ChatRetentionCursor, error) {
	return r.advance(ctx, owner, cmd,
		func(c *model.Conversation) int { return c.ContextStartSeq },
		func(c *model.Conversation, messages []retentionMessage) error {
			if cmd.ThroughSeq > c.ProcessedThroughSeq+1 {
				return fmt.Errorf("%w: context would skip unprocessed messages", model.ErrRetentionCursorInvalid)
			}
			if !turnBoundary(messages, cmd.ThroughSeq-1) {
				return fmt.Errorf("%w: seq %d splits an interaction", model.ErrRetentionCursorInvalid, cmd.ThroughSeq)
			}
			return nil
		}, "context_start_seq")
}

func pruneJobKey(conversationID uuid.UUID, target int) string {
	return fmt.Sprintf("%s:%s:%d", model.ChatRetentionPruneJobType, conversationID, target)
}

// protectedMessages returns live messages in (from, through] that a durable
// record still references: BodyFact/Safety statement evidence, a Diagnosis
// request receipt, a Safety screen or lift, or an accepted Ask User answer.
// A reference held by another owner is an anomaly and fails the pass.
func protectedMessages(ctx context.Context, tx *gorm.DB, conversation *model.Conversation, from, through int) (map[uuid.UUID]bool, error) {
	var refs []struct {
		MessageID uuid.UUID
		RefUserID uuid.UUID
	}
	err := tx.WithContext(ctx).Raw(`
WITH window_messages AS (
  SELECT id FROM messages WHERE conversation_id = ? AND seq > ? AND seq <= ?
)
SELECT e.source_message_id AS message_id, e.user_id AS ref_user_id
  FROM statement_evidence e JOIN window_messages w ON w.id = e.source_message_id
UNION ALL
SELECT c.request_message_id, c.user_id
  FROM condition_analysis_runs c JOIN window_messages w ON w.id = c.request_message_id
UNION ALL
SELECT s.source_message_id, s.user_id
  FROM safety_screens s JOIN window_messages w ON w.id = s.source_message_id
UNION ALL
SELECT s.source_message_id, s.user_id
  FROM safety_resolutions s JOIN window_messages w ON w.id = s.source_message_id
UNION ALL
SELECT ir.user_message_id, ir.user_id
  FROM interaction_resumes ir JOIN window_messages w ON w.id = ir.user_message_id
UNION ALL
SELECT ir.assistant_message_id, ir.user_id
  FROM interaction_resumes ir JOIN window_messages w ON w.id = ir.assistant_message_id`,
		conversation.ID, from, through).Scan(&refs).Error
	if err != nil {
		return nil, err
	}
	protected := map[uuid.UUID]bool{}
	for _, ref := range refs {
		if ref.RefUserID != conversation.UserID {
			return nil, fmt.Errorf("%w: message referenced by another owner", model.ErrRetentionOwnerAnomaly)
		}
		protected[ref.MessageID] = true
	}
	return protected, nil
}

// referencedRuns returns runs whose identity durable evidence still names.
func referencedRuns(ctx context.Context, tx *gorm.DB, owner uuid.UUID, candidates []uuid.UUID) (map[uuid.UUID]bool, error) {
	out := map[uuid.UUID]bool{}
	if len(candidates) == 0 {
		return out, nil
	}
	var refs []struct {
		RunID     uuid.UUID
		RefUserID uuid.UUID
	}
	err := tx.WithContext(ctx).Raw(`
SELECT source_run_id AS run_id, user_id AS ref_user_id FROM statement_evidence WHERE source_run_id IN ?
UNION ALL
SELECT source_run_id, user_id FROM safety_screens WHERE source_run_id IN ?
UNION ALL
SELECT run_id, user_id FROM interaction_resumes WHERE run_id IN ?`,
		candidates, candidates, candidates).Scan(&refs).Error
	if err != nil {
		return nil, err
	}
	for _, ref := range refs {
		if ref.RefUserID != owner {
			return nil, fmt.Errorf("%w: run referenced by another owner", model.ErrRetentionOwnerAnomaly)
		}
		out[ref.RunID] = true
	}
	return out, nil
}

// replayPrune returns the receipt of the pass that produced the current
// pruned cursor, so a duplicate or concurrent request is answered once.
func replayPrune(ctx context.Context, tx *gorm.DB, conversation *model.Conversation) (*model.ChatRetentionPruneReceipt, error) {
	receipt := &model.ChatRetentionPruneReceipt{
		ConversationID:    conversation.ID,
		FromPrunedSeq:     conversation.PrunedThroughSeq,
		PrunedThroughSeq:  conversation.PrunedThroughSeq,
		RetentionRevision: conversation.RetentionRevision,
		CheckpointCleanup: "not_run",
		Replayed:          true,
	}
	if conversation.PrunedThroughSeq == 0 {
		return receipt, nil
	}
	var job model.Job
	err := tx.WithContext(ctx).Where("user_id = ? AND job_type = ? AND idempotency_key = ?",
		conversation.UserID, model.ChatRetentionPruneJobType, pruneJobKey(conversation.ID, conversation.PrunedThroughSeq)).
		First(&job).Error
	if err != nil {
		return nil, fmt.Errorf("prune receipt for pruned cursor: %w", err)
	}
	if err := json.Unmarshal(job.Result, receipt); err != nil {
		return nil, err
	}
	receipt.Replayed = true
	return receipt, nil
}

// Prune physically deletes raw messages and their controlled runtime copies
// behind the safe boundary in one transaction, then advances R and records a
// completed `chat.retention.prune` job as the receipt.
//
// The boundary is min(C - 1, last seq - 1), cut back before the first message
// of any active/waiting Run or pending Ask User and down to a complete
// interaction boundary. The newest message is never pruned so MAX(seq)+1 seq
// allocation can never reuse a pruned seq. Messages a durable record still
// references are retained in place; their Runs keep their event logs.
func (r *ChatRetentionRepository) Prune(ctx context.Context, owner uuid.UUID) (*model.ChatRetentionPruneReceipt, error) {
	var receipt *model.ChatRetentionPruneReceipt
	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		conversation, err := r.lock(ctx, tx, owner)
		if err != nil {
			return err
		}
		messages, err := liveMessages(ctx, tx, conversation)
		if err != nil {
			return err
		}
		from := conversation.PrunedThroughSeq
		target := conversation.ContextStartSeq - 1
		if target > conversation.ProcessedThroughSeq {
			target = conversation.ProcessedThroughSeq
		}
		if len(messages) == 0 {
			target = from
		} else if last := messages[len(messages)-1].Seq; target > last-1 {
			target = last - 1
		}
		for _, m := range messages {
			if !m.settled() {
				// An active/waiting Run or pending Ask User blocks pruning past it.
				if m.Seq-1 < target {
					target = m.Seq - 1
				}
				break
			}
		}
		for target > from && !turnBoundary(messages, target) {
			target = earliestSpanningTurnStart(messages, target) - 1
		}
		if target <= from {
			receipt, err = replayPrune(ctx, tx, conversation)
			return err
		}

		protected, err := protectedMessages(ctx, tx, conversation, from, target)
		if err != nil {
			return err
		}
		runSet := map[uuid.UUID]bool{}
		heldRuns := map[uuid.UUID]bool{}
		for _, m := range messages {
			if m.RunID == nil {
				continue
			}
			if m.Seq > target || protected[m.ID] || !m.settled() {
				heldRuns[*m.RunID] = true
				continue
			}
			runSet[*m.RunID] = true
		}
		candidates := make([]uuid.UUID, 0, len(runSet))
		for id := range runSet {
			if !heldRuns[id] {
				candidates = append(candidates, id)
			}
		}
		referenced, err := referencedRuns(ctx, tx, owner, candidates)
		if err != nil {
			return err
		}
		runs := make([]uuid.UUID, 0, len(candidates))
		for _, id := range candidates {
			if !referenced[id] {
				runs = append(runs, id)
			}
		}
		keep := make([]uuid.UUID, 0, len(protected))
		for id := range protected {
			keep = append(keep, id)
		}

		var deletedEvents, deletedToolCalls int64
		if len(runs) > 0 {
			if err := tx.WithContext(ctx).Exec(
				"DELETE FROM thread_projection_tool_calls WHERE conversation_id = ? AND run_id IN ?", conversation.ID, runs).Error; err != nil {
				return err
			}
			res := tx.WithContext(ctx).Exec("DELETE FROM agent_tool_calls WHERE conversation_id = ? AND run_id IN ?", conversation.ID, runs)
			if res.Error != nil {
				return res.Error
			}
			deletedToolCalls = res.RowsAffected
			res = tx.WithContext(ctx).Exec("DELETE FROM runtime_events WHERE conversation_id = ? AND run_id IN ?", conversation.ID, runs)
			if res.Error != nil {
				return res.Error
			}
			deletedEvents = res.RowsAffected
		}
		del := tx.WithContext(ctx).Where("conversation_id = ? AND seq > ? AND seq <= ?", conversation.ID, from, target)
		if len(keep) > 0 {
			del = del.Where("id NOT IN ?", keep)
		}
		deleted := del.Delete(&model.Message{})
		if deleted.Error != nil {
			return deleted.Error
		}

		next := conversation.RetentionRevision + 1
		update := tx.WithContext(ctx).Model(&model.Conversation{}).
			Where("id = ? AND user_id = ? AND retention_revision = ?", conversation.ID, owner, conversation.RetentionRevision).
			Updates(map[string]any{"pruned_through_seq": target, "retention_revision": next, "updated_at": r.now()})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return model.ErrRetentionRevisionConflict
		}

		now := r.now()
		jobID := uuid.New()
		receipt = &model.ChatRetentionPruneReceipt{
			JobID: jobID, ConversationID: conversation.ID, FromPrunedSeq: from, PrunedThroughSeq: target,
			RetentionRevision: next, DeletedMessages: deleted.RowsAffected, RetainedMessages: len(keep),
			PrunedRuns: len(runs), DeletedRuntimeEvents: deletedEvents, DeletedToolCalls: deletedToolCalls,
			CheckpointCleanup: "not_run",
		}
		result, err := json.Marshal(receipt)
		if err != nil {
			return err
		}
		input, err := json.Marshal(map[string]any{"fromPrunedSeq": from, "targetSeq": target, "expectedRevision": conversation.RetentionRevision})
		if err != nil {
			return err
		}
		key := pruneJobKey(conversation.ID, target)
		convID := conversation.ID
		job := model.Job{
			ID: jobID, ConversationID: &convID, UserID: owner, JobType: model.ChatRetentionPruneJobType,
			Status: model.JobStatusCompleted, Input: datatypes.JSON(input), Result: datatypes.JSON(result),
			IdempotencyKey: &key, Attempts: 1, MaxAttempts: 1, StartedAt: &now, FinishedAt: &now,
			Metadata: datatypes.JSON(`{}`),
		}
		if err := tx.WithContext(ctx).Create(&job).Error; err != nil {
			return err
		}
		return tx.WithContext(ctx).Create(&model.JobEvent{
			JobID: jobID, EventType: "job.completed", Payload: datatypes.JSON(result),
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return receipt, nil
}
