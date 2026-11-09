package repository

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrEpisodicShadowDisabled  = errors.New("episodic shadow admission disabled")
	ErrEpisodicSourceInvalid   = errors.New("episodic shadow source is unavailable or incomplete")
	ErrEpisodicDraftConflict   = errors.New("episodic shadow draft conflicts with an existing range")
	ErrEpisodicNoteUserDeleted = errors.New("episodic source window was deleted by user")
)

// Stage S2-A is deliberately *not* wired to consultation, retention, AI or
// HTTP. Enabling this Go-only draft admission must remain an explicit future
// product decision; disabled is the constructor default.
const (
	episodicMaxSourceMessages = 128
	episodicMaxNoteRunes      = 800

	// Approved per-owner rolling retention policy (product owner,
	// 2026-11-05; basis: episodic-s2-benchmark-recall-eval.md). Both the
	// note-count and stored-rune limits apply, and the oldest notes roll
	// off first. The former test-only 48 / 16,000 values are historical.
	episodicRetentionPolicyMaxNotes       = 96
	episodicRetentionPolicyMaxStoredRunes = 32000
)

// EpisodicShadowDraft is a candidate from a future, typed, bounded generator.
// Go, never Python, owns the source IDs, sequence bounds and DB admission.
// S2-A conservatively permits *text-only, non-clinical* source windows; it
// rejects asset-bearing and statement-evidence-linked windows until later
// provenance/clinical admission has a separately tested implementation.
type EpisodicShadowDraft struct {
	ConversationID         uuid.UUID
	FromSeq                int
	ToSeq                  int
	SourceMessageIDs       []uuid.UUID
	SourceAssetIDs         []uuid.UUID
	GeneratorVersion       string
	RetentionPolicyVersion string
	ContentText            string
	// S2-B: if quoted evidence is supplied, Go must verify the *entire*
	// user message against the locked database row before accepting a
	// canonical, fixed non-clinical preference. No model-authored summary.
	QuotedMessageID   uuid.UUID
	QuotedMessageText string
}

// EpisodicShadowRepository is a server-only, disabled-by-default boundary.
// No endpoint exposes it, no scheduler runs it, and no context path reads it.
type EpisodicShadowRepository struct {
	db      *gorm.DB
	enabled bool
}

func NewEpisodicShadowRepository(db *gorm.DB) *EpisodicShadowRepository {
	return &EpisodicShadowRepository{db: db}
}

// WithShadowAdmissionEnabled must be opted into by an isolated caller. Do not
// wire it to normal ChatRetentionService or a deployment flag during S2-A.
func (r *EpisodicShadowRepository) WithShadowAdmissionEnabled(enabled bool) *EpisodicShadowRepository {
	r.enabled = enabled
	return r
}

func validShadowDraft(d EpisodicShadowDraft) bool {
	n := len(d.SourceMessageIDs)
	if d.ConversationID == uuid.Nil || d.FromSeq < 1 || d.ToSeq < d.FromSeq ||
		n < 1 || n > episodicMaxSourceMessages || d.ToSeq-d.FromSeq+1 != n ||
		len(d.SourceAssetIDs) != 0 || // no asset-origin text until S2-B
		d.GeneratorVersion == "" || len(d.GeneratorVersion) > 128 ||
		d.RetentionPolicyVersion == "" || len(d.RetentionPolicyVersion) > 128 ||
		strings.TrimSpace(d.GeneratorVersion) != d.GeneratorVersion ||
		strings.TrimSpace(d.RetentionPolicyVersion) != d.RetentionPolicyVersion ||
		strings.TrimSpace(d.ContentText) != d.ContentText {
		return false
	}
	size := utf8.RuneCountInString(d.ContentText)
	if size < 1 || size > episodicMaxNoteRunes || !utf8.ValidString(d.ContentText) {
		return false
	}
	seen := make(map[uuid.UUID]bool, n)
	for _, id := range d.SourceMessageIDs {
		if id == uuid.Nil || seen[id] {
			return false
		}
		seen[id] = true
	}
	if d.QuotedMessageID != uuid.Nil || d.QuotedMessageText != "" {
		canonical, ok := CanonicalEpisodicPreference(d.QuotedMessageText)
		if !ok || !seen[d.QuotedMessageID] || canonical.Text != d.ContentText {
			return false
		}
	}
	return true
}

