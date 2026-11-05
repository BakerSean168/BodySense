package repository

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Requires BODYSENSE_INTEGRATION_DATABASE_URL; openSingletonTestDB reads it
// through the shared fixture. Inventory this file for both PR and Main CI.
// S6-A uses only disposable PostgreSQL 18 and fabricated chat content.
// The repository is deliberately not reachable from an HTTP route or UI.
func TestEpisodicNoteControlsOwnerDeleteAndNoRegenerationPostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	f := newRetentionFixture(t, db)
	other := newRetentionFixture(t, db)
	ctx := context.Background()
	turn1 := f.addTurn("completed", "synthetic first reply preference")
	turn2 := f.addTurn("completed", "synthetic second reply preference")
	turn3 := f.addTurn("completed", "synthetic unrelated reply preference")
	f.advance(6, 7)
	writer := NewEpisodicShadowRepository(db).WithShadowAdmissionEnabled(true)
	controls := NewEpisodicNoteControlsRepository(db)
	note1, created, err := writer.SaveShadowDraft(ctx, f.owner, shadowDraft(f, turn1))
	if err != nil || !created {
		t.Fatalf("seed first note: %v", err)
	}
	overlap := shadowDraft(f, turn1)
	overlap.ToSeq = turn2.replySeq
	overlap.SourceMessageIDs = []uuid.UUID{turn1.user, turn1.reply, turn2.user, turn2.reply}
	overlap.ContentText = "Synthetic combined reply preference."
	if _, created, err := writer.SaveShadowDraft(ctx, f.owner, overlap); err != nil || !created {
		t.Fatalf("seed overlapping note: %v", err)
	}
	lastNote, created, err := writer.SaveShadowDraft(ctx, f.owner, shadowDraft(f, turn3))
	if err != nil || !created {
		t.Fatalf("seed unrelated note: %v", err)
	}
	otherTurn := other.addTurn("completed", "synthetic foreign source")
	other.advance(2, 3)
	foreignNote, created, err := writer.SaveShadowDraft(ctx, other.owner, shadowDraft(other, otherTurn))
	if err != nil || !created {
		t.Fatalf("seed other owner: %v", err)
	}
	list, err := controls.ListOwned(ctx, f.owner)
	if err != nil || len(list) != 3 {
		t.Fatalf("owned note list: %d %v", len(list), err)
	}
	for _, n := range list {
		if n.UserID != f.owner || n.Status != "shadow" {
			t.Fatalf("owner scoping violated: %+v", n)
		}
	}
	foreign, err := controls.ListOwned(ctx, other.owner)
	if err != nil || len(foreign) != 1 || foreign[0].ID != foreignNote.ID {
		t.Fatalf("other owner list: %+v %v", foreign, err)
	}
	if _, err := controls.ListOwned(ctx, uuid.Nil); !errors.Is(err, ErrEpisodicNoteNotFound) {
		t.Fatalf("nil owner was not rejected: %v", err)
	}
	if _, err := controls.DeleteOwned(ctx, f.owner, foreignNote.ID); !errors.Is(err, ErrEpisodicNoteNotFound) {
		t.Fatalf("foreign deletion leaked note existence: %v", err)
	}
	if _, err := controls.DeleteOwned(ctx, other.owner, note1.ID); !errors.Is(err, ErrEpisodicNoteNotFound) {
		t.Fatalf("reverse foreign delete: %v", err)
	}
	if _, err := controls.DeleteOwned(ctx, f.owner, uuid.New()); !errors.Is(err, ErrEpisodicNoteNotFound) {
		t.Fatalf("unknown id was accepted: %v", err)
	}
	if _, err := controls.DeleteOwned(ctx, uuid.Nil, note1.ID); !errors.Is(err, ErrEpisodicNoteNotFound) {
		t.Fatalf("unauthenticated caller was accepted: %v", err)
	}
	// An adjacent BodyState evidence record is authoritative and MUST NOT
	// be modified by a nonclinical memory-note deletion.
	evidenceID := f.addEvidence(f.owner, turn3)
	removed, err := controls.DeleteOwned(ctx, f.owner, note1.ID)
	if err != nil || !removed {
		t.Fatalf("owner delete: %v %v", removed, err)
	}
	list, err = controls.ListOwned(ctx, f.owner)
	if err != nil || len(list) != 1 || list[0].ID != lastNote.ID {
		t.Fatalf("overlapping copies survived user deletion: %+v %v", list, err)
	}
	foreign, err = controls.ListOwned(ctx, other.owner)
	if err != nil || len(foreign) != 1 || foreign[0].ID != foreignNote.ID {
		t.Fatalf("delete touched another owner: %+v %v", foreign, err)
	}
	if f.count("SELECT COUNT(*) FROM statement_evidence WHERE id=?", evidenceID) != 1 ||
		f.count("SELECT COUNT(*) FROM messages WHERE conversation_id=?", f.conversation) != 6 {
		t.Fatal("memory deletion mutated authoritative evidence or raw chat")
	}
	// Repeat request succeeds but does not create a second tombstone.
	removed, err = controls.DeleteOwned(ctx, f.owner, note1.ID)
	if err != nil || removed {
		t.Fatalf("repeat deletion is not idempotent: %v %v", removed, err)
	}
	if f.count("SELECT COUNT(*) FROM episodic_note_deletions WHERE user_id=?", f.owner) != 1 {
		t.Fatal("missing or duplicate owner delete marker")
	}
	var tomb EpisodicNoteDeletion
	if err := db.Where("user_id=? AND note_id=?", f.owner, note1.ID).Take(&tomb).Error; err != nil {
		t.Fatal(err)
	}
	if tomb.FromSeq != turn1.userSeq || tomb.ToSeq != turn1.replySeq ||
		tomb.ConversationID != f.conversation || tomb.DeletedAt.IsZero() {
		t.Fatalf("invalid content-free deletion marker: %+v", tomb)
	}
	// The schema has no columns with source content or message IDs.
	var columns []string
	if err := db.Raw(`SELECT column_name FROM information_schema.columns
WHERE table_schema='public' AND table_name='episodic_note_deletions'`).Scan(&columns).Error; err != nil {
		t.Fatal(err)
	}
	for _, column := range columns {
		if strings.Contains(column, "text") || strings.Contains(column, "content") ||
			strings.Contains(column, "message_id") || strings.Contains(column, "asset_id") {
			t.Fatalf("user deletion retained source content: %s", column)
		}
	}
	// Go rejects retries and *new* generation policy/version over the same
	// source or an expanded window, even if raw messages still exist.
	for _, retry := range []EpisodicShadowDraft{
		shadowDraft(f, turn1),
		func() EpisodicShadowDraft {
			d := shadowDraft(f, turn1)
			d.GeneratorVersion = "synthetic-v2"
			return d
		}(),
		overlap,
	} {
		if _, _, err := writer.SaveShadowDraft(ctx, f.owner, retry); !errors.Is(err, ErrEpisodicNoteUserDeleted) {
			t.Fatalf("deleted content was regenerated: %+v %v", retry, err)
		}
	}
	// Direct metadata edits must not mutate an unrelated surviving note
	// into a new copy of the deleted source window.
	err = db.Exec("UPDATE episodic_notes SET from_seq=?, to_seq=? WHERE user_id=? AND id=?",
		turn1.userSeq, turn1.replySeq, f.owner, lastNote.ID).Error
	if err == nil || !strings.Contains(err.Error(), "user-deleted") {
		t.Fatalf("DB update bypassed deleted source suppression: %v", err)
	}
	// A direct DB write also fails closed, bypassing any Go generator.
	err = db.Exec(`INSERT INTO episodic_notes
(user_id,conversation_id,from_seq,to_seq,source_message_ids,
 generator_version,retention_policy_version,content_text,status)
VALUES (?,?,?,?,?,'synthetic-forged-v2','synthetic-new-policy','regenerated copied note','shadow')`,
		f.owner, f.conversation, turn1.userSeq, turn2.replySeq,
		"{"+turn1.user.String()+","+turn1.reply.String()+","+turn2.user.String()+","+turn2.reply.String()+"}").Error
	if err == nil || !strings.Contains(err.Error(), "user-deleted") {
		t.Fatalf("DB trigger permitted user-deleted memory revival: %v", err)
	}
	// Future, disjoint source ranges are not suppressed by an older delete.
	turn4 := f.addTurn("completed", "later synthetic unrelated source")
	f.advance(8, 9)
	// Avoid a new note for the third turn, which is now clinical evidence.
	if _, created, err := writer.SaveShadowDraft(ctx, f.owner, shadowDraft(f, turn4)); err != nil || !created {
		t.Fatalf("disjoint future note was blocked: %v %v", created, err)
	}
	// Tombstone and note both cascade on account erasure.
	f.exec("DELETE FROM users WHERE id=?", f.owner)
	if f.count("SELECT COUNT(*) FROM episodic_note_deletions WHERE user_id=?", f.owner) != 0 ||
		f.count("SELECT COUNT(*) FROM episodic_notes WHERE user_id=?", f.owner) != 0 {
		t.Fatal("account erasure retained content or deletion marker")
	}
}

