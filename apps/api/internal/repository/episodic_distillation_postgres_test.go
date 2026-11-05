package repository

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Real PostgreSQL 18, synthetic chat only. Requires
// BODYSENSE_INTEGRATION_DATABASE_URL. The distillation repository never calls
// a model; these tests assert the content-free ledger, the segment boundary
// and the watermark that the S2 trigger relies on.
func TestEpisodicDistillationSegmentsWatermarkAndGapPostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	f := newRetentionFixture(t, db)
	ctx := context.Background()
	first := f.addTurn("completed", "synthetic first")
	second := f.addTurn("completed", "synthetic second")
	third := f.addTurn("completed", "synthetic third")
	repo := NewEpisodicDistillationRepository(db)

	state, err := repo.State(ctx, f.owner)
	if err != nil || state.DistilledThroughSeq != 0 || state.PrunedThroughSeq != 0 {
		t.Fatalf("initial state: %+v %v", state, err)
	}
	segments, err := repo.Segments(ctx, f.owner, f.conversation, 0, 6)
	if err != nil || len(segments) != 3 {
		t.Fatalf("segments=%d %v", len(segments), err)
	}
	for i, want := range []retentionTurn{first, second, third} {
		seg := segments[i]
		if seg.FromSeq != want.userSeq || seg.ToSeq != want.replySeq || !seg.Eligible ||
			len(seg.Messages) != 2 || seg.Messages[0].ID != want.user || seg.Messages[1].ID != want.reply {
			t.Fatalf("segment %d=%+v", i, seg)
		}
	}
	if len(segments[0].Context) != 0 || len(segments[1].Context) != 2 || len(segments[2].Context) != 2 {
		t.Fatalf("bounded lookback: %d/%d/%d", len(segments[0].Context), len(segments[1].Context), len(segments[2].Context))
	}
	if segments[1].Context[0].ID != first.user || segments[1].Context[1].ID != first.reply {
		t.Fatalf("lookback is not the preceding complete turn: %+v", segments[1].Context)
	}

	f.advance(6, 7)
	if err := repo.RecordPlan(ctx, f.owner, f.conversation, segments); err != nil {
		t.Fatal(err)
	}
	var pending int64
	db.Raw("SELECT count(*) FROM episodic_distillation_segments WHERE conversation_id=? AND status='pending'", f.conversation).Scan(&pending)
	if pending != 3 {
		t.Fatalf("plan did not persist every segment before generation: %d", pending)
	}
	if err := repo.RecordResolved(ctx, f.owner, f.conversation, first.userSeq, first.replySeq,
		EpisodicDistillationDistilled, "abstained", nil); err != nil {
		t.Fatal(err)
	}
	if err := repo.AdvanceWaterline(ctx, f.owner, f.conversation, first.replySeq); err != nil {
		t.Fatal(err)
	}
	// The second segment fails once and stays pending; the third is skipped
	// as ineligible. D must not jump over the unresolved segment.
	status, attempts, err := repo.RecordFailure(ctx, f.owner, f.conversation,
		second.userSeq, second.replySeq, "timeout", EpisodicDistillationMaxAttempts)
	if err != nil || status != EpisodicDistillationPending || attempts != 1 {
		t.Fatalf("failure: %s %d %v", status, attempts, err)
	}
	if err := repo.RecordResolved(ctx, f.owner, f.conversation, third.userSeq, third.replySeq,
		EpisodicDistillationSkipped, "clinical_source", nil); err != nil {
		t.Fatal(err)
	}
	// The service only advances over a resolved prefix; the second segment is
	// still pending, so D stays at the first segment.
	if err := repo.AdvanceWaterline(ctx, f.owner, f.conversation, first.replySeq); err != nil {
		t.Fatal(err)
	}
	state, _ = repo.State(ctx, f.owner)
	if state.DistilledThroughSeq != first.replySeq {
		t.Fatalf("watermark skipped a pending segment: %d", state.DistilledThroughSeq)
	}

	// Prune deletes every source; the pending segment becomes a recorded gap
	// and D catches up with R. No source row is kept alive.
	if _, err := f.prune(ctx, f.owner); err != nil {
		t.Fatal(err)
	}
	gaps, err := repo.HealAfterPrune(ctx, f.owner, f.conversation)
	if err != nil || gaps != 1 {
		t.Fatalf("heal gaps=%d %v", gaps, err)
	}
	state, _ = repo.State(ctx, f.owner)
	if state.DistilledThroughSeq != state.PrunedThroughSeq || f.messageExists(second.reply) {
		t.Fatalf("heal did not close the pruned window: %+v", state)
	}
	var rowStatus, reason string
	if err := db.Raw(`SELECT status, reason FROM episodic_distillation_segments
WHERE conversation_id=? AND from_seq=? AND to_seq=?`, f.conversation, second.userSeq, second.replySeq).
		Row().Scan(&rowStatus, &reason); err != nil {
		t.Fatal(err)
	}
	if rowStatus != EpisodicDistillationGapped || reason == "" {
		t.Fatalf("pruned pending segment was not recorded: %s/%s", rowStatus, reason)
	}
}

