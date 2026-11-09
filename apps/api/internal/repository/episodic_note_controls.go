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

var ErrEpisodicNoteNotFound = errors.New("episodic note not found")

// EpisodicNoteDeletion stores only provenance range and note identity.
// It deliberately contains no message UUIDs, quoted text, clinical detail or
// model output. Owner account erasure cascades from Conversation.
type EpisodicNoteDeletion struct {
	UserID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	NoteID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	ConversationID uuid.UUID `gorm:"type:uuid;not null"`
	FromSeq        int       `gorm:"not null"`
	ToSeq          int       `gorm:"not null"`
	DeletedAt      time.Time `gorm:"not null"`
}

func (EpisodicNoteDeletion) TableName() string { return "episodic_note_deletions" }

// EpisodicNoteControlsRepository is the S6-A owner-only read/delete boundary.
// No public API, UI or production application constructor uses it yet.
// Clinical BodyState/diagnosis/safety objects are NEVER read or modified here.
type EpisodicNoteControlsRepository struct {
	db *gorm.DB
}

func NewEpisodicNoteControlsRepository(db *gorm.DB) *EpisodicNoteControlsRepository {
	return &EpisodicNoteControlsRepository{db: db}
}

// ListOwned returns only the caller's *active* shadow notes. Existing notes
// are bounded to the approved per-owner policy cap; no erased tombstones or
// raw message provenance are exposed as separate user content.
func (r *EpisodicNoteControlsRepository) ListOwned(ctx context.Context, owner uuid.UUID) ([]model.EpisodicNote, error) {
	if owner == uuid.Nil {
		return nil, ErrEpisodicNoteNotFound
	}
	notes := make([]model.EpisodicNote, 0)
	err := database.FromContext(ctx, r.db).
		Where("user_id = ? AND status = 'shadow'", owner).
		Order("created_at DESC, id DESC").Limit(episodicRetentionPolicyMaxNotes).Find(&notes).Error
	return notes, err
}

// DeleteOwned removes a user note and *all* other shadow notes whose source
// ranges overlap it, so a second active note cannot retain the same text.
// It then leaves a content-free tombstone spanning the deleted source range.
// This blocks retries, new generator versions and clipped/expanded windows.
// Same-owner repeat deletion is idempotent, but foreign/unknown ids are masked
// as not found. Deletion never changes BodyState, statements or raw chat.
func (r *EpisodicNoteControlsRepository) DeleteOwned(ctx context.Context, owner, noteID uuid.UUID) (bool, error) {
	if owner == uuid.Nil || noteID == uuid.Nil {
		return false, ErrEpisodicNoteNotFound
	}
	deleted := false
	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		var note model.EpisodicNote
		err := tx.Where("user_id = ? AND id = ?", owner, noteID).Take(&note).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// A repeated owner deletion is safe and indistinguishable from
			// completion; another owner's tombstone is never visible here.
			var count int64
			if err := tx.Model(&EpisodicNoteDeletion{}).Where("user_id = ? AND note_id = ?", owner, noteID).Count(&count).Error; err != nil {
				return err
			}
			if count == 1 {
				return nil
			}
			return ErrEpisodicNoteNotFound
		}
		if err != nil {
			return err
		}
		// S2 writes and raw chat pruning lock this same Conversation row.
		// The SQL insertion guard takes FOR SHARE on the same row so even
		// direct SQL cannot race to insert the deleted source window.
		var conv model.Conversation
		if err := tx.Clauses(clause.Locking{Strength: "NO KEY UPDATE"}).
			Where("id = ? AND user_id = ?", note.ConversationID, owner).
			Take(&conv).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrEpisodicNoteNotFound
			}
			return err
		}
		// The note can have been removed by a separate source-erasure pass
		// between the initial lookup and acquiring the Conversation lock.
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND id = ?", owner, noteID).
			Take(&note).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrEpisodicNoteNotFound
			}
			return err
		}
		tombstone := EpisodicNoteDeletion{
			UserID: owner, NoteID: note.ID, ConversationID: note.ConversationID,
			FromSeq: note.FromSeq, ToSeq: note.ToSeq, DeletedAt: time.Now().UTC(),
		}
		if err := tx.Create(&tombstone).Error; err != nil {
			return err
		}
		// Remove overlapping copies *atomically*, never just the selected
		// row. Any future attempt to write overlapping evidence is blocked
		// by the durable tombstone and the DB trigger.
		if err := tx.Where("user_id = ? AND conversation_id = ? AND from_seq <= ? AND to_seq >= ?",
			owner, note.ConversationID, note.ToSeq, note.FromSeq).
			Delete(&model.EpisodicNote{}).Error; err != nil {
			return err
		}
		deleted = true
		return nil
	})
	return deleted, err
}