func TestEpisodicNoteDeletionMigrationRefusesRollbackWithTombstonePostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	f := newRetentionFixture(t, db)
	turn := f.addTurn("completed", "synthetic rollback gate")
	f.advance(2, 3)
	writer := NewEpisodicShadowRepository(db).WithShadowAdmissionEnabled(true)
	note, _, err := writer.SaveShadowDraft(context.Background(), f.owner, shadowDraft(f, turn))
	if err != nil {
		t.Fatal(err)
	}
	controls := NewEpisodicNoteControlsRepository(db)
	if _, err := controls.DeleteOwned(context.Background(), f.owner, note.ID); err != nil {
		t.Fatal(err)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve migration location")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "migrations", "000006_episodic_user_deletion.down.sql")
	migration, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(string(migration)).Error; err == nil ||
		!strings.Contains(err.Error(), "cannot drop episodic user-deletion tombstones") {
		t.Fatalf("unsafe rollback unexpectedly succeeded: %v", err)
	}
	if f.count("SELECT COUNT(*) FROM episodic_note_deletions WHERE user_id=?", f.owner) != 1 {
		t.Fatal("rejected downgrade removed deletion suppression")
	}
	if _, _, err := writer.SaveShadowDraft(context.Background(), f.owner, shadowDraft(f, turn)); !errors.Is(err, ErrEpisodicNoteUserDeleted) {
		t.Fatalf("downgrade reopened previously deleted content: %v", err)
	}
}

