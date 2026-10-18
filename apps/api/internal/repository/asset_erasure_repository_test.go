package repository

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func seedErasureAsset(t *testing.T, db *gorm.DB, owner uuid.UUID, kind string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if err := db.Exec(`INSERT INTO user_assets(id,user_id,kind,original_name,mime_type,byte_size,content_sha256,storage_backend,storage_key)
 VALUES (?,?,?,'private-name.pdf','application/pdf',1,?,'local',?)`, id, owner, kind, strings.Repeat("e", 64), "as002/"+id.String()).Error; err != nil {
		t.Fatal(err)
	}
	return id
}

func seedErasureExtraction(t *testing.T, db *gorm.DB, owner, asset uuid.UUID, withReview bool) uuid.UUID {
	t.Helper()
	run, hash := uuid.New(), strings.Repeat("f", 64)
	if err := db.Exec(`INSERT INTO document_extraction_runs(id,user_id,asset_id,status,configuration_id,mechanism_revision,document_sha256,result_sha256,raw_text_sha256,result_payload)
 VALUES (?,?,?,'completed','synthetic','1',?,?,?,'{"raw_text":"synthetic private text"}')`, run, owner, asset, hash, hash, hash).Error; err != nil {
		t.Fatal(err)
	}
	if withReview {
		if err := db.Exec(`INSERT INTO document_indicator_reviews(id,user_id,asset_id,extraction_run_id,indicator_index,action,idempotency_key,reviewer_user_id,reviewed_payload)
 VALUES (?,?,?,?,0,'confirm',?,?,'{"value":"synthetic"}')`, uuid.New(), owner, asset, run, uuid.NewString(), owner).Error; err != nil {
			t.Fatal(err)
		}
	}
	return run
}

func seedErasureJob(t *testing.T, db *gorm.DB, owner, asset uuid.UUID, status string) uuid.UUID {
	t.Helper()
	job := uuid.New()
	if err := db.Exec(`INSERT INTO jobs(id,user_id,job_type,status,input,result) VALUES (?,?,'ocr',?,jsonb_build_object('asset_id',?::text),'{"raw_text":"synthetic"}')`,
		job, owner, status, asset.String()).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO job_events(job_id,event_type,payload) VALUES (?,'progress','{"stage":"ocr"}')`, job).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO ai_output_reviews(user_id,job_id,output_type,raw_output) VALUES (?,?,'ocr','{"raw":"synthetic"}')`, owner, job).Error; err != nil {
		t.Fatal(err)
	}
	return job
}

func execErasureTarget(t *testing.T, repo *AssetErasureRepository, request *model.AssetErasureRequest, target model.AssetErasureTargetName) model.AssetErasureOutcome {
	t.Helper()
	outcome, err := repo.ExecuteDatabaseTarget(context.Background(), request, target)
	if err != nil {
		t.Fatalf("%s: %v", target, err)
	}
	if err := repo.RecordOutcome(context.Background(), request, target, outcome); err != nil {
		t.Fatal(err)
	}
	return outcome
}

func wantErasureOutcome(t *testing.T, target model.AssetErasureTargetName, got model.AssetErasureOutcome, state model.AssetErasureTargetState, code string, outstanding int) {
	t.Helper()
	if got.State != state || got.FailureCode != code || got.Outstanding != outstanding {
		t.Fatalf("%s: got %+v want state=%s code=%q outstanding=%d", target, got, state, code, outstanding)
	}
}

