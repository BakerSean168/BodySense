// Episodic distillation PostgreSQL scenarios: bounded retries, idempotent
// replay, tombstone suppression and the generation flag-off path. Requires
// BODYSENSE_INTEGRATION_DATABASE_URL; fixtures are synthetic and provider
// calls are faked (shared fixture in episodic_distillation_postgres_test.go).

package service

import (
	"context"
	"errors"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"testing"
)

// Retries are bounded across prune attempts while the source is still present.
// The third failure records a gap and releases the segment; a later successful
// cleanup then deletes the raw rows.
func TestEpisodicDistillationRetriesAreBoundedPostgres(t *testing.T) {
	f := newPruneJobFixture(t)
	ctx := context.Background()
	f.addTurn("completed")
	f.addTurn("completed")
	f.advance(2, 3)

	proposer := &fakeEpisodicProposer{err: errors.New("synthetic model failure")}
	f.attachDistillation(proposer, true)
	job, err := f.svc.EnqueuePrune(ctx, f.owner, "retry")
	if err != nil {
		t.Fatal(err)
	}
	f.ai.setFail(true)
	for attempt := 1; attempt <= 3; attempt++ {
		got := f.runJob(job.ID)
		if got.Status != model.JobStatusPending {
			t.Fatalf("attempt %d job=%s", attempt, got.Status)
		}
		if f.pruned() != 0 || f.messages() != 4 {
			t.Fatal("failed cleanup moved source rows")
		}
		status, reason, attempts := f.ledger(1, 2)
		want := repository.EpisodicDistillationPending
		if attempt == 3 {
			want = repository.EpisodicDistillationGapped
		}
		if status != want || attempts != attempt || reason != "model_error" {
			t.Fatalf("attempt %d ledger=%s/%s/%d", attempt, status, reason, attempts)
		}
	}
	if f.distilled() != 2 {
		t.Fatalf("retry limit did not release the segment: D=%d", f.distilled())
	}
	f.ai.setFail(false)
	got := f.runJob(job.ID)
	if got.Status != model.JobStatusCompleted || f.pruned() != 2 || f.messages() != 2 {
		t.Fatalf("job=%s pruned=%d messages=%d", got.Status, f.pruned(), f.messages())
	}
	if f.noteCount() != 0 {
		t.Fatal("gapped segment produced a note")
	}
}

// A crash between the note write and the watermark update must not create a
// duplicate note: the durable segment ledger wins.
func TestEpisodicDistillationReplayIsIdempotentPostgres(t *testing.T) {
	f := newPruneJobFixture(t)
	ctx := context.Background()
	f.addTurn("completed")
	f.addTurn("completed")
	ids := f.messageIDs(1, 4)
	f.setText(ids[0], "请简短回答。")
	f.advance(2, 3)

	proposer := &fakeEpisodicProposer{quote: true}
	trigger := f.attachDistillation(proposer, true)
	plan, replay, err := f.repo.PlanPrune(ctx, f.owner)
	if err != nil || replay != nil || plan == nil {
		t.Fatalf("plan=%v replay=%v %v", plan, replay, err)
	}
	stats := trigger.BeforePrune(ctx, f.owner, plan)
	if stats.Distilled != 1 || f.noteCount() != 1 {
		t.Fatalf("first pass stats=%+v notes=%d", stats, f.noteCount())
	}
	if err := f.db.Exec("UPDATE conversations SET distilled_through_seq = 0 WHERE id = ?", f.conversation).Error; err != nil {
		t.Fatal(err)
	}
	calls := len(proposer.snapshot())
	again := trigger.BeforePrune(ctx, f.owner, plan)
	if again.Considered != 1 || again.Distilled != 0 {
		t.Fatalf("replay stats=%+v", again)
	}
	if len(proposer.snapshot()) != calls || f.noteCount() != 1 {
		t.Fatalf("replay called the model or duplicated the note: calls=%d notes=%d",
			len(proposer.snapshot()), f.noteCount())
	}
	var count int64
	f.db.Raw("SELECT count(*) FROM episodic_distillation_segments WHERE conversation_id = ?", f.conversation).Scan(&count)
	if count != 1 {
		t.Fatalf("ledger rows=%d, want one", count)
	}
}

