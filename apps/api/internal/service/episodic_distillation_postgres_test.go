package service

import (
	"context"
	"errors"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"strings"
	"sync"
	"testing"
)

// fakeEpisodicProposer is the only "model" in these tests. It never opens a
// network connection and only returns a proposal built from the Go-selected
// source window. Requires BODYSENSE_INTEGRATION_DATABASE_URL; every fixture is
// synthetic and every provider call is faked.
type fakeEpisodicProposer struct {
	mu    sync.Mutex
	calls []EpisodicShadowProposalRequest
	err   error
	quote bool
}

func (f *fakeEpisodicProposer) ProposeEpisodicShadowQuote(
	_ context.Context, request EpisodicShadowProposalRequest,
) (EpisodicShadowProposalResponse, error) {
	f.mu.Lock()
	f.calls = append(f.calls, request)
	err, quote := f.err, f.quote
	f.mu.Unlock()
	if err != nil {
		return EpisodicShadowProposalResponse{}, err
	}
	if !quote {
		return EpisodicShadowProposalResponse{}, nil
	}
	for _, source := range request.Source {
		if source.Role != "user" {
			continue
		}
		canonical, ok := repository.CanonicalEpisodicPreference(source.Text)
		if !ok {
			continue
		}
		return EpisodicShadowProposalResponse{Quotes: []EpisodicShadowQuoteSuggestion{{
			SourceMessageID: source.ID, Quote: source.Text, Preference: canonical.Kind,
		}}}, nil
	}
	return EpisodicShadowProposalResponse{}, nil
}

func (f *fakeEpisodicProposer) snapshot() []EpisodicShadowProposalRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]EpisodicShadowProposalRequest(nil), f.calls...)
}

func (f *pruneJobFixture) messageIDs(from, to int) []uuid.UUID {
	f.t.Helper()
	var ids []uuid.UUID
	if err := f.db.Raw(`SELECT id FROM messages WHERE conversation_id = ?
AND seq BETWEEN ? AND ? ORDER BY seq`, f.conversation, from, to).Scan(&ids).Error; err != nil {
		f.t.Fatal(err)
	}
	return ids
}

func (f *pruneJobFixture) noteCount() int64 {
	var n int64
	f.db.Raw("SELECT count(*) FROM episodic_notes WHERE user_id = ?", f.owner).Scan(&n)
	return n
}

func (f *pruneJobFixture) distilled() int {
	var d int
	f.db.Raw("SELECT distilled_through_seq FROM conversations WHERE id = ?", f.conversation).Scan(&d)
	return d
}

func (f *pruneJobFixture) ledger(from, to int) (status, reason string, attempts int) {
	f.t.Helper()
	if err := f.db.Raw(`SELECT status, reason, attempts FROM episodic_distillation_segments
WHERE conversation_id = ? AND from_seq = ? AND to_seq = ?`, f.conversation, from, to).
		Row().Scan(&status, &reason, &attempts); err != nil {
		f.t.Fatal(err)
	}
	return status, reason, attempts
}

func (f *pruneJobFixture) attachDistillation(proposer EpisodicShadowProposer, enabled bool) *EpisodicDistillationTrigger {
	f.t.Helper()
	trigger := NewEpisodicDistillationTrigger(
		repository.NewEpisodicDistillationRepository(f.db),
		repository.NewEpisodicShadowRepository(f.db).WithShadowAdmissionEnabled(true),
		proposer,
		testProcessConfig().Episodic,
	).WithEnabled(enabled)
	f.svc.WithEpisodicDistillation(trigger)
	return trigger
}

func (f *pruneJobFixture) setText(id uuid.UUID, text string) {
	f.t.Helper()
	f.exec("UPDATE messages SET content_text = ? WHERE id = ?", text, id)
}