func erasureRows(t *testing.T, db *gorm.DB, query string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := db.Raw(query, args...).Scan(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

// Copies a Diagnosis still cites are never deleted under it: the Diagnosis
// family and the cited extraction evidence stay outstanding (review needed),
// the Diagnosis rows are untouched, and the asset is never reported erased.
func TestUserAssetErasureDiagnosisPinnedPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	f := seedDiagnosisFixture(t, db)
	review := seedDiagnosisDocumentReview(t, db, f.owner, "confirm")
	var assetText string
	if err := db.Raw(`SELECT asset_id::text FROM document_indicator_reviews WHERE id=?`, review).Scan(&assetText).Error; err != nil {
		t.Fatal(err)
	}
	asset := uuid.MustParse(assetText)
	confirm := "confirm"
	link := model.ConditionEntryEvidence{UserID: f.owner, ConditionID: f.entry.ID, Role: model.DiagnosisSupporting,
		Rationale: "Synthetic relevance", AnalyzedByRunID: f.run.ID, EvidenceType: model.DiagnosisDocumentEvidence,
		EvidenceID: review, EvidenceVersion: 1, DocumentReviewAction: &confirm}
	if err := db.Create(&link).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewAssetErasureRepository(db)
	ctx := context.Background()
	receipt, err := repo.CreateOrGet(ctx, f.owner, asset, "pinned")
	if err != nil {
		t.Fatal(err)
	}
	request := &receipt.Request
	got := execErasureTarget(t, repo, request, model.AssetErasureDiagnosisRecords)
	if got.State != model.AssetErasureTargetBlocked || got.FailureCode != model.AssetErasureCodeDiagnosisReview || got.Outstanding < 1 {
		t.Fatalf("diagnosis: %+v", got)
	}
	got = execErasureTarget(t, repo, request, model.AssetErasureDocumentExtractions)
	wantErasureOutcome(t, model.AssetErasureDocumentExtractions, got, model.AssetErasureTargetBlocked, model.AssetErasureCodeDiagnosisPinned, 2)
	if n := erasureRows(t, db, `SELECT count(*) FROM document_indicator_reviews WHERE id=?`, review); n != 1 {
		t.Fatalf("cited review deleted: %d", n)
	}
	if n := erasureRows(t, db, `SELECT count(*) FROM condition_entry_evidence WHERE user_id=? AND document_review_id=?`, f.owner, review); n != 1 {
		t.Fatalf("diagnosis evidence changed: %d", n)
	}
	if err := db.Exec(`UPDATE user_assets SET availability='source_removed', source_removed_at=now() WHERE id=?`, asset).Error; err != nil {
		t.Fatal(err)
	}
	erased, err := repo.Finalize(ctx, request)
	if err != nil || erased {
		t.Fatalf("finalize with a cited copy: erased=%v err=%v", erased, err)
	}
	var availability string
	if err := db.Raw(`SELECT availability FROM user_assets WHERE id=?`, asset).Scan(&availability).Error; err != nil || availability != "source_removed" {
		t.Fatalf("availability=%q err=%v", availability, err)
	}
}

// Each target verifies on its own: live work, frozen assessment input and an
// unsupported runtime store leave the request outstanding while every other
// family is erased; a copy that reappears reopens its target; only an
// all-verified request tombstones the asset, and erased is terminal.
func TestUserAssetErasureTargetsPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	ctx := context.Background()
	owner, foreign := seedBodyStateContextUser(t, db), seedBodyStateContextUser(t, db)
	asset, unrelated := seedErasureAsset(t, db, owner, "report"), seedErasureAsset(t, db, owner, "report")
	foreignAsset := seedErasureAsset(t, db, foreign, "report")
	seedErasureExtraction(t, db, owner, asset, true)
	seedErasureExtraction(t, db, owner, unrelated, true)
	job := seedErasureJob(t, db, owner, asset, "running")
	unrelatedJob := seedErasureJob(t, db, owner, unrelated, "completed")
	seedUserMessageFactEnvelope(t, db, owner, datatypes.JSON(`[{"type":"image","upload_id":"`+asset.String()+`"}]`), "user")
	if err := db.Exec(`INSERT INTO assessment_reports(user_id,status,observations) VALUES (?,'completed',?::jsonb)`,
		owner, `[{"ref":"report:upload:`+asset.String()+`:indicator:0"}]`).Error; err != nil {
		t.Fatal(err)
	}

	repo := NewAssetErasureRepository(db)
	if _, err := repo.CreateOrGet(ctx, owner, foreignAsset, "foreign"); !errors.Is(err, model.ErrAssetErasureNotFound) {
		t.Fatalf("foreign asset: %v", err)
	}
	receipt, err := repo.CreateOrGet(ctx, owner, asset, "erase-1")
	if err != nil || receipt.Request.Status != model.AssetErasurePending || len(receipt.Targets) != len(model.AssetErasureTargets) {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	if again, err := repo.CreateOrGet(ctx, owner, asset, "erase-2"); err != nil || again.Request.ID != receipt.Request.ID {
		t.Fatalf("repeat command: %+v %v", again, err)
	}
	if _, err := repo.CreateOrGet(ctx, owner, unrelated, "erase-1"); !errors.Is(err, model.ErrAssetErasureIdempotencyUsed) {
		t.Fatalf("key reuse: %v", err)
	}
	request := &receipt.Request

	got := execErasureTarget(t, repo, request, model.AssetErasureDerivedJobs)
	wantErasureOutcome(t, model.AssetErasureDerivedJobs, got, model.AssetErasureTargetFailed, model.AssetErasureCodeDerivedWorkActive, 1)
	if err := db.Exec(`UPDATE jobs SET status='completed' WHERE id=?`, job).Error; err != nil {
		t.Fatal(err)
	}
	got = execErasureTarget(t, repo, request, model.AssetErasureDerivedJobs)
	if got.State != model.AssetErasureTargetVerified || got.Erased != 2 {
		t.Fatalf("jobs: %+v", got)
	}
	if n := erasureRows(t, db, `SELECT count(*) FROM job_events WHERE job_id=?`, job); n != 0 {
		t.Fatalf("job events kept: %d", n)
	}
	wantErasureOutcome(t, "posture", execErasureTarget(t, repo, request, model.AssetErasurePostureAnalyses), model.AssetErasureTargetVerified, "", 0)
	wantErasureOutcome(t, "diagnosis", execErasureTarget(t, repo, request, model.AssetErasureDiagnosisRecords), model.AssetErasureTargetVerified, "", 0)
	got = execErasureTarget(t, repo, request, model.AssetErasureDocumentExtractions)
	if got.State != model.AssetErasureTargetVerified || got.Erased != 2 {
		t.Fatalf("extractions: %+v", got)
	}
	wantErasureOutcome(t, "assessment", execErasureTarget(t, repo, request, model.AssetErasureAssessmentReports),
		model.AssetErasureTargetBlocked, model.AssetErasureCodeAssessmentFrozen, 1)
	wantErasureOutcome(t, "runtime", execErasureTarget(t, repo, request, model.AssetErasureRuntimeCheckpoints),
		model.AssetErasureTargetUnsupported, model.AssetErasureCodeRuntimeHookPending, 1)
	if err := repo.RecordOutcome(ctx, request, model.AssetErasureSourceObjects, model.AssetErasureOutcome{State: model.AssetErasureTargetVerified, Erased: 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`UPDATE user_assets SET availability='source_removed', source_removed_at=now() WHERE id=?`, asset).Error; err != nil {
		t.Fatal(err)
	}
	if erased, err := repo.Finalize(ctx, request); err != nil || erased {
		t.Fatalf("finalize with outstanding targets: erased=%v err=%v", erased, err)
	}
	receipt, err = repo.GetByAsset(ctx, owner, asset)
	if err != nil {
		t.Fatal(err)
	}
	outstanding := []string{}
	for _, target := range receipt.Outstanding() {
		outstanding = append(outstanding, string(target.Target))
	}
	sort.Strings(outstanding)
	if strings.Join(outstanding, ",") != "assessment_reports,asset_manifest,runtime_checkpoints" {
		t.Fatalf("outstanding=%v", outstanding)
	}
	if _, err := repo.GetByAsset(ctx, foreign, asset); !errors.Is(err, model.ErrAssetErasureNotFound) {
		t.Fatalf("foreign receipt read: %v", err)
	}

	// The later slices' handlers/decisions remove these copies; simulate that.
	if err := db.Exec(`DELETE FROM assessment_reports WHERE user_id=?`, owner).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`DELETE FROM messages WHERE parts @> jsonb_build_array(jsonb_build_object('upload_id',?::text))`, asset.String()).Error; err != nil {
		t.Fatal(err)
	}
	execErasureTarget(t, repo, request, model.AssetErasureAssessmentReports)
	execErasureTarget(t, repo, request, model.AssetErasureRuntimeCheckpoints)
	// A copy written after its target verified reopens that target.
	seedErasureExtraction(t, db, owner, asset, false)
	if erased, err := repo.Finalize(ctx, request); err != nil || erased {
		t.Fatalf("finalize with a reappeared copy: erased=%v err=%v", erased, err)
	}
	receipt, _ = repo.GetByAsset(ctx, owner, asset)
	for _, target := range receipt.Targets {
		if target.Target == model.AssetErasureDocumentExtractions && (target.State != model.AssetErasureTargetFailed || target.FailureCode == nil || *target.FailureCode != model.AssetErasureCodeCopyReappeared) {
			t.Fatalf("reappeared copy: %+v", target)
		}
	}
	execErasureTarget(t, repo, request, model.AssetErasureDocumentExtractions)
	if erased, err := repo.Finalize(ctx, request); err != nil || !erased {
		t.Fatalf("finalize: erased=%v err=%v", erased, err)
	}
	if erased, err := repo.Finalize(ctx, request); err != nil || !erased {
		t.Fatalf("repeat finalize: erased=%v err=%v", erased, err)
	}
	var stored model.UserAsset
	if err := db.Where("id=?", asset).Take(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Availability != model.UserAssetErased || stored.ErasedAt == nil || stored.OriginalName != "" {
		t.Fatalf("tombstone=%+v", stored)
	}
	receipt, _ = repo.GetByAsset(ctx, owner, asset)
	if receipt.Request.Status != model.AssetErasureErased || len(receipt.Outstanding()) != 0 {
		t.Fatalf("erased receipt=%+v", receipt)
	}
	if err := db.Exec(`UPDATE asset_erasure_requests SET status='outstanding', erased_at=NULL WHERE id=?`, request.ID).Error; err == nil {
		t.Fatal("erased request reopened")
	}
	// Unrelated sources of the same owner are untouched.
	if n := erasureRows(t, db, `SELECT count(*) FROM document_indicator_reviews WHERE asset_id=?`, unrelated); n != 1 {
		t.Fatalf("unrelated review: %d", n)
	}
	if n := erasureRows(t, db, `SELECT count(*) FROM jobs WHERE id=?`, unrelatedJob); n != 1 {
		t.Fatalf("unrelated job: %d", n)
	}
	if n := erasureRows(t, db, `SELECT count(*) FROM user_assets WHERE id=? AND availability='available' AND original_name<>''`, unrelated); n != 1 {
		t.Fatalf("unrelated asset: %d", n)
	}
}

// A request can never be marked erased in SQL while a target is unverified.
func TestUserAssetErasureCompletionGuardPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	asset := seedErasureAsset(t, db, owner, "photo")
	repo := NewAssetErasureRepository(db)
	receipt, err := repo.CreateOrGet(context.Background(), owner, asset, "guard")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`UPDATE asset_erasure_requests SET status='erased', erased_at=now() WHERE id=?`, receipt.Request.ID).Error; err == nil {
		t.Fatal("erased with unverified targets")
	}
	if err := db.Exec(`UPDATE asset_erasure_requests SET asset_id=? WHERE id=?`, uuid.New(), receipt.Request.ID).Error; err == nil {
		t.Fatal("request identity rewritten")
	}
}

// The controlled-copy inventory must name every table that can hold an
// asset reference by column or foreign key; a new one fails this test until
// it is inventoried (or explicitly documented as a pointer-only reference).
func TestUserAssetErasureInventoryCoversSchemaPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	var tables []string
	if err := db.Raw(`SELECT DISTINCT table_name::text FROM information_schema.columns
 WHERE table_schema='public' AND column_name IN ('asset_id','upload_id')
 UNION
 SELECT DISTINCT c.conrelid::regclass::text FROM pg_constraint c
 WHERE c.contype='f' AND c.confrelid='public.user_assets'::regclass
 ORDER BY 1`).Scan(&tables).Error; err != nil {
		t.Fatal(err)
	}
	inventoried := map[string]bool{
		"document_extraction_runs":    true, // document_extractions
		"document_indicator_reviews":  true, // document_extractions
		"posture_analysis_runs":       true, // posture_analyses
		"user_asset_storage_cleanups": true, // source_objects
		"asset_erasure_requests":      true, // the receipt itself
	}
	for _, table := range tables {
		if !inventoried[strings.TrimPrefix(table, "public.")] {
			t.Errorf("table %s references assets but is not in the erasure inventory", table)
		}
	}
}
