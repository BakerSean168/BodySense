package repository

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// S2 best-effort distillation bookkeeping. This repository reads source text
// only to hand it to the already-admitted S2-B1 proposal path; it never
// stores text, message ids or model output in the ledger. The ledger is
// content-free provenance: seq range, status, attempt count and a stable
// reason code.
const (
	// EpisodicDistillationMaxAttempts is the per-segment retry ceiling. A
	// segment is gapped (and therefore released for pruning) after this many
	// failed model/admission attempts across prune passes.
	EpisodicDistillationMaxAttempts = 3
	// EpisodicDistillationMaxSegmentsPerPass bounds model calls in one prune
	// pass. Segments beyond the bound are recorded as gaps, never silently
	// dropped.
	EpisodicDistillationMaxSegmentsPerPass = 8
	// EpisodicShadowSourceMaxRunes mirrors the private Proto string bound.
	EpisodicShadowSourceMaxRunes = 1200
	// EpisodicShadowSourceWindowRunes mirrors the S2-B1 command budget.
	EpisodicShadowSourceWindowRunes = 8192
)

// EpisodicDistillationStatus values are terminal except pending.
const (
	EpisodicDistillationPending   = "pending"
	EpisodicDistillationDistilled = "distilled"
	EpisodicDistillationSkipped   = "skipped"
	EpisodicDistillationGapped    = "gapped"
)

// EpisodicDistillationState is the cursor view the trigger needs. R is the
// physical prune boundary; D is the last final distillation outcome.
type EpisodicDistillationState struct {
	ConversationID      uuid.UUID
	ProcessedThroughSeq int
	ContextStartSeq     int
	PrunedThroughSeq    int
	DistilledThroughSeq int
}

// EpisodicDistillationMessage is one text-only source or context row. It is
// not persisted by this repository.
type EpisodicDistillationMessage struct {
	ID   uuid.UUID
	Seq  int
	Role string
	Text string
}

// EpisodicDistillationSegment is a complete interaction (exactly one user and
// one assistant message, contiguous seqs) that retention is about to remove.
// Status/Attempts come from the ledger; Eligible/Reason are a conservative
// pre-check and SaveShadowDraft remains the authoritative gate.
type EpisodicDistillationSegment struct {
	FromSeq  int
	ToSeq    int
	Messages []EpisodicDistillationMessage
	Context  []EpisodicDistillationMessage
	Eligible bool
	Reason   string
	Status   string
	Attempts int
	NoteID   *uuid.UUID
}

type episodicDistillationRow struct {
	ID                uuid.UUID
	TurnID            uuid.UUID
	Seq               int
	Role              string
	Status            string
	Parts             string
	ContentBytes      int
	ContentText       string
	RunStatus         *string
	RunUserID         *uuid.UUID
	RunConversationID *uuid.UUID
	TurnSize          int
	ClinicalRefs      int
}

type episodicDistillationLedgerRow struct {
	FromSeq  int
	ToSeq    int
	Status   string
	Attempts int
	NoteID   *uuid.UUID
}

// EpisodicDistillationRepository is the server-only bookkeeping boundary for
// the S2 trigger. It is not wired to HTTP, chat serving or clinical inputs.
type EpisodicDistillationRepository struct {
	db  *gorm.DB
	now func() time.Time
}

func NewEpisodicDistillationRepository(db *gorm.DB) *EpisodicDistillationRepository {
	return &EpisodicDistillationRepository{db: db, now: time.Now}
}

func (r *EpisodicDistillationRepository) State(ctx context.Context, owner uuid.UUID) (*EpisodicDistillationState, error) {
	if owner == uuid.Nil {
		return nil, model.ErrRetentionNotFound
	}
	var c model.Conversation
	err := database.FromContext(ctx, r.db).
		Select("id", "processed_through_seq", "context_start_seq", "pruned_through_seq", "distilled_through_seq").
		Where("user_id = ?", owner).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, model.ErrRetentionNotFound
	}
	if err != nil {
		return nil, err
	}
	return &EpisodicDistillationState{
		ConversationID: c.ID, ProcessedThroughSeq: c.ProcessedThroughSeq,
		ContextStartSeq: c.ContextStartSeq, PrunedThroughSeq: c.PrunedThroughSeq,
		DistilledThroughSeq: c.DistilledThroughSeq,
	}, nil
}

