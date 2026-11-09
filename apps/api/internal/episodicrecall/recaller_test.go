package episodicrecall

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

type fakeStore struct {
	notes []model.EpisodicNote
	err   error
	calls int
	delay time.Duration
}

func (f *fakeStore) ListOwnedRecallCandidates(ctx context.Context, owner uuid.UUID, limit int) ([]model.EpisodicNote, error) {
	f.calls++
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if f.err != nil {
		return nil, f.err
	}
	if len(f.notes) > limit {
		return f.notes[:limit], nil
	}
	return f.notes, nil
}

func testNote(text string, createdAt time.Time, sourceIDs ...uuid.UUID) model.EpisodicNote {
	raw := make([]string, len(sourceIDs))
	for i, id := range sourceIDs {
		raw[i] = id.String()
	}
	return model.EpisodicNote{
		ID:               uuid.New(),
		UserID:           uuid.New(),
		ConversationID:   uuid.New(),
		FromSeq:          1,
		ToSeq:            2,
		SourceMessageIDs: pq.StringArray(raw),
		ContentText:      text,
		Status:           "shadow",
		CreatedAt:        createdAt,
	}
}

func newTestRecaller(store CandidateStore) *Recaller {
	r := NewRecaller(store)
	r.logf = func(string, ...any) {}
	return r
}

func TestRecallRanksByOverlapThenRecency(t *testing.T) {
	now := time.Now().UTC()
	best := testNote("膝盖和肩膀都疼", now.Add(-time.Hour), uuid.New())
	middle := testNote("膝盖有点疼", now.Add(-time.Minute), uuid.New())
	weak := testNote("膝盖", now.Add(-2*time.Hour), uuid.New())
	unrelated := testNote("最近睡眠很好", now, uuid.New())
	store := &fakeStore{notes: []model.EpisodicNote{unrelated, weak, best, middle}}

	got := newTestRecaller(store).Recall(context.Background(), uuid.New(), "膝盖 肩膀")
	if len(got) != 3 {
		t.Fatalf("recalled %d notes, want 3 (unrelated filtered by threshold)", len(got))
	}
	if got[0].ID != best.ID || got[1].ID != middle.ID || got[2].ID != weak.ID {
		t.Fatalf("ranking order = %v, want best, middle, weak", []uuid.UUID{got[0].ID, got[1].ID, got[2].ID})
	}
	// Injected text is the canonical note text, untouched.
	if got[0].Text != best.ContentText {
		t.Fatalf("text was rewritten: %q", got[0].Text)
	}
	if len(got[0].SourceMessageIDs) != 1 || got[0].FromSeq != 1 || got[0].ToSeq != 2 {
		t.Fatalf("provenance missing: %+v", got[0])
	}
}

func TestRecallCapsAtTopK(t *testing.T) {
	now := time.Now().UTC()
	notes := make([]model.EpisodicNote, 0, TopK+4)
	for i := 0; i < TopK+4; i++ {
		notes = append(notes, testNote("膝盖", now.Add(time.Duration(i)*time.Minute), uuid.New()))
	}
	got := newTestRecaller(&fakeStore{notes: notes}).Recall(context.Background(), uuid.New(), "膝盖")
	if len(got) != TopK {
		t.Fatalf("recalled %d notes, want top K = %d", len(got), TopK)
	}
}

func TestRecallHardTimeoutDegradesSilently(t *testing.T) {
	store := &fakeStore{delay: time.Second, notes: []model.EpisodicNote{testNote("膝盖", time.Now(), uuid.New())}}
	started := time.Now()
	got := newTestRecaller(store).Recall(context.Background(), uuid.New(), "膝盖")
	elapsed := time.Since(started)
	if len(got) != 0 {
		t.Fatalf("timeout returned %d notes, want silent empty result", len(got))
	}
	if elapsed > RecallTimeout+50*time.Millisecond {
		t.Fatalf("timeout took %s, want <= %s (+tolerance)", elapsed, RecallTimeout)
	}
	if store.calls != 1 {
		t.Fatalf("store calls = %d, want 1", store.calls)
	}
}

func TestRecallStoreErrorDegradesSilently(t *testing.T) {
	got := newTestRecaller(&fakeStore{err: errors.New("db unavailable")}).
		Recall(context.Background(), uuid.New(), "膝盖")
	if len(got) != 0 {
		t.Fatalf("error returned %d notes, want empty", len(got))
	}
}

func TestRecallSkipsNotesWithUnverifiableProvenance(t *testing.T) {
	bad := testNote("膝盖疼", time.Now(), uuid.New())
	bad.SourceMessageIDs = pq.StringArray{"not-a-uuid"}
	good := testNote("膝盖疼", time.Now().Add(-time.Hour), uuid.New())
	got := newTestRecaller(&fakeStore{notes: []model.EpisodicNote{bad, good}}).
		Recall(context.Background(), uuid.New(), "膝盖")
	if len(got) != 1 || got[0].ID != good.ID {
		t.Fatalf("unverifiable provenance was served: %+v", got)
	}
}

func TestRecallSkipsQueryWithoutSubjectTerms(t *testing.T) {
	store := &fakeStore{notes: []model.EpisodicNote{testNote("膝盖", time.Now(), uuid.New())}}
	for _, query := range []string{"", "   "} {
		if got := newTestRecaller(store).Recall(context.Background(), uuid.New(), query); len(got) != 0 {
			t.Fatalf("query %q recalled %d notes, want 0", query, len(got))
		}
	}
	if store.calls != 0 {
		t.Fatalf("empty queries hit the store %d times, want 0", store.calls)
	}
}

func TestRecallRejectsNilOwner(t *testing.T) {
	store := &fakeStore{notes: []model.EpisodicNote{testNote("膝盖", time.Now(), uuid.New())}}
	if got := newTestRecaller(store).Recall(context.Background(), uuid.Nil, "膝盖"); len(got) != 0 {
		t.Fatalf("nil owner returned %d notes", len(got))
	}
	if store.calls != 0 {
		t.Fatalf("nil owner hit the store %d times, want 0", store.calls)
	}
}
