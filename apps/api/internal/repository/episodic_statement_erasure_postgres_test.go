package repository

import (
	"context"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// S3-A: note deletion is a mandatory receipt target. A retry, an intervening
// write and the old source's post-erasure resurrection are all fail-closed.
func TestEpisodicNoteStatementErasureAndGhostSourceGatePostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	f := newRetentionFixture(t, db)
	ctx := context.Background()

	original := f.addTurn("completed", "synthetic source to erase")
	unrelated := f.addTurn("completed", "synthetic unrelated source")
	_ = f.addTurn("completed", "synthetic tail")
	statementID := f.addEvidence(f.owner, original)
	_ = f.addEvidence(f.owner, unrelated)

	insertNote := func(source uuid.UUID, seq int, content string) error {
		return db.Exec(`INSERT INTO episodic_notes
(user_id, conversation_id, from_seq, to_seq, source_message_ids,
 generator_version, retention_policy_version, content_text)
VALUES (?, ?, ?, ?, ARRAY[?::uuid], 'test-v1', 'ret-v1', ?)`,
			f.owner, f.conversation, seq, seq+1, source, content).Error
	}
	countNote := func(source uuid.UUID) int64 {
		f.t.Helper()
		return f.count(`SELECT count(*) FROM episodic_notes
WHERE user_id = ? AND source_message_ids @> ARRAY[?::uuid]`, f.owner, source)
	}

	if err := insertNote(original.user, 1, "synthetic copied source"); err != nil {
		t.Fatal(err)
	}
	if err := insertNote(unrelated.user, 3, "synthetic independent note"); err != nil {
		t.Fatal(err)
	}
	repo := NewStatementErasureRepository(db)
	receipt, err := repo.CreateOrGet(ctx, f.owner, statementID, "synthetic-erase")
	if err != nil || receipt.Request.InventoryRevision != model.StatementErasureInventoryRevision ||
		len(receipt.Targets) != len(model.StatementErasureTargets) {
		t.Fatalf("admission failed receipt=%+v err=%v", receipt, err)
	}
	req := &receipt.Request

	// Trigger must acquire the statement row's FOR SHARE lock. A concurrent
	// note write must not pass a held erasure row lock.
	locker := db.Begin()
	if locker.Error != nil {
		t.Fatal(locker.Error)
	}
	if err := locker.Exec(`SELECT id FROM statement_evidence
WHERE user_id = ? AND id = ? FOR UPDATE`, f.owner, statementID).Error; err != nil {
		_ = locker.Rollback()
		t.Fatal(err)
	}
	writer := db.Begin()
	if writer.Error != nil {
		_ = locker.Rollback()
		t.Fatal(writer.Error)
	}
	if err := writer.Exec("SET LOCAL lock_timeout = '250ms'").Error; err != nil {
		_ = writer.Rollback()
		_ = locker.Rollback()
		t.Fatal(err)
	}
	if err := writer.Exec(`INSERT INTO episodic_notes
(user_id, conversation_id, from_seq, to_seq, source_message_ids,
 generator_version, retention_policy_version, content_text)
VALUES (?, ?, 5, 6, ARRAY[?::uuid], 'lock-test-v1','ret-v1','blocked concurrent note')`,
		f.owner, f.conversation, original.user).Error; err == nil {
		_ = writer.Rollback()
		_ = locker.Rollback()
		t.Fatal("source insert bypassed statement row lock")
	}
	_ = writer.Rollback()
	_ = locker.Rollback()

	outcome := execStatementTarget(t, repo, req, model.StatementErasureEpisodicNotes)
	if outcome.State != model.AssetErasureTargetVerified || outcome.Erased != 1 ||
		countNote(original.user) != 0 || countNote(unrelated.user) != 1 {
		t.Fatalf("note erasure or owner scoping failed: %+v", outcome)
	}

	for _, target := range model.StatementErasureTargets {
		switch target {
		case model.StatementErasureManifest, model.StatementErasureEpisodicNotes:
		case model.StatementErasureRuntimeCheckpoints:
			if err := repo.RecordOutcome(ctx, req, target,
				model.AssetErasureOutcome{State: model.AssetErasureTargetVerified}); err != nil {
				t.Fatal(err)
			}
		default:
			got := execStatementTarget(t, repo, req, target)
			if got.State != model.AssetErasureTargetVerified {
				t.Fatalf("target %s did not verify: %+v", target, got)
			}
		}
	}

	// A shadow writer racing before finalize has to be detected, even if the
	// raw message has already been pruned by the source_message target.
	if err := insertNote(original.user, 7, "copy reappeared"); err != nil {
		t.Fatal(err)
	}
	terminal, err := repo.Finalize(ctx, req)
	if err != nil || terminal {
		t.Fatalf("unsafe finalize: terminal=%v err=%v", terminal, err)
	}
	again, err := repo.GetByStatement(ctx, f.owner, statementID)
	if err != nil {
		t.Fatal(err)
	}
	outstanding := statementReceiptTarget(t, again, model.StatementErasureEpisodicNotes)
	if outstanding.State != model.AssetErasureTargetFailed ||
		outstanding.FailureCode == nil || *outstanding.FailureCode != model.AssetErasureCodeCopyReappeared {
		t.Fatalf("missed revived note target: %+v", outstanding)
	}

	second := execStatementTarget(t, repo, req, model.StatementErasureEpisodicNotes)
	if second.State != model.AssetErasureTargetVerified || second.Erased != 1 {
		t.Fatalf("retry: %+v", second)
	}
	terminal, err = repo.Finalize(ctx, req)
	if err != nil || !terminal {
		t.Fatalf("finalize after retry: %v %v", terminal, err)
	}
	if countNote(unrelated.user) != 1 || countNote(original.user) != 0 {
		t.Fatal("source erasure deleted unrelated note or leaked source copy")
	}
	// The old source statement persists as a tombstone, and the DB source
	// gate rejects both fresh inserts and rewrites of an unrelated note.
	if err := insertNote(original.user, 9, "illegal resurrection"); err == nil {
		t.Fatal("erased source accepted into a new note")
	}
	if err := db.Exec(`UPDATE episodic_notes
SET source_message_ids = ARRAY[?::uuid]
WHERE user_id = ? AND source_message_ids @> ARRAY[?::uuid]`,
		original.user, f.owner, unrelated.user).Error; err == nil {
		t.Fatal("existing note rewrote provenance to erased source")
	}
	if countNote(unrelated.user) != 1 {
		t.Fatal("failed provenance rewrite changed an unrelated note")
	}
}
