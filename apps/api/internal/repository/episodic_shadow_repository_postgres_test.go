package repository

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

func shadowDraft(f *retentionFixture, turn retentionTurn) EpisodicShadowDraft {
	return EpisodicShadowDraft{
		ConversationID: f.conversation,
		FromSeq:        turn.userSeq, ToSeq: turn.replySeq,
		SourceMessageIDs: []uuid.UUID{turn.user, turn.reply},
		GeneratorVersion: "synthetic-v1", RetentionPolicyVersion: "shadow-policy-v1",
		ContentText: "Synthetic preference for short coaching summaries.",
	}
}

// All input is synthetic and the writer is explicitly enabled only inside
// this isolated test. No Python model, clinical data or serving path is used.
func TestEpisodicShadowAdmissionStrictProvenancePostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	f := newRetentionFixture(t, db)
	ctx := context.Background()
	first := f.addTurn("completed", "synthetic conversation preference")
	second := f.addTurn("completed", "synthetic unrelated preference")
	f.advance(4, 5)
	writer := NewEpisodicShadowRepository(db)
	d := shadowDraft(f, first)

	if _, _, err := writer.SaveShadowDraft(ctx, f.owner, d); !errors.Is(err, ErrEpisodicShadowDisabled) {
		t.Fatalf("default-off admission: %v", err)
	}
	writer.WithShadowAdmissionEnabled(true)
	note, created, err := writer.SaveShadowDraft(ctx, f.owner, d)
	if err != nil || !created || note.ID == uuid.Nil || note.Status != "shadow" ||
		note.UserID != f.owner || note.FromSeq != 1 || note.ToSeq != 2 ||
		len(note.SourceAssetIDs) != 0 {
		t.Fatalf("admit: note=%+v created=%v err=%v", note, created, err)
	}
	again, newRow, err := writer.SaveShadowDraft(ctx, f.owner, d)
	if err != nil || newRow || again.ID != note.ID {
		t.Fatalf("idempotent generation retry: %+v %v %v", again, newRow, err)
	}
	changed := d
	changed.ContentText = "a different synthetic claim"
	if _, _, err := writer.SaveShadowDraft(ctx, f.owner, changed); !errors.Is(err, ErrEpisodicDraftConflict) {
		t.Fatalf("version-and-range collision accepted: %v", err)
	}
	foreign := createSingletonTestUser(t, db)
	if _, _, err := writer.SaveShadowDraft(ctx, foreign, d); !errors.Is(err, ErrEpisodicSourceInvalid) {
		t.Fatalf("another user admitted: %v", err)
	}
	for _, bad := range []EpisodicShadowDraft{
		{ConversationID: f.conversation, FromSeq: 1, ToSeq: 3,
			SourceMessageIDs: []uuid.UUID{first.user, first.reply}, ContentText: "invalid"},
		func() EpisodicShadowDraft { x := shadowDraft(f, second); x.SourceMessageIDs[0] = first.user; return x }(),
		func() EpisodicShadowDraft { x := shadowDraft(f, second); x.SourceMessageIDs[1] = second.user; return x }(),
		func() EpisodicShadowDraft {
			x := shadowDraft(f, second)
			x.SourceAssetIDs = []uuid.UUID{uuid.New()}
			return x
		}(),
		func() EpisodicShadowDraft {
			x := shadowDraft(f, second)
			x.ContentText = strings.Repeat("a", episodicMaxNoteRunes+1)
			return x
		}(),
	} {
		if _, _, err := writer.SaveShadowDraft(ctx, f.owner, bad); !errors.Is(err, ErrEpisodicSourceInvalid) {
			t.Fatalf("bad source accepted: %+v err=%v", bad, err)
		}
	}

	// A syntactically complete range can still cut two distinct turns in
	// half (assistant from one and user from the next). Reject that range.
	clipped := shadowDraft(f, second)
	clipped.FromSeq, clipped.ToSeq = first.replySeq, second.userSeq
	clipped.SourceMessageIDs = []uuid.UUID{first.reply, second.user}
	if _, _, err := writer.SaveShadowDraft(ctx, f.owner, clipped); !errors.Is(err, ErrEpisodicSourceInvalid) {
		t.Fatalf("partial turns admitted: %v", err)
	}
	// A statement-evidence-backed message is authoritative domain input:
	// do not fork its content into a narrative shadow note.
	f.addEvidence(f.owner, second)
	if _, _, err := writer.SaveShadowDraft(ctx, f.owner, shadowDraft(f, second)); !errors.Is(err, ErrEpisodicSourceInvalid) {
		t.Fatalf("BodyState evidence copied into a shadow note: %v", err)
	}
	if f.count("SELECT count(*) FROM episodic_notes WHERE user_id=?", f.owner) != 1 {
		t.Fatal("rejected draft persisted")
	}
	// Normal chat pruning never pins a raw message merely because a note
	// references its UUID. The saved note continues to carry soft provenance.
	if receipt, err := f.prune(ctx, f.owner); err != nil || receipt.PrunedThroughSeq < 2 {
		t.Fatalf("prune was blocked by soft provenance: %+v %v", receipt, err)
	}
	if f.messageExists(first.user) || f.messageExists(first.reply) {
		t.Fatal("episodic source unexpectedly protected raw chat")
	}
	after, newRow, err := writer.SaveShadowDraft(ctx, f.owner, d)
	if err != nil || newRow || after.ID != note.ID {
		t.Fatalf("post-prune idempotent replay: %+v %v %v", after, newRow, err)
	}
}

