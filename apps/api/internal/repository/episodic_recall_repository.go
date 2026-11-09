package repository

import (
	"context"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// EpisodicRecallRepository is the read-only S4 candidate source. It returns
// only the caller's own non-clinical shadow notes and never mutates anything.
//
// Tombstoned source ranges are excluded by an anti-join even though S6-A
// deletes the note rows themselves: the anti-join is the serving-time
// guarantee that a completed user deletion can never be resurrected by a row
// that survived a race or a future writer.
type EpisodicRecallRepository struct {
	db *gorm.DB
}

func NewEpisodicRecallRepository(db *gorm.DB) *EpisodicRecallRepository {
	return &EpisodicRecallRepository{db: db}
}

// ListOwnedRecallCandidates returns at most limit notes, newest first. Ranking
// and the relevance threshold live in internal/episodicrecall so the runtime
// and the offline evaluation share one scorer.
func (r *EpisodicRecallRepository) ListOwnedRecallCandidates(
	ctx context.Context,
	owner uuid.UUID,
	limit int,
) ([]model.EpisodicNote, error) {
	if owner == uuid.Nil || limit <= 0 {
		return nil, nil
	}
	notes := make([]model.EpisodicNote, 0, limit)
	err := database.FromContext(ctx, r.db).
		Where("user_id = ? AND status = 'shadow'", owner).
		Where(`NOT EXISTS (
			SELECT 1 FROM public.episodic_note_deletions d
			WHERE d.user_id = episodic_notes.user_id
			  AND d.conversation_id = episodic_notes.conversation_id
			  AND d.from_seq <= episodic_notes.to_seq
			  AND d.to_seq >= episodic_notes.from_seq)`).
		Order("created_at DESC, id DESC").
		Limit(limit).
		Find(&notes).Error
	if err != nil {
		return nil, err
	}
	return notes, nil
}
