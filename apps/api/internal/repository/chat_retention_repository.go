package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
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

// PruneJobKey is the idempotency key of a prune job: one per pruned cursor
// position and trigger, so a new trigger (say, another settled Run) can ask
// again while the cursor has not moved. Concurrent jobs are safe: the commit
// compare-and-sets the cursor and later passes replay its receipt.
func PruneJobKey(conversationID uuid.UUID, from int, trigger string) string {
	return fmt.Sprintf("%s:%s:%d:%s", model.ChatRetentionPruneJobType, conversationID, from, trigger)
}

// citedInteractionsSQL lists every Ask User interaction a durable record
// cites, with the citing owner: a structured-answer statement evidence (the
// BodyFact/Observation/Hypothesis funnel), a Safety intake screen, or a Safety
// reverification. Only these interactions and their resume receipts outlive a
// prune; every other interaction of a pruned window is raw chat and goes.
const citedInteractionsSQL = `
SELECT source_interaction_id AS interaction_id, user_id FROM statement_evidence WHERE source_interaction_id IS NOT NULL
UNION ALL
SELECT source_interaction_id, user_id FROM safety_screens
UNION ALL
SELECT check_interaction_id, user_id FROM safety_resolutions WHERE check_interaction_id IS NOT NULL`

// protectedMessages returns live messages in (from, through] that a durable
// record still cites. The full scope and its proof are documented in
// docs/security/chat-retention-evidence-scope.md: BodyFact/Observation/
// Hypothesis/Safety statement evidence (which every Diagnosis, BodyState and
// Plan citation funnels through), a Diagnosis analysis or direct-edit request,
// a Safety screen or resolution, and both messages of a cited Ask User resume.
// A reference held by another owner is an anomaly and fails the pass.
func protectedMessages(ctx context.Context, tx *gorm.DB, conversation *model.Conversation, from, through int) (map[uuid.UUID]bool, error) {
	var refs []struct {
		MessageID uuid.UUID
		RefUserID uuid.UUID
	}
	err := tx.WithContext(ctx).Raw(`
WITH window_messages AS (
  SELECT id FROM messages WHERE conversation_id = ? AND seq > ? AND seq <= ?
), cited AS (`+citedInteractionsSQL+`
)
SELECT e.source_message_id AS message_id, e.user_id AS ref_user_id
  FROM statement_evidence e JOIN window_messages w ON w.id = e.source_message_id
UNION ALL
SELECT c.request_message_id, c.user_id
  FROM condition_analysis_runs c JOIN window_messages w ON w.id = c.request_message_id
UNION ALL
SELECT c.request_message_id, c.user_id
  FROM condition_direct_edits c JOIN window_messages w ON w.id = c.request_message_id
UNION ALL
SELECT s.source_message_id, s.user_id
  FROM safety_screens s JOIN window_messages w ON w.id = s.source_message_id
UNION ALL
SELECT s.source_message_id, s.user_id
  FROM safety_resolutions s JOIN window_messages w ON w.id = s.source_message_id
UNION ALL
SELECT ir.user_message_id, cited.user_id
  FROM interaction_resumes ir JOIN window_messages w ON w.id = ir.user_message_id
  JOIN cited ON cited.interaction_id = ir.interaction_id
UNION ALL
SELECT ir.assistant_message_id, cited.user_id
  FROM interaction_resumes ir JOIN window_messages w ON w.id = ir.assistant_message_id
  JOIN cited ON cited.interaction_id = ir.interaction_id`,
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

// referencedRuns returns runs whose identity durable evidence still names:
// statement evidence, a Safety screen, or a cited Ask User interaction.
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
SELECT ai.run_id, cited.user_id
  FROM agent_interactions ai JOIN (`+citedInteractionsSQL+`) cited ON cited.interaction_id = ai.id
 WHERE ai.run_id IN ?`,
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
		CheckpointCleanup: "not_needed",
		Replayed:          true,
	}
	if conversation.PrunedThroughSeq == 0 {
		return receipt, nil
	}
	var job model.Job
	err := tx.WithContext(ctx).
		Where("user_id = ? AND conversation_id = ? AND job_type = ? AND status = ? AND (result->>'prunedThroughSeq')::int = ?",
			conversation.UserID, conversation.ID, model.ChatRetentionPruneJobType, model.JobStatusCompleted, conversation.PrunedThroughSeq).
		Order("finished_at DESC").First(&job).Error
	if err != nil {
		return nil, fmt.Errorf("prune receipt for pruned cursor: %w", err)
	}
	if err := json.Unmarshal(job.Result, receipt); err != nil {
		return nil, err
	}
	receipt.Replayed = true
	return receipt, nil
}

// pruneWindow is what one pass would remove, computed from live metadata.
type pruneWindow struct {
	from, target, lastSeq int
	busy                  bool
	messages              []retentionMessage
	// runIDs are every Run whose messages all sit at or before target.
	runIDs []uuid.UUID
}

// computePruneWindow picks the boundary min(C - 1, P, last seq - 1), cut down
// to a complete interaction boundary. The newest message is never pruned so
// MAX(seq)+1 seq allocation can never reuse a pruned seq.
func computePruneWindow(conversation *model.Conversation, messages []retentionMessage) pruneWindow {
	w := pruneWindow{from: conversation.PrunedThroughSeq, messages: messages}
	w.target = min(conversation.ContextStartSeq-1, conversation.ProcessedThroughSeq)
	if len(messages) == 0 {
		w.target = w.from
	} else {
		w.lastSeq = messages[len(messages)-1].Seq
		w.target = min(w.target, w.lastSeq-1)
	}
	for _, m := range messages {
		if !m.settled() {
			// Never past an unsettled message, even with forced cursors.
			w.busy = true
			w.target = min(w.target, m.Seq-1)
			break
		}
	}
	for w.target > w.from && !turnBoundary(messages, w.target) {
		w.target = earliestSpanningTurnStart(messages, w.target) - 1
	}
	behind, ahead := map[uuid.UUID]bool{}, map[uuid.UUID]bool{}
	for _, m := range messages {
		if m.RunID == nil {
			continue
		}
		if m.Seq <= w.target {
			behind[*m.RunID] = true
		} else {
			ahead[*m.RunID] = true
		}
	}
	for id := range behind {
		if !ahead[id] {
			w.runIDs = append(w.runIDs, id)
		}
	}
	slices.SortFunc(w.runIDs, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
	return w
}

// pruneDeletion is the exact row set a pass deletes inside its window.
type pruneDeletion struct {
	keep []uuid.UUID // referenced messages retained in place
	runs []uuid.UUID // Runs whose event logs and tool calls go
}

func planDeletion(ctx context.Context, tx *gorm.DB, conversation *model.Conversation, w pruneWindow) (pruneDeletion, error) {
	protected, err := protectedMessages(ctx, tx, conversation, w.from, w.target)
	if err != nil {
		return pruneDeletion{}, err
	}
	held := map[uuid.UUID]bool{}
	for _, m := range w.messages {
		if m.RunID != nil && m.Seq <= w.target && protected[m.ID] {
			held[*m.RunID] = true
		}
	}
	candidates := make([]uuid.UUID, 0, len(w.runIDs))
	for _, id := range w.runIDs {
		if !held[id] {
			candidates = append(candidates, id)
		}
	}
	referenced, err := referencedRuns(ctx, tx, conversation.UserID, candidates)
	if err != nil {
		return pruneDeletion{}, err
	}
	var d pruneDeletion
	for _, id := range candidates {
		if !referenced[id] {
			d.runs = append(d.runs, id)
		}
	}
	for id := range protected {
		d.keep = append(d.keep, id)
	}
	return d, nil
}

// PlanPrune computes the next prune pass without deleting anything. It returns
// a replay receipt instead when there is nothing left to prune, and
// ErrRetentionBusy while any Run is unsettled: its LangGraph thread is mid-step
// or waiting on Ask User, so its checkpoint must not be rewritten.
func (r *ChatRetentionRepository) PlanPrune(ctx context.Context, owner uuid.UUID) (*model.ChatRetentionPrunePlan, *model.ChatRetentionPruneReceipt, error) {
	var plan *model.ChatRetentionPrunePlan
	var replay *model.ChatRetentionPruneReceipt
	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		conversation, err := r.lock(ctx, tx, owner)
		if err != nil {
			return err
		}
		messages, err := liveMessages(ctx, tx, conversation)
		if err != nil {
			return err
		}
		w := computePruneWindow(conversation, messages)
		if w.target <= w.from {
			replay, err = replayPrune(ctx, tx, conversation)
			return err
		}
		if w.busy {
			return model.ErrRetentionBusy
		}
		// Surface owner anomalies before anything outside Go is touched.
		if _, err := planDeletion(ctx, tx, conversation, w); err != nil {
			return err
		}
		plan = &model.ChatRetentionPrunePlan{
			ConversationID: conversation.ID, FromPrunedSeq: w.from, TargetSeq: w.target,
			LastSeq: w.lastSeq, ExpectedRevision: conversation.RetentionRevision, RunIDs: w.runIDs,
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return plan, replay, nil
}

// CommitPrune applies a plan whose checkpoint cleanup receipt is in hand, in
// one transaction: it re-checks that the Conversation is exactly as planned
// (same revision, no new message, nothing unsettled, same Runs), verifies the
// receipt, deletes raw messages and their controlled runtime copies, advances
// R and records the receipt on the prune job. A concurrent pass that already
// applied the plan is answered with its receipt.
//
// jobID names the claimed (running) job to complete; uuid.Nil records the
// receipt on the pass's own job instead.
func (r *ChatRetentionRepository) CommitPrune(
	ctx context.Context,
	owner uuid.UUID,
	plan *model.ChatRetentionPrunePlan,
	cleanup *model.CheckpointCleanupReceipt,
	jobID uuid.UUID,
) (*model.ChatRetentionPruneReceipt, error) {
	var receipt *model.ChatRetentionPruneReceipt
	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		conversation, err := r.lock(ctx, tx, owner)
		if err != nil {
			return err
		}
		if conversation.ID != plan.ConversationID {
			return model.ErrRetentionPlanStale
		}
		if conversation.RetentionRevision != plan.ExpectedRevision {
			if conversation.PrunedThroughSeq >= plan.TargetSeq {
				receipt, err = replayPrune(ctx, tx, conversation)
				return err
			}
			return model.ErrRetentionPlanStale
		}
		messages, err := liveMessages(ctx, tx, conversation)
		if err != nil {
			return err
		}
		w := computePruneWindow(conversation, messages)
		if w.busy {
			return model.ErrRetentionBusy
		}
		if w.lastSeq != plan.LastSeq || w.target != plan.TargetSeq || !slices.Equal(w.runIDs, plan.RunIDs) {
			return model.ErrRetentionPlanStale
		}
		if err := plan.VerifyCleanup(cleanup); err != nil {
			return err
		}
		d, err := planDeletion(ctx, tx, conversation, w)
		if err != nil {
			return err
		}

		// Uncited Ask User receipts go explicitly, never by cascade: those of
		// pruned Runs, and any whose answer or reply message leaves the window
		// (a Run held by another cited message). A cited receipt is never in
		// this set; if one were, its Safety FK would fail the whole commit.
		window := tx.WithContext(ctx).Model(&model.Message{}).Select("id").
			Where("conversation_id = ? AND seq > ? AND seq <= ?", conversation.ID, w.from, w.target)
		if len(d.keep) > 0 {
			window = window.Where("id NOT IN ?", d.keep)
		}
		resumes := tx.WithContext(ctx).Where("conversation_id = ?", conversation.ID)
		if len(d.runs) > 0 {
			resumes = resumes.Where("(run_id IN ? OR user_message_id IN (?) OR assistant_message_id IN (?))", d.runs, window, window)
		} else {
			resumes = resumes.Where("(user_message_id IN (?) OR assistant_message_id IN (?))", window, window)
		}
		if err := resumes.Delete(&model.InteractionResume{}).Error; err != nil {
			return err
		}

		var deletedEvents, deletedToolCalls, deletedInteractions int64
		if len(d.runs) > 0 {
			res := tx.WithContext(ctx).Exec("DELETE FROM agent_interactions WHERE conversation_id = ? AND run_id IN ?", conversation.ID, d.runs)
			if res.Error != nil {
				return res.Error
			}
			deletedInteractions = res.RowsAffected
			if err := tx.WithContext(ctx).Exec(
				"DELETE FROM thread_projection_tool_calls WHERE conversation_id = ? AND run_id IN ?", conversation.ID, d.runs).Error; err != nil {
				return err
			}
			res = tx.WithContext(ctx).Exec("DELETE FROM agent_tool_calls WHERE conversation_id = ? AND run_id IN ?", conversation.ID, d.runs)
			if res.Error != nil {
				return res.Error
			}
			deletedToolCalls = res.RowsAffected
			res = tx.WithContext(ctx).Exec("DELETE FROM runtime_events WHERE conversation_id = ? AND run_id IN ?", conversation.ID, d.runs)
			if res.Error != nil {
				return res.Error
			}
			deletedEvents = res.RowsAffected
		}
		del := tx.WithContext(ctx).Where("conversation_id = ? AND seq > ? AND seq <= ?", conversation.ID, w.from, w.target)
		if len(d.keep) > 0 {
			del = del.Where("id NOT IN ?", d.keep)
		}
		deleted := del.Delete(&model.Message{})
		if deleted.Error != nil {
			return deleted.Error
		}

		next := conversation.RetentionRevision + 1
		update := tx.WithContext(ctx).Model(&model.Conversation{}).
			Where("id = ? AND user_id = ? AND retention_revision = ?", conversation.ID, owner, conversation.RetentionRevision).
			Updates(map[string]any{"pruned_through_seq": w.target, "retention_revision": next, "updated_at": r.now()})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return model.ErrRetentionRevisionConflict
		}

		receipt = &model.ChatRetentionPruneReceipt{
			ConversationID: conversation.ID, FromPrunedSeq: w.from, PrunedThroughSeq: w.target,
			RetentionRevision: next, DeletedMessages: deleted.RowsAffected, RetainedMessages: len(d.keep),
			PrunedRuns: len(d.runs), DeletedRuntimeEvents: deletedEvents, DeletedToolCalls: deletedToolCalls,
			DeletedInteractions: deletedInteractions,
			CheckpointCleanup:   "verified", CheckpointMessagesRemoved: cleanup.RemovedMessages,
			CheckpointsDeleted: cleanup.DeletedCheckpoints,
		}
		if cleanup.CheckpointID != nil {
			receipt.CheckpointID = *cleanup.CheckpointID
		}
		input, err := json.Marshal(map[string]any{"fromPrunedSeq": w.from, "targetSeq": w.target, "expectedRevision": plan.ExpectedRevision})
		if err != nil {
			return err
		}
		receipt.JobID, err = r.recordPruneReceipt(ctx, tx, conversation, jobID, input, receipt)
		return err
	})
	if err != nil {
		return nil, err
	}
	return receipt, nil
}

// recordPruneReceipt completes the prune job in the commit transaction, so the
// receipt exists exactly when the cursor moved. jobID names a claimed job; a
// direct pass (uuid.Nil) records its receipt on a job of its own.
func (r *ChatRetentionRepository) recordPruneReceipt(
	ctx context.Context,
	tx *gorm.DB,
	conversation *model.Conversation,
	jobID uuid.UUID,
	input []byte,
	receipt *model.ChatRetentionPruneReceipt,
) (uuid.UUID, error) {
	now := r.now()
	if jobID == uuid.Nil {
		jobID = uuid.New()
		key := PruneJobKey(conversation.ID, receipt.FromPrunedSeq, "direct")
		convID := conversation.ID
		job := model.Job{
			ID: jobID, ConversationID: &convID, UserID: conversation.UserID, JobType: model.ChatRetentionPruneJobType,
			Status: model.JobStatusRunning, Input: datatypes.JSON(input), IdempotencyKey: &key,
			Attempts: 1, MaxAttempts: 1, StartedAt: &now, Metadata: datatypes.JSON(`{}`),
		}
		if err := tx.WithContext(ctx).Create(&job).Error; err != nil {
			return uuid.Nil, err
		}
	}
	receipt.JobID = jobID
	result, err := json.Marshal(receipt)
	if err != nil {
		return uuid.Nil, err
	}
	done := tx.WithContext(ctx).Model(&model.Job{}).
		Where("id = ? AND user_id = ? AND conversation_id = ? AND job_type = ? AND status = ?",
			jobID, conversation.UserID, conversation.ID, model.ChatRetentionPruneJobType, model.JobStatusRunning).
		Updates(map[string]any{
			"status": model.JobStatusCompleted, "result": datatypes.JSON(result),
			"finished_at": now, "updated_at": now,
		})
	if done.Error != nil {
		return uuid.Nil, done.Error
	}
	if done.RowsAffected != 1 {
		// The job is no longer this pass's to complete (cancelled or taken
		// over after going stale): roll the whole pass back.
		return uuid.Nil, model.ErrRetentionPlanStale
	}
	return jobID, tx.WithContext(ctx).Create(&model.JobEvent{
		JobID: jobID, EventType: "job.completed", Payload: datatypes.JSON(result),
	}).Error
}
