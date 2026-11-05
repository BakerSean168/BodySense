package repository

import (
	"context"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// #514 S3-B: explicit asset provenance is mandatory for sourced episodic
// notes, and asset erasure must delete every derived note before completion.
// All data below is synthetic; no note writer or model serving is enabled.
func TestEpisodicNoteAssetErasureAndGhostSourceGatePostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	ctx := context.Background()
	owner, foreign := createSingletonTestUser(t, db), createSingletonTestUser(t, db)
	conversation, _, err := NewConversationRepository(db).GetOrCreateForUser(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}

	asset := seedErasureAsset(t, db, owner, "report")
	unrelated := seedErasureAsset(t, db, owner, "report")
	foreignAsset := seedErasureAsset(t, db, foreign, "report")

	insertNote := func(assetID uuid.UUID, seq int, content string) error {
		return db.Exec(`INSERT INTO episodic_notes
(user_id, conversation_id, from_seq, to_seq, source_message_ids, source_asset_ids,
 generator_version, retention_policy_version, content_text)
VALUES (?, ?, ?, ?, ARRAY[?::uuid], ARRAY[?::uuid], 'test-asset-v1','policy-v1', ?)`,
			owner, conversation.ID, seq, seq+1, uuid.New(), assetID, content).Error
	}
	countNote := func(assetID uuid.UUID) int64 {
		t.Helper()
		return erasureRows(t, db, `SELECT count(*) FROM episodic_notes WHERE user_id = ?
AND source_asset_ids @> ARRAY[?::uuid]`, owner, assetID)
	}

	if err := insertNote(asset, 1, "source-derived synthetic note"); err != nil {
		t.Fatal(err)
	}
	if err := insertNote(unrelated, 3, "independent synthetic note"); err != nil {
		t.Fatal(err)
	}
	if err := insertNote(foreignAsset, 5, "foreign owner asset"); err == nil {
		t.Fatal("episodic note admitted another owner's asset")
	}
	if err := insertNote(uuid.New(), 7, "unknown asset"); err == nil {
		t.Fatal("episodic note admitted a missing asset")
	}

	// Asset rows serialize note admission against the erasure row lock.
	locker := db.Begin()
	if locker.Error != nil {
		t.Fatal(locker.Error)
	}
	if err := locker.Exec("SELECT id FROM user_assets WHERE id = ? AND user_id = ? FOR UPDATE",
		asset, owner).Error; err != nil {
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
(user_id, conversation_id, from_seq, to_seq, source_message_ids, source_asset_ids,
 generator_version, retention_policy_version, content_text)
VALUES (?, ?, 9, 10, ARRAY[?::uuid], ARRAY[?::uuid], 'test-lock-v1','policy-v1','should block')`,
		owner, conversation.ID, uuid.New(), asset).Error; err == nil {
		_ = writer.Rollback()
		_ = locker.Rollback()
		t.Fatal("asset-row lock did not block note insertion")
	}
	_ = writer.Rollback()
	_ = locker.Rollback()

	repo := NewAssetErasureRepository(db)
	receipt, err := repo.CreateOrGet(ctx, owner, asset, "erase-synthetic-note")
	if err != nil || receipt.Request.InventoryRevision != model.AssetErasureInventoryRevision ||
		len(receipt.Targets) != len(model.AssetErasureTargets) {
		t.Fatalf("admission failed: %+v err=%v", receipt, err)
	}
	req := &receipt.Request
	first := execErasureTarget(t, repo, req, model.AssetErasureEpisodicNotes)
	if first.State != model.AssetErasureTargetVerified || first.Erased != 1 ||
		countNote(asset) != 0 || countNote(unrelated) != 1 {
		t.Fatalf("note erasure changed unrelated or kept source: %+v", first)
	}

	// Simulate successful source-object deletion and independent runtime
	// verification without touching any real storage or model service.
	for _, target := range model.AssetErasureTargets {
		switch target {
		case model.AssetErasureManifest, model.AssetErasureEpisodicNotes:
		case model.AssetErasureSourceObjects, model.AssetErasureRuntimeCheckpoints:
			if err := repo.RecordOutcome(ctx, req, target,
				model.AssetErasureOutcome{State: model.AssetErasureTargetVerified}); err != nil {
				t.Fatal(err)
			}
		default:
			got := execErasureTarget(t, repo, req, target)
			if got.State != model.AssetErasureTargetVerified {
				t.Fatalf("target %s not verified: %+v", target, got)
			}
		}
	}
	if err := db.Exec(`UPDATE user_assets SET availability='source_removed',
source_removed_at=now() WHERE user_id = ? AND id = ?`, owner, asset).Error; err != nil {
		t.Fatal(err)
	}

	// A note can reappear between the first target pass and the final
	// manifest; Finalize must reopen the target, not report full erasure.
	if err := insertNote(asset, 11, "reappeared before finalize"); err != nil {
		t.Fatal(err)
	}
	done, err := repo.Finalize(ctx, req)
	if err != nil || done {
		t.Fatalf("finalization ignored reappeared note: %v %v", done, err)
	}
	latest, err := repo.GetByAsset(ctx, owner, asset)
	if err != nil {
		t.Fatal(err)
	}
	var outstanding *model.AssetErasureTarget
	for i := range latest.Targets {
		if latest.Targets[i].Target == model.AssetErasureEpisodicNotes {
			outstanding = &latest.Targets[i]
			break
		}
	}
	if outstanding == nil || outstanding.State != model.AssetErasureTargetFailed ||
		outstanding.FailureCode == nil || *outstanding.FailureCode != model.AssetErasureCodeCopyReappeared {
		t.Fatalf("reappeared note not flagged: %+v", outstanding)
	}
	retry := execErasureTarget(t, repo, req, model.AssetErasureEpisodicNotes)
	if retry.State != model.AssetErasureTargetVerified || retry.Erased != 1 {
		t.Fatalf("retry did not delete note: %+v", retry)
	}
	done, err = repo.Finalize(ctx, req)
	if err != nil || !done {
		t.Fatalf("finalize after source purge: %v %v", done, err)
	}
	if countNote(asset) != 0 || countNote(unrelated) != 1 {
		t.Fatal("post-erasure note preservation mismatch")
	}
	if err := insertNote(asset, 13, "ghost copy"); err == nil {
		t.Fatal("new note references an erased asset")
	}
	if err := db.Exec(`UPDATE episodic_notes SET source_asset_ids=ARRAY[?::uuid]
WHERE user_id=? AND source_asset_ids @> ARRAY[?::uuid]`, asset, owner, unrelated).Error; err == nil {
		t.Fatal("existing note rewrote source to an erased asset")
	}
	if countNote(unrelated) != 1 {
		t.Fatal("failed asset rewrite destroyed unrelated note")
	}
}
