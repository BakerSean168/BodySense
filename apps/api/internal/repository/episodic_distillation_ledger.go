package repository

import (
	"context"
	"errors"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RecordPlan makes every enumerated segment durably accounted for before any
// model call. An ineligible segment is skipped; an eligible one starts or
// keeps a pending row. This is one transaction, so a crash cannot leave a
// later segment with no ledger record at all.
func (r *EpisodicDistillationRepository) RecordPlan(
	ctx context.Context, owner, conversationID uuid.UUID, segments []EpisodicDistillationSegment,
) error {
	if owner == uuid.Nil || conversationID == uuid.Nil || len(segments) == 0 {
		return nil
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
		now := r.now().UTC()
		for _, seg := range segments {
			status := EpisodicDistillationPending
			reason := ""
			if !seg.Eligible {
				status, reason = EpisodicDistillationSkipped, seg.Reason
			}
			if err := recordDistillationStatus(ctx, tx, owner, conversationID, seg.FromSeq, seg.ToSeq,
				status, reason, nil, now); err != nil {
				return err
			}
		}
		return nil
	})
}

// RecordFailure increments the attempt count and either keeps the segment
// pending or gap it when the retry ceiling is reached. A terminal row is
// returned unchanged, so a late retry cannot reopen a resolved segment.
func (r *EpisodicDistillationRepository) RecordFailure(
	ctx context.Context, owner, conversationID uuid.UUID, fromSeq, toSeq int,
	reason string, maxAttempts int,
) (string, int, error) {
	if maxAttempts < 1 {
		maxAttempts = EpisodicDistillationMaxAttempts
	}
	if maxAttempts > 16 {
		maxAttempts = 16
	}
	var status string
	var attempts int
	now := r.now().UTC()
	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		var c model.Conversation
		if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "NO KEY UPDATE"}).
			Where("id = ? AND user_id = ?", conversationID, owner).Take(&c).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return model.ErrRetentionNotFound
			}
			return err
		}
		row := tx.WithContext(ctx).Raw(`
INSERT INTO episodic_distillation_segments
    (user_id, conversation_id, from_seq, to_seq, status, attempts, reason, created_at, updated_at)
VALUES (?, ?, ?, ?, 'pending', 1, ?, ?, ?)
ON CONFLICT (conversation_id, from_seq, to_seq) DO UPDATE
SET attempts = CASE
        WHEN episodic_distillation_segments.status = 'pending'
        THEN LEAST(episodic_distillation_segments.attempts + 1, 16)
        ELSE episodic_distillation_segments.attempts
    END,
    status = CASE
        WHEN episodic_distillation_segments.status <> 'pending' THEN episodic_distillation_segments.status
        WHEN LEAST(episodic_distillation_segments.attempts + 1, 16) >= ? THEN 'gapped'
        ELSE 'pending'
    END,
    reason = CASE
        WHEN episodic_distillation_segments.status <> 'pending' THEN episodic_distillation_segments.reason
        ELSE EXCLUDED.reason
    END,
    updated_at = EXCLUDED.updated_at
RETURNING status, attempts`,
			owner, conversationID, fromSeq, toSeq, reason, now, now, maxAttempts).
			Row().Scan(&status, &attempts)
		return row
	})
	return status, attempts, err
}

// RecordResolved writes a terminal outcome. An existing terminal row wins
// (idempotent replay), while a pending row may transition once.
func (r *EpisodicDistillationRepository) RecordResolved(
	ctx context.Context, owner, conversationID uuid.UUID, fromSeq, toSeq int,
	status, reason string, noteID *uuid.UUID,
) error {
	if status != EpisodicDistillationDistilled && status != EpisodicDistillationSkipped &&
		status != EpisodicDistillationGapped {
		return errors.New("invalid episodic distillation status")
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
		return recordDistillationStatus(ctx, tx, owner, conversationID, fromSeq, toSeq,
			status, reason, noteID, r.now().UTC())
	})
}

func recordDistillationStatus(
	ctx context.Context, tx *gorm.DB, owner, conversationID uuid.UUID,
	fromSeq, toSeq int, status, reason string, noteID *uuid.UUID, now time.Time,
) error {
	return tx.WithContext(ctx).Exec(`
INSERT INTO episodic_distillation_segments
    (user_id, conversation_id, from_seq, to_seq, status, attempts, note_id, reason, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, 0, ?, ?, ?, ?)
ON CONFLICT (conversation_id, from_seq, to_seq) DO UPDATE
SET status = CASE
        WHEN episodic_distillation_segments.status = 'pending' THEN EXCLUDED.status
        ELSE episodic_distillation_segments.status
    END,
    reason = CASE
        WHEN episodic_distillation_segments.status = 'pending' THEN EXCLUDED.reason
        ELSE episodic_distillation_segments.reason
    END,
    note_id = COALESCE(episodic_distillation_segments.note_id, EXCLUDED.note_id),
    updated_at = EXCLUDED.updated_at`,
		owner, conversationID, fromSeq, toSeq, status, noteID, reason, now, now).Error
}
