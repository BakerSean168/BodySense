package repository

import (
	"context"
	"errors"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AdvanceWaterline moves D forward monotonically. It never bumps
// RetentionRevision, so a prune plan taken before the trigger stays valid.
func (r *EpisodicDistillationRepository) AdvanceWaterline(
	ctx context.Context, owner, conversationID uuid.UUID, through int,
) error {
	if owner == uuid.Nil || conversationID == uuid.Nil || through < 0 {
		return model.ErrRetentionNotFound
	}
	return database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		var c model.Conversation
		if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "NO KEY UPDATE"}).
			Where("id = ? AND user_id = ?", conversationID, owner).Take(&c).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return model.ErrRetentionNotFound
			}
			return err
		}
		if through <= c.DistilledThroughSeq {
			return nil
		}
		if through > c.ProcessedThroughSeq {
			return model.ErrRetentionCursorInvalid
		}
		return tx.WithContext(ctx).Exec(`
UPDATE conversations SET distilled_through_seq = ?, updated_at = ?
WHERE id = ? AND user_id = ? AND distilled_through_seq < ?`,
			through, r.now().UTC(), conversationID, owner, through).Error
	})
}

// HealAfterPrune closes the window after R moves. Any segment still pending
// inside the pruned range is a recorded gap (the source is gone by design),
// and D advances to R. If the trigger never enumerated the range at all, one
// coarse content-free gap row preserves that fact instead of losing it.
func (r *EpisodicDistillationRepository) HealAfterPrune(
	ctx context.Context, owner, conversationID uuid.UUID,
) (int, error) {
	if owner == uuid.Nil || conversationID == uuid.Nil {
		return 0, model.ErrRetentionNotFound
	}
	gaps := 0
	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		var c model.Conversation
		if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "NO KEY UPDATE"}).
			Where("id = ? AND user_id = ?", conversationID, owner).Take(&c).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return model.ErrRetentionNotFound
			}
			return err
		}
		if c.DistilledThroughSeq >= c.PrunedThroughSeq {
			return nil
		}
		if c.PrunedThroughSeq > c.ProcessedThroughSeq {
			return model.ErrRetentionCursorInvalid
		}
		now := r.now().UTC()
		res := tx.WithContext(ctx).Exec(`
UPDATE episodic_distillation_segments
SET status = 'gapped', reason = 'pruned_before_distillation', updated_at = ?
WHERE conversation_id = ? AND user_id = ? AND status = 'pending' AND to_seq <= ?`,
			now, conversationID, owner, c.PrunedThroughSeq)
		if res.Error != nil {
			return res.Error
		}
		gaps = int(res.RowsAffected)
		var accounted int64
		if err := tx.WithContext(ctx).Raw(`
SELECT count(*) FROM episodic_distillation_segments
WHERE conversation_id = ? AND user_id = ? AND from_seq > ? AND to_seq <= ?`,
			conversationID, owner, c.DistilledThroughSeq, c.PrunedThroughSeq).Scan(&accounted).Error; err != nil {
			return err
		}
		if accounted == 0 {
			if err := tx.WithContext(ctx).Exec(`
INSERT INTO episodic_distillation_segments
    (user_id, conversation_id, from_seq, to_seq, status, attempts, reason, created_at, updated_at)
VALUES (?, ?, ?, ?, 'gapped', 0, 'pruned_unenumerated', ?, ?)
ON CONFLICT (conversation_id, from_seq, to_seq) DO NOTHING`,
				owner, conversationID, c.DistilledThroughSeq+1, c.PrunedThroughSeq, now, now).Error; err != nil {
				return err
			}
			gaps++
		}
		return tx.WithContext(ctx).Exec(`
UPDATE conversations SET distilled_through_seq = ?, updated_at = ?
WHERE id = ? AND user_id = ? AND distilled_through_seq < ?`,
			c.PrunedThroughSeq, now, conversationID, owner, c.PrunedThroughSeq).Error
	})
	if err != nil {
		return 0, err
	}
	return gaps, nil
}