// A complete, settled, text-only segment at the prune boundary creates one
// shadow note. The new segment is the only provenance; the preceding turn is
// passed to the model as bounded, non-quotable context.
func TestEpisodicDistillationPruneBoundaryCreatesShadowNotePostgres(t *testing.T) {
	f := newPruneJobFixture(t)
	ctx := context.Background()
	f.addTurn("completed")
	f.addTurn("completed")
	f.addTurn("completed")
	ids := f.messageIDs(1, 6)
	f.setText(ids[2], "请简短回答。")
	f.advance(4, 5)

	proposer := &fakeEpisodicProposer{quote: true}
	f.attachDistillation(proposer, true)
	receipt, err := f.svc.Prune(ctx, f.owner)
	if err != nil || receipt == nil || receipt.PrunedThroughSeq != 4 || receipt.DeletedMessages != 4 {
		t.Fatalf("prune receipt=%+v err=%v", receipt, err)
	}
	if len(f.messageIDs(1, 4)) != 0 || len(f.messageIDs(5, 6)) != 2 {
		t.Fatalf("prune kept the wrong rows")
	}
	if f.noteCount() != 1 {
		t.Fatalf("notes=%d, want one", f.noteCount())
	}
	var note model.EpisodicNote
	if err := f.db.Where("user_id = ?", f.owner).Take(&note).Error; err != nil {
		t.Fatal(err)
	}
	if note.FromSeq != 3 || note.ToSeq != 4 || note.ContentText != "对话偏好：回答简短。" ||
		len(note.SourceMessageIDs) != 2 || note.SourceMessageIDs[0] != ids[2].String() ||
		note.SourceMessageIDs[1] != ids[3].String() {
		t.Fatalf("note provenance=%+v", note)
	}
	calls := proposer.snapshot()
	if len(calls) != 2 {
		t.Fatalf("proposer calls=%d, want two segments", len(calls))
	}
	if len(calls[0].Context) != 0 {
		t.Fatalf("first segment invented context: %+v", calls[0].Context)
	}
	if len(calls[1].Context) != 2 ||
		calls[1].Context[0].ID != ids[0] ||
		calls[1].Context[1].ID != ids[1] ||
		len(calls[1].Source) != 2 {
		t.Fatalf("bounded lookback=%+v", calls[1].Context)
	}
	if status, reason, attempts := f.ledger(1, 2); status != repository.EpisodicDistillationDistilled || reason != "abstained" || attempts != 0 {
		t.Fatalf("abstained ledger=%s/%s/%d", status, reason, attempts)
	}
	if status, reason, _ := f.ledger(3, 4); status != repository.EpisodicDistillationDistilled || reason != "persisted" {
		t.Fatalf("persisted ledger=%s/%s", status, reason)
	}
	if f.distilled() != 4 {
		t.Fatalf("watermark=%d, want 4", f.distilled())
	}
	var payload string
	if err := f.db.Raw(`SELECT payload::text FROM job_events WHERE job_id = ? AND event_type = 'episodic.distillation'`,
		receipt.JobID).Row().Scan(&payload); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"considered": 2`, `"distilled": 1`, `"abstained": 1`} {
		if !strings.Contains(payload, want) {
			t.Fatalf("job event=%s missing %s", payload, want)
		}
	}
}

// The runtime persistence shape (user body only in a text part with empty
// content_text, assistant body in content_text beside a quote part) must flow
// through generation, Go quote validation and the locked write.
func TestEpisodicDistillationRuntimeSourceShapeCreatesShadowNotePostgres(t *testing.T) {
	f := newPruneJobFixture(t)
	ctx := context.Background()
	f.addTurn("completed")
	f.addTurn("completed")
	f.addTurn("completed")
	ids := f.messageIDs(1, 6)
	f.exec(`UPDATE messages SET content_text='', parts='[{"text":"请简短回答。","type":"text"}]'::jsonb WHERE id=?`,
		ids[2])
	f.exec(`UPDATE messages SET parts=?::jsonb WHERE id=?`,
		`[{"type":"quote","message_id":"`+ids[2].String()+`"},{"type":"text","text":"请简短回答。"}]`,
		ids[3])
	f.advance(4, 5)

	proposer := &fakeEpisodicProposer{quote: true}
	f.attachDistillation(proposer, true)
	receipt, err := f.svc.Prune(ctx, f.owner)
	if err != nil || receipt == nil || receipt.PrunedThroughSeq != 4 {
		t.Fatalf("prune receipt=%+v err=%v", receipt, err)
	}
	if f.noteCount() != 1 {
		t.Fatalf("notes=%d, want one", f.noteCount())
	}
	var note model.EpisodicNote
	if err := f.db.Where("user_id = ?", f.owner).Take(&note).Error; err != nil {
		t.Fatal(err)
	}
	if note.FromSeq != 3 || note.ToSeq != 4 || note.ContentText != "对话偏好：回答简短。" ||
		len(note.SourceMessageIDs) != 2 || note.SourceMessageIDs[0] != ids[2].String() ||
		note.SourceMessageIDs[1] != ids[3].String() {
		t.Fatalf("note provenance=%+v", note)
	}
	calls := proposer.snapshot()
	if len(calls) != 2 || len(calls[1].Source) != 2 ||
		calls[1].Source[0].Text != "请简短回答。" ||
		calls[1].Source[1].Text != "synthetic" {
		t.Fatalf("canonical source handed to the proposer=%+v", calls)
	}
	if status, reason, _ := f.ledger(3, 4); status != repository.EpisodicDistillationDistilled || reason != "persisted" {
		t.Fatalf("ledger=%s/%s", status, reason)
	}
	if f.distilled() != 4 {
		t.Fatalf("watermark=%d, want 4", f.distilled())
	}
}

// Clinical, multimodal and unsupported turns are skipped without a model
// call; the prune still deletes every source row on schedule.
func TestEpisodicDistillationIneligibleSegmentsDoNotCallTheModelPostgres(t *testing.T) {
	f := newPruneJobFixture(t)
	ctx := context.Background()
	clinicalRun := f.addTurn("completed")
	f.addTurn("completed")
	f.addTurn("completed")
	ids := f.messageIDs(1, 6)
	f.exec(`INSERT INTO statement_evidence
(id,user_id,source_kind,source_version,excerpt,availability,recorded_at,source_conversation_id,source_run_id,source_message_id)
VALUES (?,?,'user_message',1,'synthetic','available',now(),?,?,?)`,
		uuid.New(), f.owner, f.conversation, clinicalRun, ids[0])
	f.exec(`UPDATE messages SET parts =
'[{"type":"image","upload_id":"00000000-0000-4000-8000-000000000001"}]'::jsonb WHERE id=?`, ids[2])
	f.advance(4, 5)

	proposer := &fakeEpisodicProposer{quote: true}
	f.attachDistillation(proposer, true)
	receipt, err := f.svc.Prune(ctx, f.owner)
	if err != nil || receipt == nil || receipt.PrunedThroughSeq != 4 {
		t.Fatalf("prune receipt=%+v err=%v", receipt, err)
	}
	if len(proposer.snapshot()) != 0 {
		t.Fatal("ineligible source reached the model")
	}
	if f.noteCount() != 0 {
		t.Fatalf("ineligible source created a note: %d", f.noteCount())
	}
	for _, want := range []struct {
		from, to int
		reason   string
	}{{1, 2, "clinical_source"}, {3, 4, "multimodal_source"}} {
		status, reason, _ := f.ledger(want.from, want.to)
		if status != repository.EpisodicDistillationSkipped || reason != want.reason {
			t.Fatalf("ledger %d..%d=%s/%s", want.from, want.to, status, reason)
		}
	}
	// The cited statement keeps its own source message (existing retention
	// policy), so three rows remain; the trigger still skipped both segments.
	if f.distilled() != 4 || f.messages() != 3 {
		t.Fatalf("watermark=%d messages=%d", f.distilled(), f.messages())
	}
}

// An unsettled Run still defers the pass before any generation, exactly as
// the pre-S2 retention path does.
func TestEpisodicDistillationDoesNotRunBesideAnUnsettledRunPostgres(t *testing.T) {
	f := newPruneJobFixture(t)
	ctx := context.Background()
	f.addTurn("completed")
	f.addTurn("completed")
	f.advance(2, 3)
	f.exec("UPDATE messages SET status = 'streaming' WHERE seq = 4")

	proposer := &fakeEpisodicProposer{quote: true}
	f.attachDistillation(proposer, true)
	if _, err := f.svc.Prune(ctx, f.owner); !errors.Is(err, model.ErrRetentionBusy) {
		t.Fatalf("unsettled pass: %v", err)
	}
	if len(proposer.snapshot()) != 0 || f.noteCount() != 0 || f.messages() != 4 {
		t.Fatal("generation ran beside an unsettled Run")
	}
}

// A model failure never blocks deletion. The segment is gapped at the prune
// boundary and the pass completes.
func TestEpisodicDistillationFailureDoesNotBlockPrunePostgres(t *testing.T) {
	f := newPruneJobFixture(t)
	ctx := context.Background()
	f.addTurn("completed")
	f.addTurn("completed")
	f.advance(2, 3)

	proposer := &fakeEpisodicProposer{err: errors.New("synthetic model failure")}
	f.attachDistillation(proposer, true)
	receipt, err := f.svc.Prune(ctx, f.owner)
	if err != nil || receipt == nil || receipt.PrunedThroughSeq != 2 || f.messages() != 2 {
		t.Fatalf("prune receipt=%+v messages=%d err=%v", receipt, f.messages(), err)
	}
	if f.noteCount() != 0 {
		t.Fatal("failed generation created a note")
	}
	status, reason, _ := f.ledger(1, 2)
	if status != repository.EpisodicDistillationGapped || reason != "pruned_before_distillation" {
		t.Fatalf("failure ledger=%s/%s", status, reason)
	}
	if f.distilled() != 2 {
		t.Fatalf("watermark=%d, want 2", f.distilled())
	}
}