type episodicSourceRow struct {
	ID           uuid.UUID
	TurnID       uuid.UUID
	Seq          int
	Role         string
	Status       string
	RunStatus    *string
	Parts        string
	ContentBytes int
	ContentText  string
	// Text is the canonical source text derived by episodicSourceText. It is
	// never scanned from the database; the scan only fills the row columns.
	Text string
}

// SaveShadowDraft enforces same-owner, exact and fully settled source
// provenance under the same Conversation lock as a prune/turn commit.
// Its transaction either commits the entire bounded shadow note or changes
// nothing. It never keeps the raw messages alive.
func (r *EpisodicShadowRepository) SaveShadowDraft(
	ctx context.Context, owner uuid.UUID, draft EpisodicShadowDraft,
) (*model.EpisodicNote, bool, error) {
	if !r.enabled {
		return nil, false, ErrEpisodicShadowDisabled
	}
	if owner == uuid.Nil || !validShadowDraft(draft) {
		return nil, false, ErrEpisodicSourceInvalid
	}
	var saved model.EpisodicNote
	created := false
	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		var c model.Conversation
		err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "NO KEY UPDATE"}).
			Where("id = ? AND user_id = ?", draft.ConversationID, owner).
			Take(&c).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrEpisodicSourceInvalid
		}
		if err != nil {
			return err
		}
		// A durable user delete wins over a generator retry, a later model
		// version, or any overlapping replay source window. The record has
		// no content and is kept until the owner Conversation is erased.
		var blocked int64
		if err := tx.Table("episodic_note_deletions").
			Where("user_id = ? AND conversation_id = ? AND from_seq <= ? AND to_seq >= ?",
				owner, c.ID, draft.ToSeq, draft.FromSeq).
			Count(&blocked).Error; err != nil {
			return err
		}
		if blocked > 0 {
			return ErrEpisodicNoteUserDeleted
		}
		// A generation retry after normal pruning must replay the existing
		// note, not read resurrected or stale source messages.
		err = tx.Where("user_id = ? AND conversation_id = ? AND from_seq = ? AND to_seq = ? AND generator_version = ? AND retention_policy_version = ?",
			owner, c.ID, draft.FromSeq, draft.ToSeq, draft.GeneratorVersion, draft.RetentionPolicyVersion).
			Take(&saved).Error
		if err == nil {
			want := make([]string, len(draft.SourceMessageIDs))
			for i, id := range draft.SourceMessageIDs {
				want[i] = id.String()
			}
			if !slices.Equal([]string(saved.SourceMessageIDs), want) ||
				len(saved.SourceAssetIDs) != 0 || saved.ContentText != draft.ContentText || saved.Status != "shadow" {
				return ErrEpisodicDraftConflict
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		// C has moved past this complete segment; the input is still
		// available before R removes it. Retention never waits for a note.
		if c.ProcessedThroughSeq < draft.ToSeq || c.ContextStartSeq <= draft.ToSeq ||
			c.PrunedThroughSeq >= draft.FromSeq {
			return ErrEpisodicSourceInvalid
		}
		var rows []episodicSourceRow
		if err := tx.WithContext(ctx).Raw(`SELECT m.id,m.turn_id,m.seq,m.role,m.status,m.parts::text AS parts,
octet_length(COALESCE(m.content_text,'')) AS content_bytes,
COALESCE(m.content_text,'') AS content_text, r.status AS run_status FROM messages m
LEFT JOIN runs r ON r.id = m.run_id
WHERE m.conversation_id = ? AND m.seq BETWEEN ? AND ?
ORDER BY m.seq`, c.ID, draft.FromSeq, draft.ToSeq).Scan(&rows).Error; err != nil {
			return err
		}
		if len(rows) != len(draft.SourceMessageIDs) {
			return ErrEpisodicSourceInvalid
		}
		for i := range rows {
			msg := &rows[i]
			text, pureText, ok := episodicSourceText(msg.Parts, msg.ContentText)
			// The user message is the only quotable source, so it must still
			// be pure text. Assistant replies may carry non-text parts
			// (quotes and similar) as long as their canonical body exists.
			if !ok || msg.ID != draft.SourceMessageIDs[i] || msg.Seq != draft.FromSeq+i ||
				msg.Status != "completed" || text == "" ||
				(msg.Role != "user" && msg.Role != "assistant") ||
				(msg.RunStatus != nil && *msg.RunStatus != "completed") ||
				(msg.Role == "user" && !pureText) {
				return ErrEpisodicSourceInvalid
			}
			msg.Text = text
		}
		// S2-B quote must match the actual owner-locked *whole user message*,
		// compared against its canonical text rather than the legacy
		// content_text column. Quotes from a model or its caller have no
		// authority over DB source content, and never enter the stored text
		// verbatim. Only a user message may be quoted.
		if draft.QuotedMessageID != uuid.Nil {
			matched := false
			for _, msg := range rows {
				if msg.ID == draft.QuotedMessageID && msg.Role == "user" &&
					msg.Text == draft.QuotedMessageText {
					matched = true
					break
				}
			}
			if !matched {
				return ErrEpisodicSourceInvalid
			}
		}
		// Do not cut a turn in half. A clipped user/assistant exchange
		// could turn incomplete context into an invented enduring claim.
		turnIDs := make([]uuid.UUID, 0, len(rows))
		for _, row := range rows {
			if !slices.Contains(turnIDs, row.TurnID) {
				turnIDs = append(turnIDs, row.TurnID)
			}
		}
		var missingParts int64
		if err := tx.Table("messages").Where("conversation_id = ? AND turn_id IN ? AND (seq < ? OR seq > ?)",
			c.ID, turnIDs, draft.FromSeq, draft.ToSeq).Count(&missingParts).Error; err != nil {
			return err
		}
		if missingParts != 0 {
			return ErrEpisodicSourceInvalid
		}
		// Clinical and Safety sources are owned by their proper domains.
		// A draft must never fork these authoritative facts into narrative.
		for _, table := range []string{"statement_evidence", "safety_screens", "safety_resolutions"} {
			var n int64
			if err := tx.Table(table).Where("source_message_id IN ?", draft.SourceMessageIDs).Count(&n).Error; err != nil {
				return err
			}
			if n != 0 {
				return ErrEpisodicSourceInvalid
			}
		}
		now := time.Now().UTC()
		ids := make(pq.StringArray, len(draft.SourceMessageIDs))
		for i, id := range draft.SourceMessageIDs {
			ids[i] = id.String()
		}
		// Do not exceed a rolling per-owner storage policy; all writes and
		// roll-offs serialize on the owner's singleton Conversation lock.
		var old []model.EpisodicNote
		if err := tx.Where("user_id = ?", owner).Order("created_at ASC").Order("id ASC").Find(&old).Error; err != nil {
			return err
		}
		total := utf8.RuneCountInString(draft.ContentText)
		for _, note := range old {
			total += utf8.RuneCountInString(note.ContentText)
		}
		for len(old)+1 > episodicRetentionPolicyMaxNotes || total > episodicRetentionPolicyMaxStoredRunes {
			if len(old) == 0 {
				return ErrEpisodicSourceInvalid
			}
			first := old[0]
			if err := tx.Where("user_id = ? AND id = ?", owner, first.ID).Delete(&model.EpisodicNote{}).Error; err != nil {
				return err
			}
			total -= utf8.RuneCountInString(first.ContentText)
			old = old[1:]
		}
		saved = model.EpisodicNote{
			UserID: owner, ConversationID: c.ID, FromSeq: draft.FromSeq, ToSeq: draft.ToSeq,
			SourceMessageIDs: ids, SourceAssetIDs: pq.StringArray{},
			GeneratorVersion: draft.GeneratorVersion, RetentionPolicyVersion: draft.RetentionPolicyVersion,
			ContentText: draft.ContentText, Status: "shadow", CreatedAt: now, UpdatedAt: now,
		}
		if err := tx.Create(&saved).Error; err != nil {
			return fmt.Errorf("persist shadow note: %w", err)
		}
		created = true
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return &saved, created, nil
}