func TestEpisodicDistillationRetryCeilingAndHealPostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	f := newRetentionFixture(t, db)
	ctx := context.Background()
	turn := f.addTurn("completed", "synthetic retry")
	f.addTurn("completed", "synthetic newest")
	repo := NewEpisodicDistillationRepository(db)
	if err := repo.RecordPlan(ctx, f.owner, f.conversation, []EpisodicDistillationSegment{{
		FromSeq: turn.userSeq, ToSeq: turn.replySeq, Eligible: true,
	}}); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= EpisodicDistillationMaxAttempts; i++ {
		status, attempts, err := repo.RecordFailure(ctx, f.owner, f.conversation,
			turn.userSeq, turn.replySeq, "model_error", EpisodicDistillationMaxAttempts)
		if err != nil || attempts != i {
			t.Fatalf("attempt %d: %s %d %v", i, status, attempts, err)
		}
		want := EpisodicDistillationPending
		if i == EpisodicDistillationMaxAttempts {
			want = EpisodicDistillationGapped
		}
		if status != want {
			t.Fatalf("attempt %d status=%s want=%s", i, status, want)
		}
	}
	// A later pass sees the segment terminal and must not reopen it.
	status, attempts, err := repo.RecordFailure(ctx, f.owner, f.conversation,
		turn.userSeq, turn.replySeq, "late_retry", EpisodicDistillationMaxAttempts)
	if err != nil || status != EpisodicDistillationGapped || attempts != EpisodicDistillationMaxAttempts {
		t.Fatalf("terminal segment reopened: %s %d %v", status, attempts, err)
	}

	// A trigger that already enumerated the window does not add a duplicate
	// coarse gap; the existing segment row accounts for the pruned range.
	f.advance(4, 5)
	if _, err := f.prune(ctx, f.owner); err != nil {
		t.Fatal(err)
	}
	gaps, err := repo.HealAfterPrune(ctx, f.owner, f.conversation)
	if err != nil || gaps != 0 {
		t.Fatalf("unenumerated gap=%d %v", gaps, err)
	}
	var n int64
	db.Raw(`SELECT count(*) FROM episodic_distillation_segments
WHERE conversation_id=? AND status='gapped' AND reason='pruned_unenumerated'`, f.conversation).Scan(&n)
	if n != 0 {
		// The earlier segment row already accounts for this window; the
		// heuristic only adds a coarse row when nothing was enumerated.
		t.Fatalf("unexpected coarse gap beside an enumerated segment: %d", n)
	}
	state, _ := repo.State(ctx, f.owner)
	if state.DistilledThroughSeq != state.PrunedThroughSeq {
		t.Fatalf("watermark did not catch up: %+v", state)
	}
}

func TestEpisodicDistillationUnenumeratedWindowRecordsCoarseGapPostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	f := newRetentionFixture(t, db)
	ctx := context.Background()
	f.addTurn("completed", "synthetic unenumerated")
	f.addTurn("completed", "synthetic newest")
	repo := NewEpisodicDistillationRepository(db)
	f.advance(4, 5)
	if _, err := f.prune(ctx, f.owner); err != nil {
		t.Fatal(err)
	}
	gaps, err := repo.HealAfterPrune(ctx, f.owner, f.conversation)
	if err != nil || gaps != 1 {
		t.Fatalf("coarse gap=%d %v", gaps, err)
	}
	var from, to int
	var reason string
	if err := db.Raw(`SELECT from_seq, to_seq, reason FROM episodic_distillation_segments
WHERE conversation_id=? AND status='gapped'`, f.conversation).Row().Scan(&from, &to, &reason); err != nil {
		t.Fatal(err)
	}
	if from != 1 || to != 2 || reason != "pruned_unenumerated" {
		t.Fatalf("coarse gap=%d..%d %s", from, to, reason)
	}
}

