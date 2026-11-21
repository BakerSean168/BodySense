package repository

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

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
