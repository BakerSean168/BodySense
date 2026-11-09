package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/bodysense/api/internal/episodicrecall"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// S4 candidate reads and the S6-A deletion tombstone must agree: a completed
// user deletion can never be resurrected by serving. Everything here is
// synthetic and runs only against the disposable integration database.
func TestEpisodicRecallExcludesUserDeletedNotesPostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	f := newRetentionFixture(t, db)
	ctx := context.Background()
	writer := NewEpisodicShadowRepository(db).WithShadowAdmissionEnabled(true)

	first := f.addTurn("completed", "synthetic morning market chat")
	f.advance(2, 3)
	market := shadowDraft(f, first)
	market.ContentText = "用户提到周末去了早市买菜。"
	marketNote, created, err := writer.SaveShadowDraft(ctx, f.owner, market)
	if err != nil || !created {
		t.Fatalf("seed market note: %v", err)
	}

	second := f.addTurn("completed", "synthetic morning market follow-up")
	f.advance(4, 5)
	noodle := shadowDraft(f, second)
	noodle.ContentText = "用户提到早市附近有一家面馆。"
	noodleNote, created, err := writer.SaveShadowDraft(ctx, f.owner, noodle)
	if err != nil || !created {
		t.Fatalf("seed noodle note: %v", err)
	}

	third := f.addTurn("completed", "synthetic unrelated sleep chat")
	f.advance(6, 7)
	sleep := shadowDraft(f, third)
	sleep.ContentText = "用户说最近睡眠很好。"
	if _, created, err := writer.SaveShadowDraft(ctx, f.owner, sleep); err != nil || !created {
		t.Fatalf("seed sleep note: %v", err)
	}

	// A second owner with an identical-topic note must never leak.
	other := newRetentionFixture(t, db)
	otherTurn := other.addTurn("completed", "synthetic foreign market chat")
	other.advance(2, 3)
	foreign := shadowDraft(other, otherTurn)
	foreign.ContentText = "另一位用户提到周末去了早市。"
	foreignNote, created, err := writer.SaveShadowDraft(ctx, other.owner, foreign)
	if err != nil || !created {
		t.Fatalf("seed foreign note: %v", err)
	}

	recaller := episodicrecall.NewRecaller(NewEpisodicRecallRepository(db))
	before := recaller.Recall(ctx, f.owner, "你还记得我去早市的事吗")
	if len(before) != 2 {
		t.Fatalf("recall before deletion = %d notes, want 2", len(before))
	}
	for _, note := range before {
		if note.ID == foreignNote.ID {
			t.Fatalf("foreign owner note was recalled")
		}
		if note.Text != "用户提到周末去了早市买菜。" && note.Text != "用户提到早市附近有一家面馆。" {
			t.Fatalf("unexpected recalled text: %q", note.Text)
		}
	}

	controls := NewEpisodicNoteControlsRepository(db)
	if _, err := controls.DeleteOwned(ctx, f.owner, marketNote.ID); err != nil {
		t.Fatalf("delete market note: %v", err)
	}

	after := recaller.Recall(ctx, f.owner, "你还记得我去早市的事吗")
	if len(after) != 1 || after[0].ID != noodleNote.ID {
		t.Fatalf("ghost memory after deletion: %+v", after)
	}

	// A retry of the same source window is also blocked, so the deletion is
	// not merely a one-time row removal.
	if _, _, err := writer.SaveShadowDraft(ctx, f.owner, market); !errors.Is(err, ErrEpisodicNoteUserDeleted) {
		t.Fatalf("deleted window regenerated: %v", err)
	}
	if again := recaller.Recall(ctx, f.owner, "你还记得我去早市的事吗"); len(again) != 1 {
		t.Fatalf("recall after regeneration attempt = %d, want 1", len(again))
	}
}

func TestEpisodicRecallCandidateReadIsBoundedAndOwnerScopedPostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	f := newRetentionFixture(t, db)
	ctx := context.Background()
	writer := NewEpisodicShadowRepository(db).WithShadowAdmissionEnabled(true)

	turn := f.addTurn("completed", "synthetic bounded read chat")
	f.advance(2, 3)
	draft := shadowDraft(f, turn)
	draft.ContentText = "用户提到早市买菜。"
	if _, created, err := writer.SaveShadowDraft(ctx, f.owner, draft); err != nil || !created {
		t.Fatalf("seed note: %v", err)
	}

	repo := NewEpisodicRecallRepository(db)
	notes, err := repo.ListOwnedRecallCandidates(ctx, f.owner, 8)
	if err != nil {
		t.Fatalf("list candidates: %v", err)
	}
	if len(notes) != 1 || notes[0].UserID != f.owner || notes[0].Status != "shadow" {
		t.Fatalf("candidates = %+v", notes)
	}
	if notes[0].CreatedAt.IsZero() {
		t.Fatalf("candidate missing created_at")
	}
	if empty, err := repo.ListOwnedRecallCandidates(ctx, uuid.Nil, 8); err != nil || len(empty) != 0 {
		t.Fatalf("nil owner read = %v %v", empty, err)
	}
	// A caller cannot widen the read past its own bounded request.
	if none, err := repo.ListOwnedRecallCandidates(ctx, f.owner, 0); err != nil || len(none) != 0 {
		t.Fatalf("zero limit read = %v %v", none, err)
	}

	var stored model.EpisodicNote
	if err := db.Where("id = ?", notes[0].ID).Take(&stored).Error; err != nil {
		t.Fatalf("read stored note: %v", err)
	}
	if stored.ContentText != draft.ContentText {
		t.Fatalf("stored text differs from injected text")
	}
}