// A user-deleted note leaves a durable tombstone. The trigger must not send
// that range to the model and must not recreate the note.
func TestEpisodicDistillationTombstoneBlocksRegenerationAndEgressPostgres(t *testing.T) {
	f := newPruneJobFixture(t)
	ctx := context.Background()
	f.addTurn("completed")
	f.addTurn("completed")
	ids := f.messageIDs(1, 4)
	f.setText(ids[0], "请简短回答。")
	f.advance(2, 3)

	writer := repository.NewEpisodicShadowRepository(f.db).WithShadowAdmissionEnabled(true)
	note, _, err := writer.SaveShadowDraft(ctx, f.owner, repository.EpisodicShadowDraft{
		ConversationID: f.conversation, FromSeq: 1, ToSeq: 2,
		SourceMessageIDs: []uuid.UUID{ids[0], ids[1]},
		GeneratorVersion: "synthetic-v1", RetentionPolicyVersion: "shadow-policy-v1",
		ContentText:     "对话偏好：回答简短。",
		QuotedMessageID: ids[0], QuotedMessageText: "请简短回答。",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.NewEpisodicNoteControlsRepository(f.db).DeleteOwned(ctx, f.owner, note.ID); err != nil {
		t.Fatal(err)
	}
	proposer := &fakeEpisodicProposer{quote: true}
	f.attachDistillation(proposer, true)
	if _, err := f.svc.Prune(ctx, f.owner); err != nil {
		t.Fatal(err)
	}
	if len(proposer.snapshot()) != 0 || f.noteCount() != 0 {
		t.Fatalf("tombstoned range reached the model or regenerated: calls=%d notes=%d",
			len(proposer.snapshot()), f.noteCount())
	}
	status, reason, _ := f.ledger(1, 2)
	if status != repository.EpisodicDistillationSkipped || reason != "user_deleted" {
		t.Fatalf("tombstone ledger=%s/%s", status, reason)
	}
	var tombstones int64
	f.db.Raw("SELECT count(*) FROM episodic_note_deletions WHERE user_id = ?", f.owner).Scan(&tombstones)
	if tombstones != 1 {
		t.Fatal("tombstone was not preserved")
	}
}

// Flag off is byte-for-byte the pre-S2 prune path: no ledger, no watermark
// movement and no model call.
func TestEpisodicDistillationFlagOffPreservesPrunePostgres(t *testing.T) {
	control := newPruneJobFixture(t)
	disabled := newPruneJobFixture(t)
	ctx := context.Background()
	for _, f := range []*pruneJobFixture{control, disabled} {
		f.addTurn("completed")
		f.addTurn("completed")
		f.addTurn("completed")
		f.advance(4, 5)
	}
	proposer := &fakeEpisodicProposer{quote: true}
	disabled.attachDistillation(proposer, false)

	controlReceipt, err := control.svc.Prune(ctx, control.owner)
	if err != nil {
		t.Fatal(err)
	}
	disabledReceipt, err := disabled.svc.Prune(ctx, disabled.owner)
	if err != nil {
		t.Fatal(err)
	}
	if controlReceipt.PrunedThroughSeq != disabledReceipt.PrunedThroughSeq ||
		controlReceipt.DeletedMessages != disabledReceipt.DeletedMessages ||
		controlReceipt.PrunedRuns != disabledReceipt.PrunedRuns ||
		controlReceipt.RetainedMessages != disabledReceipt.RetainedMessages ||
		controlReceipt.CheckpointCleanup != disabledReceipt.CheckpointCleanup ||
		control.messages() != disabled.messages() {
		t.Fatalf("flag-off changed prune: control=%+v disabled=%+v", controlReceipt, disabledReceipt)
	}
	if len(proposer.snapshot()) != 0 || disabled.noteCount() != 0 {
		t.Fatal("disabled trigger ran generation")
	}
	var rows int64
	disabled.db.Raw("SELECT count(*) FROM episodic_distillation_segments WHERE conversation_id = ?",
		disabled.conversation).Scan(&rows)
	if rows != 0 || disabled.distilled() != 0 {
		t.Fatalf("disabled trigger wrote state: rows=%d D=%d", rows, disabled.distilled())
	}
}

func TestEpisodicShadowGenerationFlagFromEnv(t *testing.T) {
	for value, want := range map[string]bool{
		"": false, "false": false, "0": false, "yes": false, "true": true, "1": true, " TRUE ": true,
	} {
		got := EpisodicShadowGenerationEnabledFromEnv(func(string) string { return value })
		if got != want {
			t.Fatalf("value %q -> %v, want %v", value, got, want)
		}
	}
	trigger := NewEpisodicDistillationTriggerFromEnv(
		func(string) string { return "true" },
		repository.NewEpisodicDistillationRepository(nil),
		repository.NewEpisodicShadowRepository(nil),
		nil,
	)
	if trigger.Enabled() {
		t.Fatal("env flag alone enabled generation without an approved proposer")
	}
}
