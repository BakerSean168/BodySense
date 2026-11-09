package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// runtimeSourceTurn rewrites a synthetic turn into the shapes the runtime
// actually persists: the user body lives only in a text part with an empty
// content_text, and the assistant body lives in content_text beside a quote
// part. Requires BODYSENSE_INTEGRATION_DATABASE_URL through the fixture.
func runtimeSourceTurn(t *testing.T, f *retentionFixture, turn retentionTurn) {
	t.Helper()
	f.exec(`UPDATE messages SET content_text='', parts='[{"text":"请简短回答。","type":"text"}]'::jsonb WHERE id=?`,
		turn.user)
	f.exec(`UPDATE messages SET parts=?::jsonb WHERE id=?`,
		`[{"type":"quote","message_id":"`+turn.user.String()+`"},{"type":"text","text":"请简短回答。"}]`,
		turn.reply)
}

// The planner must admit the real shape and expose the canonical user body as
// the only quotable source; the writer must then accept a draft that quotes
// that canonical text while the assistant carries non-text parts.
func TestEpisodicSourceShapePlansAndWritesRuntimeTurnPostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	f := newRetentionFixture(t, db)
	ctx := context.Background()
	turn := f.addTurn("completed", "synthetic runtime shape")
	runtimeSourceTurn(t, f, turn)
	f.advance(2, 3)

	repo := NewEpisodicDistillationRepository(db)
	segments, err := repo.Segments(ctx, f.owner, f.conversation, 0, 2)
	if err != nil || len(segments) != 1 {
		t.Fatalf("segments=%d %v", len(segments), err)
	}
	seg := segments[0]
	if !seg.Eligible || seg.Reason != "" {
		t.Fatalf("runtime shape rejected: %+v", seg)
	}
	if len(seg.Messages) != 2 || seg.Messages[0].Text != "请简短回答。" ||
		seg.Messages[1].Text != "reply about synthetic runtime shape" {
		t.Fatalf("canonical source text=%+v", seg.Messages)
	}

	writer := NewEpisodicShadowRepository(db).WithShadowAdmissionEnabled(true)
	draft := EpisodicShadowDraft{
		ConversationID: f.conversation, FromSeq: seg.FromSeq, ToSeq: seg.ToSeq,
		SourceMessageIDs: []uuid.UUID{turn.user, turn.reply},
		GeneratorVersion: "synthetic-v1", RetentionPolicyVersion: "shadow-policy-v1",
		ContentText:     "对话偏好：回答简短。",
		QuotedMessageID: turn.user, QuotedMessageText: "请简短回答。",
	}
	note, created, err := writer.SaveShadowDraft(ctx, f.owner, draft)
	if err != nil || !created || note.ContentText != "对话偏好：回答简短。" {
		t.Fatalf("runtime shape write: note=%+v created=%v err=%v", note, created, err)
	}
}

// The canonical-text comparison still fails closed: a quote that is not
// byte-for-byte the whole canonical user message is rejected, and a quote can
// never target the assistant reply even when the text matches.
func TestEpisodicSourceShapeWriterRejectsNonCanonicalQuotesPostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	f := newRetentionFixture(t, db)
	ctx := context.Background()
	turn := f.addTurn("completed", "synthetic runtime shape")
	runtimeSourceTurn(t, f, turn)
	f.advance(2, 3)

	writer := NewEpisodicShadowRepository(db).WithShadowAdmissionEnabled(true)
	base := EpisodicShadowDraft{
		ConversationID: f.conversation, FromSeq: 1, ToSeq: 2,
		SourceMessageIDs: []uuid.UUID{turn.user, turn.reply},
		GeneratorVersion: "synthetic-v1", RetentionPolicyVersion: "shadow-policy-v1",
		ContentText: "对话偏好：回答简短。",
	}
	mismatched := base
	// "请用中文回答。" is a valid approved quote, but not this source row.
	mismatched.QuotedMessageID = turn.user
	mismatched.QuotedMessageText = "请用中文回答。"
	mismatched.ContentText = "对话偏好：使用中文。"
	if _, _, err := writer.SaveShadowDraft(ctx, f.owner, mismatched); !errors.Is(err, ErrEpisodicSourceInvalid) {
		t.Fatalf("non-canonical quote accepted: %v", err)
	}

	assistantQuote := base
	assistantQuote.QuotedMessageID = turn.reply
	assistantQuote.QuotedMessageText = "请简短回答。"
	if _, _, err := writer.SaveShadowDraft(ctx, f.owner, assistantQuote); !errors.Is(err, ErrEpisodicSourceInvalid) {
		t.Fatalf("assistant quote accepted: %v", err)
	}

	if count := f.count("SELECT count(*) FROM episodic_notes WHERE user_id=?", f.owner); count != 0 {
		t.Fatalf("rejected drafts persisted: %d", count)
	}
}

// A runtime user message whose parts contain a non-text fragment is still
// multimodal_source, even though it has no legacy content_text fallback.
func TestEpisodicSourceShapePlannerRejectsUserNonTextPartsPostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	f := newRetentionFixture(t, db)
	turn := f.addTurn("completed", "synthetic multimodal user")
	f.exec(`UPDATE messages SET content_text='', parts='[{"type":"image","upload_id":"00000000-0000-4000-8000-000000000001"}]'::jsonb WHERE id=?`,
		turn.user)
	repo := NewEpisodicDistillationRepository(db)
	segments, err := repo.Segments(context.Background(), f.owner, f.conversation, 0, 2)
	if err != nil || len(segments) != 1 {
		t.Fatalf("segments=%d %v", len(segments), err)
	}
	if segments[0].Eligible || segments[0].Reason != "multimodal_source" {
		t.Fatalf("user non-text parts admitted: %+v", segments[0])
	}
}
