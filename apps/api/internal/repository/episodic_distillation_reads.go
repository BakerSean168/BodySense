package repository

import (
	"context"
	"errors"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

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