func TestEpisodicNoteControlsDeletionWaitsForConversationLockPostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	f := newRetentionFixture(t, db)
	turn := f.addTurn("completed", "synthetic lock proof")
	f.advance(2, 3)
	writer := NewEpisodicShadowRepository(db).WithShadowAdmissionEnabled(true)
	note, _, err := writer.SaveShadowDraft(context.Background(), f.owner, shadowDraft(f, turn))
	if err != nil {
		t.Fatal(err)
	}
	controls := NewEpisodicNoteControlsRepository(db)
	hold := db.Begin()
	if hold.Error != nil {
		t.Fatal(hold.Error)
	}
	if err := hold.Exec("SELECT id FROM conversations WHERE id=? FOR NO KEY UPDATE", f.conversation).Error; err != nil {
		_ = hold.Rollback()
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 160*time.Millisecond)
	defer cancel()
	if _, err := controls.DeleteOwned(ctx, f.owner, note.ID); err == nil {
		_ = hold.Rollback()
		t.Fatal("note deletion bypassed the owner's Conversation lock")
	}
	_ = hold.Rollback()
	if f.count("SELECT COUNT(*) FROM episodic_notes WHERE id=?", note.ID) != 1 ||
		f.count("SELECT COUNT(*) FROM episodic_note_deletions WHERE note_id=?", note.ID) != 0 {
		t.Fatal("failed concurrent delete left a partial tombstone")
	}
	if removed, err := controls.DeleteOwned(context.Background(), f.owner, note.ID); err != nil || !removed {
		t.Fatalf("retry after lock release: %v %v", removed, err)
	}
}