// Segments returns every complete two-message interaction that starts after
// afterSeq and ends at or before throughSeq, in seq order. It includes up to
// two readable messages immediately before each segment as untrusted model
// context, but the segment's own Messages remain the only provenance and the
// only quotable source. A partially deleted or unsupported turn is returned
// as ineligible with a stable reason so the caller can record a skip and move
// the watermark; it is never sent to a model.
func (r *EpisodicDistillationRepository) Segments(
	ctx context.Context, owner, conversationID uuid.UUID, afterSeq, throughSeq int,
) ([]EpisodicDistillationSegment, error) {
	if owner == uuid.Nil || conversationID == uuid.Nil || throughSeq < 1 || throughSeq <= afterSeq {
		return nil, nil
	}
	start := afterSeq - 1
	if start < 1 {
		start = 1
	}
	var rows []episodicDistillationRow
	err := database.FromContext(ctx, r.db).Raw(`
SELECT m.id, m.turn_id, m.seq, m.role, m.status, m.parts::text AS parts,
       octet_length(COALESCE(m.content_text, '')) AS content_bytes,
       COALESCE(m.content_text, '') AS content_text,
       r.status AS run_status, r.user_id AS run_user_id,
       r.conversation_id AS run_conversation_id,
       (SELECT count(*) FROM messages x
         WHERE x.conversation_id = m.conversation_id AND x.turn_id = m.turn_id) AS turn_size,
       ((SELECT count(*) FROM statement_evidence e WHERE e.source_message_id = m.id)
        + (SELECT count(*) FROM safety_screens s WHERE s.source_message_id = m.id)
        + (SELECT count(*) FROM safety_resolutions sr WHERE sr.source_message_id = m.id)
        + (SELECT count(*) FROM condition_analysis_runs c WHERE c.request_message_id = m.id)
        + (SELECT count(*) FROM condition_direct_edits d WHERE d.request_message_id = m.id)) AS clinical_refs
FROM messages m
LEFT JOIN runs r ON r.id = m.run_id
WHERE m.conversation_id = ? AND m.seq >= ? AND m.seq <= ?
ORDER BY m.seq`, conversationID, start, throughSeq).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	var deletionRows []struct {
		FromSeq int
		ToSeq   int
	}
	if err := database.FromContext(ctx, r.db).Raw(
		"SELECT from_seq, to_seq FROM episodic_note_deletions WHERE conversation_id = ? AND user_id = ?",
		conversationID, owner).Scan(&deletionRows).Error; err != nil {
		return nil, err
	}
	tombstones := make([]seqRange, len(deletionRows))
	for i, row := range deletionRows {
		tombstones[i] = seqRange{from: row.FromSeq, to: row.ToSeq}
	}
	ledger, err := r.ledgerRows(ctx, owner, conversationID, afterSeq, throughSeq)
	if err != nil {
		return nil, err
	}
	order, byTurn := groupTurns(rows)
	segments := make([]EpisodicDistillationSegment, 0, len(order))
	for _, turnID := range order {
		grp := byTurn[turnID]
		from, to := grp[0].Seq, grp[len(grp)-1].Seq
		// The first group(s) are lookback only. A group that straddles the
		// watermark is not a complete segment in this pass.
		if to <= afterSeq || from <= afterSeq || to > throughSeq {
			continue
		}
		seg := EpisodicDistillationSegment{FromSeq: from, ToSeq: to}
		if row, ok := ledger[seqRange{from: from, to: to}]; ok {
			seg.Status, seg.Attempts, seg.NoteID = row.Status, row.Attempts, row.NoteID
		}
		seg.Eligible, seg.Reason = evaluateDistillationGroup(grp, owner, conversationID, tombstones)
		if seg.Eligible {
			seg.Messages = make([]EpisodicDistillationMessage, len(grp))
			for i, row := range grp {
				text, _, _ := episodicSourceText(row.Parts, row.ContentText)
				seg.Messages[i] = EpisodicDistillationMessage{ID: row.ID, Seq: row.Seq, Role: row.Role, Text: text}
			}
			seg.Context = distillationContext(rows, from, owner, conversationID, tombstones)
		}
		segments = append(segments, seg)
	}
	return segments, nil
}

type seqRange struct {
	from int
	to   int
}

func (s seqRange) overlaps(from, to int) bool {
	return s.from <= to && s.to >= from
}

func overlapsAny(ranges []seqRange, from, to int) bool {
	for _, r := range ranges {
		if r.overlaps(from, to) {
			return true
		}
	}
	return false
}

func groupTurns(rows []episodicDistillationRow) ([]uuid.UUID, map[uuid.UUID][]episodicDistillationRow) {
	order := make([]uuid.UUID, 0, len(rows)/2)
	byTurn := make(map[uuid.UUID][]episodicDistillationRow, len(rows)/2)
	for _, row := range rows {
		if _, ok := byTurn[row.TurnID]; !ok {
			order = append(order, row.TurnID)
		}
		byTurn[row.TurnID] = append(byTurn[row.TurnID], row)
	}
	for _, group := range byTurn {
		slices.SortFunc(group, func(a, b episodicDistillationRow) int { return a.Seq - b.Seq })
	}
	return order, byTurn
}