func TestEpisodicShadowAdmissionRejectsStaleMultimodalAndBusySourcesPostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	f := newRetentionFixture(t, db)
	ctx := context.Background()
	first := f.addTurn("completed", "synthetic first")
	second := f.addTurn("completed", "synthetic second")
	writer := NewEpisodicShadowRepository(db).WithShadowAdmissionEnabled(true)
	if _, _, err := writer.SaveShadowDraft(ctx, f.owner, shadowDraft(f, first)); !errors.Is(err, ErrEpisodicSourceInvalid) {
		t.Fatalf("source not past context boundary was admitted: %v", err)
	}
	f.advance(4, 5)
	// Unknown multimodal references are never silently dropped from the
	// source inventory. S2-B will require typed explicit asset provenance.
	if err := db.Exec(`UPDATE messages SET parts =
'[{"type":"image","upload_id":"00000000-0000-4000-8000-000000000001"}]'::jsonb WHERE id=?`, first.user).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := writer.SaveShadowDraft(ctx, f.owner, shadowDraft(f, first)); !errors.Is(err, ErrEpisodicSourceInvalid) {
		t.Fatalf("asset-bearing message without declared provenance admitted: %v", err)
	}
	// A settled source can become stale before a generated draft commits.
	if err := db.Exec("UPDATE messages SET status='streaming' WHERE id=?", second.reply).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := writer.SaveShadowDraft(ctx, f.owner, shadowDraft(f, second)); !errors.Is(err, ErrEpisodicSourceInvalid) {
		t.Fatalf("unsettled source admitted: %v", err)
	}
	if err := db.Exec("UPDATE messages SET status='completed' WHERE id=?", second.reply).Error; err != nil {
		t.Fatal(err)
	}
	// The Conversation-row lock serializes the writer and chat pruning.
	hold := db.Begin()
	if hold.Error != nil {
		t.Fatal(hold.Error)
	}
	if err := hold.Exec("SELECT id FROM conversations WHERE id=? FOR NO KEY UPDATE", f.conversation).Error; err != nil {
		_ = hold.Rollback()
		t.Fatal(err)
	}
	timeout, cancel := context.WithTimeout(ctx, 160*time.Millisecond)
	defer cancel()
	if _, _, err := writer.SaveShadowDraft(timeout, f.owner, shadowDraft(f, second)); err == nil {
		_ = hold.Rollback()
		t.Fatal("shadow write ignored held conversation lock")
	}
	_ = hold.Rollback()
	if f.count("SELECT count(*) FROM episodic_notes WHERE user_id=?", f.owner) != 0 {
		t.Fatal("failed/busy admission left an unverified note")
	}
	if note, created, err := writer.SaveShadowDraft(ctx, f.owner, shadowDraft(f, second)); err != nil || !created || note.Status != "shadow" {
		t.Fatalf("retry after lock release: note=%+v created=%v err=%v", note, created, err)
	}
}

func TestEpisodicShadowAdmissionRollsOldestNotesWithinPolicyCapPostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	f := newRetentionFixture(t, db)
	ctx := context.Background()
	turns := make([]retentionTurn, 0, episodicMaxNotesPerUser+3)
	for i := 0; i < episodicMaxNotesPerUser+3; i++ {
		turns = append(turns, f.addTurn("completed", "synthetic preference"))
	}
	f.advance(f.seq, f.seq+1)
	writer := NewEpisodicShadowRepository(db).WithShadowAdmissionEnabled(true)
	for _, turn := range turns {
		if _, created, err := writer.SaveShadowDraft(ctx, f.owner, shadowDraft(f, turn)); err != nil || !created {
			t.Fatalf("generated seq %d: %v %v", turn.userSeq, created, err)
		}
	}
	if count := f.count("SELECT count(*) FROM episodic_notes WHERE user_id=?", f.owner); count != episodicMaxNotesPerUser {
		t.Fatalf("unbounded or prematurely removed notes: %d", count)
	}
	if count := f.count("SELECT count(*) FROM episodic_notes WHERE user_id=? AND from_seq=?", f.owner, turns[0].userSeq); count != 0 {
		t.Fatal("oldest note did not roll off")
	}
	if count := f.count("SELECT count(*) FROM episodic_notes WHERE user_id=? AND from_seq=?", f.owner, turns[len(turns)-1].userSeq); count != 1 {
		t.Fatal("newest note was removed during roll-off")
	}
	// Every note remains non-serving. No diagnosis/safety consumer reads it.
	var statuses []string
	if err := db.Model(&model.EpisodicNote{}).Distinct("status").Pluck("status", &statuses).Error; err != nil || len(statuses) != 1 || statuses[0] != "shadow" {
		t.Fatalf("non-shadow memory status: %v %v", statuses, err)
	}
}
