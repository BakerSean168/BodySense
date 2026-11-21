package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

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

// RecordPruneEvent appends a content-free auxiliary event to an already
// completed prune job. The S2 trigger uses it to expose distillation counts;
// failing to record it never changes the prune outcome or the watermark.
func (r *ChatRetentionRepository) RecordPruneEvent(
	ctx context.Context, jobID uuid.UUID, eventType string, payload any,
) error {
	if jobID == uuid.Nil || eventType == "" {
		return nil
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return database.FromContext(ctx, r.db).WithContext(ctx).Create(&model.JobEvent{
		JobID: jobID, EventType: eventType, Payload: datatypes.JSON(body),
	}).Error
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