// evaluateDistillationGroup is intentionally narrower than SaveShadowDraft:
// S2-B1 only proposes an exact whole user message from a two-message turn.
// The repository write remains the authoritative check under the row lock.
//
// Source text is read through episodicSourceText so the planner admits the
// shapes the runtime actually persists. In particular the user's body may
// live only in text parts, and the assistant's parts may carry quotes; only
// the user message has to be pure text because only it can be quoted.
func evaluateDistillationGroup(
	grp []episodicDistillationRow, owner, conversationID uuid.UUID, tombstones []seqRange,
) (bool, string) {
	if len(grp) != 2 {
		return false, "unsupported_turn_shape"
	}
	if grp[0].Seq+1 != grp[1].Seq || grp[0].TurnSize != 2 {
		return false, "partial_turn"
	}
	users, assistants := 0, 0
	totalRunes := 0
	for _, row := range grp {
		switch {
		case row.ID == uuid.Nil:
			return false, "invalid_source"
		case row.Status != "completed":
			return false, "unsettled_source"
		case row.Role == "user":
			users++
		case row.Role == "assistant":
			assistants++
		default:
			return false, "unsupported_role"
		}
		text, pureText, ok := episodicSourceText(row.Parts, row.ContentText)
		if !ok {
			return false, "invalid_source"
		}
		// Only the user message is a quotable source, so only it must be
		// pure text. Assistant replies may carry non-text parts (quotes
		// and similar) because assistant content is never a quote source;
		// the writer still constrains every quote to a user message.
		if row.Role == "user" && !pureText {
			return false, "multimodal_source"
		}
		if strings.TrimSpace(text) == "" {
			return false, "empty_source"
		}
		if strings.TrimSpace(text) != text {
			return false, "padded_source"
		}
		if row.RunStatus != nil && *row.RunStatus != "completed" {
			return false, "unsettled_source"
		}
		if row.RunStatus != nil &&
			(row.RunUserID == nil || *row.RunUserID != owner ||
				row.RunConversationID == nil || *row.RunConversationID != conversationID) {
			return false, "foreign_source"
		}
		if row.ClinicalRefs != 0 {
			return false, "clinical_source"
		}
		size := utf8.RuneCountInString(text)
		if size > EpisodicShadowSourceMaxRunes {
			return false, "oversized_source"
		}
		totalRunes += size
	}
	if users != 1 || assistants != 1 {
		return false, "unsupported_turn_shape"
	}
	if totalRunes > EpisodicShadowSourceWindowRunes {
		return false, "oversized_source"
	}
	if overlapsAny(tombstones, grp[0].Seq, grp[1].Seq) {
		return false, "user_deleted"
	}
	return true, ""
}

// distillationContext returns up to two immediately preceding messages when
// they form one complete, text-only, non-clinical, non-tombstoned turn. The
// caller passes them only as untrusted model context; they never become note
// provenance and are never quotable.
func distillationContext(
	rows []episodicDistillationRow, from int, owner, conversationID uuid.UUID, tombstones []seqRange,
) []EpisodicDistillationMessage {
	lo, hi := from-2, from-1
	var previous []episodicDistillationRow
	for _, row := range rows {
		if row.Seq >= lo && row.Seq <= hi {
			previous = append(previous, row)
		}
	}
	if len(previous) != 2 || previous[0].TurnID != previous[1].TurnID ||
		previous[0].Seq+1 != previous[1].Seq ||
		overlapsAny(tombstones, previous[0].Seq, previous[1].Seq) {
		return nil
	}
	users, assistants := 0, 0
	for _, row := range previous {
		if row.Status != "completed" || row.Parts != "[]" || row.ContentBytes == 0 ||
			strings.TrimSpace(row.ContentText) != row.ContentText ||
			row.ClinicalRefs != 0 ||
			(row.RunStatus != nil && *row.RunStatus != "completed") ||
			(row.RunStatus != nil && (row.RunUserID == nil || *row.RunUserID != owner ||
				row.RunConversationID == nil || *row.RunConversationID != conversationID)) {
			return nil
		}
		if row.Role == "user" {
			users++
		} else if row.Role == "assistant" {
			assistants++
		}
	}
	if users != 1 || assistants != 1 {
		return nil
	}
	context := make([]EpisodicDistillationMessage, len(previous))
	for i, row := range previous {
		context[i] = EpisodicDistillationMessage{ID: row.ID, Seq: row.Seq, Role: row.Role, Text: row.ContentText}
	}
	return context
}

func (r *EpisodicDistillationRepository) ledgerRows(
	ctx context.Context, owner, conversationID uuid.UUID, afterSeq, throughSeq int,
) (map[seqRange]episodicDistillationLedgerRow, error) {
	var rows []episodicDistillationLedgerRow
	err := database.FromContext(ctx, r.db).Raw(`
SELECT from_seq, to_seq, status, attempts, note_id
FROM episodic_distillation_segments
WHERE user_id = ? AND conversation_id = ? AND to_seq > ? AND from_seq <= ?`,
		owner, conversationID, afterSeq, throughSeq).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[seqRange]episodicDistillationLedgerRow, len(rows))
	for _, row := range rows {
		out[seqRange{from: row.FromSeq, to: row.ToSeq}] = row
	}
	return out, nil
}

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