func TestEpisodicDistillationSegmentsRejectClinicalMultimodalAndPartialPostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	f := newRetentionFixture(t, db)
	ctx := context.Background()
	clinical := f.addTurn("completed", "synthetic clinical source")
	f.addEvidence(f.owner, clinical)
	multimodal := f.addTurn("completed", "synthetic multimodal source")
	if err := db.Exec(`UPDATE messages SET parts =
'[{"type":"image","upload_id":"00000000-0000-4000-8000-000000000001"}]'::jsonb WHERE id=?`, multimodal.user).Error; err != nil {
		t.Fatal(err)
	}
	partial := f.addTurn("completed", "synthetic partial source")
	if err := db.Exec("UPDATE messages SET status='streaming' WHERE id=?", partial.reply).Error; err != nil {
		t.Fatal(err)
	}
	normal := f.addTurn("completed", "synthetic normal source")
	repo := NewEpisodicDistillationRepository(db)
	segments, err := repo.Segments(ctx, f.owner, f.conversation, 0, 8)
	if err != nil || len(segments) != 4 {
		t.Fatalf("segments=%d %v", len(segments), err)
	}
	for i, want := range []string{"clinical_source", "multimodal_source", "unsettled_source", ""} {
		if segments[i].Reason != want {
			t.Fatalf("segment %d reason=%q want=%q", i, segments[i].Reason, want)
		}
		if want != "" && segments[i].Eligible {
			t.Fatalf("ineligible segment %d admitted: %+v", i, segments[i])
		}
	}
	if !segments[3].Eligible || segments[3].FromSeq != normal.userSeq {
		t.Fatalf("eligible segment after rejects: %+v", segments[3])
	}
	// A clinical previous turn is never used as model context.
	if len(segments[1].Context) != 0 || len(segments[2].Context) != 0 {
		t.Fatalf("clinical/unsettled context leaked: %d/%d", len(segments[1].Context), len(segments[2].Context))
	}
}

func TestEpisodicDistillationSegmentTombstoneSuppressesContextPostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	f := newRetentionFixture(t, db)
	ctx := context.Background()
	first := f.addTurn("completed", "synthetic first")
	f.addTurn("completed", "synthetic second")
	f.advance(4, 5)
	writer := NewEpisodicShadowRepository(db).WithShadowAdmissionEnabled(true)
	note, _, err := writer.SaveShadowDraft(ctx, f.owner, shadowDraft(f, first))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewEpisodicNoteControlsRepository(db).DeleteOwned(ctx, f.owner, note.ID); err != nil {
		t.Fatal(err)
	}
	repo := NewEpisodicDistillationRepository(db)
	segments, err := repo.Segments(ctx, f.owner, f.conversation, 0, 4)
	if err != nil || len(segments) != 2 {
		t.Fatalf("segments=%d %v", len(segments), err)
	}
	if segments[0].Eligible || segments[0].Reason != "user_deleted" {
		t.Fatalf("tombstoned segment not suppressed: %+v", segments[0])
	}
	if !segments[1].Eligible || len(segments[1].Context) != 0 {
		t.Fatalf("tombstoned context leaked into the next segment: %+v", segments[1])
	}
}

func TestEpisodicDistillationMigrationRefusesRollbackWithGapPostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	f := newRetentionFixture(t, db)
	turn := f.addTurn("completed", "synthetic rollback gate")
	repo := NewEpisodicDistillationRepository(db)
	if err := repo.RecordPlan(context.Background(), f.owner, f.conversation, []EpisodicDistillationSegment{{
		FromSeq: turn.userSeq, ToSeq: turn.replySeq, Eligible: true,
	}}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repo.RecordFailure(context.Background(), f.owner, f.conversation,
		turn.userSeq, turn.replySeq, "timeout", EpisodicDistillationMaxAttempts); err != nil {
		t.Fatal(err)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve migration location")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "migrations", "000007_episodic_distillation.down.sql")
	migration, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(string(migration)).Error; err == nil ||
		!strings.Contains(err.Error(), "cannot drop unresolvable episodic distillation gaps") {
		t.Fatalf("unsafe rollback unexpectedly succeeded: %v", err)
	}
	var columns int64
	db.Raw(`SELECT count(*) FROM information_schema.columns
WHERE table_schema='public' AND table_name='conversations' AND column_name='distilled_through_seq'`).Scan(&columns)
	if columns != 1 {
		t.Fatal("rejected downgrade removed the watermark")
	}
}
