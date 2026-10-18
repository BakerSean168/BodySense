package service

import (
	"bytes"
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/uploadstorage"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type outageStore struct {
	uploadstorage.Store
	fail atomic.Bool
}

func (s *outageStore) Delete(ctx context.Context, key string) error {
	if s.fail.Load() {
		return errors.New("synthetic object store outage")
	}
	return s.Store.Delete(ctx, key)
}

type assetErasureFixture struct {
	db       *gorm.DB
	store    *outageStore
	assets   *repository.UserAssetRepository
	erasure  *AssetErasureService
	owner    uuid.UUID
	report   *model.UserAsset
	photo    *model.UserAsset
	foreignA *model.UserAsset
}

func newAssetErasureFixture(t *testing.T) *assetErasureFixture {
	t.Helper()
	dsn := os.Getenv("BODYSENSE_INTEGRATION_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires isolated PostgreSQL 18")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	local, err := uploadstorage.NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store := &outageStore{Store: local}
	registry, err := uploadstorage.NewRegistryFromStores("local", store)
	if err != nil {
		t.Fatal(err)
	}
	f := &assetErasureFixture{db: db, store: store, assets: repository.NewUserAssetRepository(db)}
	uploads := NewUploadService(f.assets, nil, nil, nil, nil, registry)
	f.erasure = NewAssetErasureService(repository.NewAssetErasureRepository(db), uploads)
	f.owner = f.seedUser(t)
	foreign := f.seedUser(t)
	f.report = f.seedAsset(t, f.owner, model.UserAssetKindReport)
	f.photo = f.seedAsset(t, f.owner, model.UserAssetKindPhoto)
	f.foreignA = f.seedAsset(t, foreign, model.UserAssetKindReport)
	return f
}

func (f *assetErasureFixture) seedUser(t *testing.T) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if err := f.db.Exec("INSERT INTO users(id,email,password_hash) VALUES(?,?,?)", id, "as002-"+id.String()+"@example.test", "synthetic").Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.db.Exec("DELETE FROM users WHERE id=?", id) })
	return id
}

func (f *assetErasureFixture) seedAsset(t *testing.T, owner uuid.UUID, kind model.UserAssetKind) *model.UserAsset {
	t.Helper()
	payload := []byte("synthetic-private-source-" + string(kind))
	asset := &model.UserAsset{ID: uuid.New(), UserID: owner, Kind: kind, OriginalName: "private-name", MimeType: "application/pdf",
		ByteSize: int64(len(payload)), ContentSHA256: strings.Repeat("a", 64), StorageBackend: "local", Availability: model.UserAssetAvailable}
	asset.StorageKey = owner.String() + "/" + asset.ID.String() + "/original"
	if err := f.assets.Create(context.Background(), asset); err != nil {
		t.Fatal(err)
	}
	if err := f.store.Put(context.Background(), asset.StorageKey, bytes.NewReader(payload), int64(len(payload)), asset.MimeType); err != nil {
		t.Fatal(err)
	}
	return asset
}

func (f *assetErasureFixture) count(t *testing.T, query string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := f.db.Raw(query, args...).Scan(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func (f *assetErasureFixture) exists(t *testing.T, asset *model.UserAsset) bool {
	t.Helper()
	ok, err := f.store.Exists(context.Background(), asset.StorageKey)
	if err != nil {
		t.Fatal(err)
	}
	return ok
}

func targetState(receipt *model.AssetErasureReceipt, name model.AssetErasureTargetName) model.AssetErasureTarget {
	for _, target := range receipt.Targets {
		if target.Target == name {
			return target
		}
	}
	return model.AssetErasureTarget{}
}

// A storage outage leaves the source target outstanding while every other
// family is erased and verified; the asset is source-removed, never reported
// erased. Repeating the command after recovery erases it; further repeats are
// idempotent. Another asset of the same owner and a foreign asset are untouched.
func TestUserAssetErasureSourceRetryPostgres(t *testing.T) {
	f := newAssetErasureFixture(t)
	ctx := context.Background()
	hash := strings.Repeat("b", 64)
	run := uuid.New()
	if err := f.db.Exec(`INSERT INTO document_extraction_runs(id,user_id,asset_id,status,configuration_id,mechanism_revision,document_sha256,result_sha256,raw_text_sha256,result_payload)
 VALUES (?,?,?,'completed','synthetic','1',?,?,?,'{"raw_text":"synthetic"}')`, run, f.owner, f.report.ID, hash, hash, hash).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Exec(`INSERT INTO document_indicator_reviews(user_id,asset_id,extraction_run_id,indicator_index,action,idempotency_key,reviewer_user_id)
 VALUES (?,?,?,0,'confirm','r1',?)`, f.owner, f.report.ID, run, f.owner).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Exec(`INSERT INTO jobs(user_id,job_type,status,input,result) VALUES (?,'ocr','completed',jsonb_build_object('asset_id',?::text),'{"raw_text":"synthetic"}')`,
		f.owner, f.report.ID.String()).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Exec(`INSERT INTO posture_analysis_runs(user_id,asset_id,view,status,result_payload) VALUES (?,?,'front','completed','{"finding":"synthetic"}')`,
		f.owner, f.photo.ID).Error; err != nil {
		t.Fatal(err)
	}

	f.store.fail.Store(true)
	receipt, err := f.erasure.Request(ctx, f.owner, f.report.ID, "erase-report")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Request.Status != model.AssetErasureOutstanding || receipt.Request.ErasedAt != nil {
		t.Fatalf("outage receipt=%+v", receipt.Request)
	}
	source := targetState(receipt, model.AssetErasureSourceObjects)
	if source.State != model.AssetErasureTargetFailed || source.FailureCode == nil || *source.FailureCode != model.AssetErasureCodeObjectDeleteFailed || source.OutstandingCount < 1 {
		t.Fatalf("source target=%+v", source)
	}
	for _, name := range []model.AssetErasureTargetName{model.AssetErasureDerivedJobs, model.AssetErasureDocumentExtractions, model.AssetErasurePostureAnalyses,
		model.AssetErasureDiagnosisRecords, model.AssetErasureAssessmentReports, model.AssetErasureRuntimeCheckpoints} {
		if target := targetState(receipt, name); target.State != model.AssetErasureTargetVerified {
			t.Fatalf("%s=%+v", name, target)
		}
	}
	if manifest := targetState(receipt, model.AssetErasureManifest); manifest.State == model.AssetErasureTargetVerified {
		t.Fatalf("manifest verified early: %+v", manifest)
	}
	current, err := f.assets.GetOwnedByID(ctx, f.report.ID, f.owner)
	if err != nil || current.Availability != model.UserAssetSourceRemoved || current.OriginalName == "" {
		t.Fatalf("asset during outage=%+v err=%v", current, err)
	}
	if !f.exists(t, f.report) {
		t.Fatal("outage test needs the object to survive")
	}
	if n := f.count(t, `SELECT count(*) FROM document_extraction_runs WHERE asset_id=?`, f.report.ID); n != 0 {
		t.Fatalf("extractions kept: %d", n)
	}

	f.store.fail.Store(false)
	receipt, err = f.erasure.Request(ctx, f.owner, f.report.ID, "erase-report")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Request.Status != model.AssetErasureErased || receipt.Request.ErasedAt == nil || len(receipt.Outstanding()) != 0 {
		t.Fatalf("retry receipt=%+v outstanding=%+v", receipt.Request, receipt.Outstanding())
	}
	if f.exists(t, f.report) {
		t.Fatal("source object survived erasure")
	}
	if n := f.count(t, `SELECT count(*) FROM user_asset_storage_cleanups WHERE asset_id=?`, f.report.ID); n != 0 {
		t.Fatalf("replica ledger kept: %d", n)
	}
	current, _ = f.assets.GetOwnedByID(ctx, f.report.ID, f.owner)
	if current.Availability != model.UserAssetErased || current.ErasedAt == nil || current.OriginalName != "" ||
		current.ContentSHA256 != "" || current.Kind != f.report.Kind || current.ByteSize != f.report.ByteSize || current.MimeType != f.report.MimeType {
		t.Fatalf("tombstone=%+v", current)
	}
	counts := map[model.AssetErasureTargetName]int{}
	for _, target := range receipt.Targets {
		counts[target.Target] = target.ErasedCount
	}

	again, err := f.erasure.Request(ctx, f.owner, f.report.ID, "a-new-key")
	if err != nil || again.Request.ID != receipt.Request.ID || again.Request.Status != model.AssetErasureErased || again.Request.AttemptCount != receipt.Request.AttemptCount {
		t.Fatalf("repeat=%+v err=%v", again, err)
	}
	for _, target := range again.Targets {
		if target.ErasedCount != counts[target.Target] {
			t.Fatalf("repeat changed %s: %d != %d", target.Target, target.ErasedCount, counts[target.Target])
		}
	}
	if _, err := f.erasure.Request(ctx, f.owner, f.photo.ID, "erase-report"); !errors.Is(err, model.ErrAssetErasureIdempotencyUsed) {
		t.Fatalf("key reuse: %v", err)
	}
	if _, err := f.erasure.Request(ctx, f.owner, f.foreignA.ID, "foreign"); !errors.Is(err, model.ErrAssetErasureNotFound) {
		t.Fatalf("foreign: %v", err)
	}
	if _, err := f.erasure.Receipt(ctx, f.owner, f.photo.ID); !errors.Is(err, model.ErrAssetErasureNotFound) {
		t.Fatalf("no request yet: %v", err)
	}
	if !f.exists(t, f.photo) || !f.exists(t, f.foreignA) {
		t.Fatal("unrelated objects removed")
	}
	if n := f.count(t, `SELECT count(*) FROM posture_analysis_runs WHERE asset_id=?`, f.photo.ID); n != 1 {
		t.Fatalf("unrelated posture run: %d", n)
	}
	photo, _ := f.assets.GetOwnedByID(ctx, f.photo.ID, f.owner)
	if photo.Availability != model.UserAssetAvailable {
		t.Fatalf("unrelated availability=%s", photo.Availability)
	}
}

// The worker resumes an interrupted pass whose lease expired and retries a
// transient failure, but never re-runs an erased request.
func TestUserAssetErasureWorkerRecoveryPostgres(t *testing.T) {
	f := newAssetErasureFixture(t)
	ctx := context.Background()
	f.erasure.retryAfter = 0
	f.store.fail.Store(true)
	receipt, err := f.erasure.Request(ctx, f.owner, f.photo.ID, "worker")
	if err != nil || receipt.Request.Status != model.AssetErasureOutstanding {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	f.store.fail.Store(false)
	if _, err := f.erasure.Recover(ctx, 50); err != nil {
		t.Fatal(err)
	}
	receipt, err = f.erasure.Receipt(ctx, f.owner, f.photo.ID)
	if err != nil || receipt.Request.Status != model.AssetErasureErased {
		t.Fatalf("recovered=%+v err=%v", receipt, err)
	}
	attempts := receipt.Request.AttemptCount
	if _, err := f.erasure.Recover(ctx, 50); err != nil {
		t.Fatal(err)
	}
	receipt, _ = f.erasure.Receipt(ctx, f.owner, f.photo.ID)
	if receipt.Request.AttemptCount != attempts {
		t.Fatal("erased request re-run")
	}
}

type fakeRuntimeEraser struct {
	scopes   []model.AssetRuntimeScope
	receipts []func(asset uuid.UUID, scope model.AssetRuntimeScope) (*model.RuntimeErasureReceipt, error)
}

func (f *fakeRuntimeEraser) EraseAssetRuntimeCopies(_ context.Context, _ uuid.UUID, assetID uuid.UUID, scope model.AssetRuntimeScope) (*model.RuntimeErasureReceipt, error) {
	f.scopes = append(f.scopes, scope)
	next := f.receipts[0]
	if len(f.receipts) > 1 {
		f.receipts = f.receipts[1:]
	}
	return next(assetID, scope)
}

func runtimeReceipt(status string, threads ...model.RuntimeThreadReceipt) func(uuid.UUID, model.AssetRuntimeScope) (*model.RuntimeErasureReceipt, error) {
	return func(asset uuid.UUID, _ model.AssetRuntimeScope) (*model.RuntimeErasureReceipt, error) {
		return &model.RuntimeErasureReceipt{Status: status, SubjectID: asset.String(), Threads: threads}, nil
	}
}

// seedPhotoConversation creates the owner's Conversation with one user
// message attaching the photo asset.
func (f *assetErasureFixture) seedPhotoConversation(t *testing.T) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if err := f.db.Exec(`INSERT INTO conversations(id,user_id) VALUES (?,?)`, id, f.owner).Error; err != nil {
		t.Fatal(err)
	}
	parts := `[{"type":"image","upload_id":"` + f.photo.ID.String() + `"}]`
	if err := f.db.Exec(`INSERT INTO messages(conversation_id,turn_id,role,status,seq,parts) VALUES (?,?,'user','completed',1,?::jsonb)`,
		id, uuid.New(), parts).Error; err != nil {
		t.Fatal(err)
	}
	return id
}

// The runtime target hands the ai-service every conversation of the owner
// (one today; marking the attached ones) and verifies only on a receipt covering all of
// them. A conversation waiting on Ask User keeps the asset outstanding (never
// erased early); the worker retries, and the attaching message itself stays.
func TestUserAssetErasureRuntimeCheckpointsPostgres(t *testing.T) {
	f := newAssetErasureFixture(t)
	ctx := context.Background()
	f.erasure.retryAfter = 0
	attached := f.seedPhotoConversation(t)
	busy := "thread_busy"
	runtime := &fakeRuntimeEraser{receipts: []func(uuid.UUID, model.AssetRuntimeScope) (*model.RuntimeErasureReceipt, error){
		func(uuid.UUID, model.AssetRuntimeScope) (*model.RuntimeErasureReceipt, error) {
			return nil, errors.New("synthetic ai-service outage")
		},
		runtimeReceipt("outstanding",
			model.RuntimeThreadReceipt{ThreadID: attached.String(), Status: "outstanding", FailureCode: &busy}),
		runtimeReceipt("erased",
			model.RuntimeThreadReceipt{ThreadID: attached.String(), Status: "erased", RedactedRefs: 1, DeletedCheckpoints: 6}),
	}}

	// Without a runtime eraser the target cannot verify.
	receipt, err := f.erasure.Request(ctx, f.owner, f.photo.ID, "runtime")
	if err != nil {
		t.Fatal(err)
	}
	target := targetState(receipt, model.AssetErasureRuntimeCheckpoints)
	if receipt.Request.Status != model.AssetErasureOutstanding || target.State != model.AssetErasureTargetFailed ||
		target.FailureCode == nil || *target.FailureCode != model.AssetErasureCodeRuntimeUnavailable || target.OutstandingCount != 1 {
		t.Fatalf("no eraser: request=%s target=%+v", receipt.Request.Status, target)
	}

	f.erasure.WithRuntimeEraser(runtime)
	wantRuntime := func(state model.AssetErasureTargetState, code string, status model.AssetErasureStatus) *model.AssetErasureReceipt {
		t.Helper()
		if _, err := f.erasure.Recover(ctx, 50); err != nil {
			t.Fatal(err)
		}
		receipt, err := f.erasure.Receipt(ctx, f.owner, f.photo.ID)
		if err != nil {
			t.Fatal(err)
		}
		target := targetState(receipt, model.AssetErasureRuntimeCheckpoints)
		gotCode := ""
		if target.FailureCode != nil {
			gotCode = *target.FailureCode
		}
		if target.State != state || gotCode != code || receipt.Request.Status != status {
			t.Fatalf("runtime target=%+v code=%s request=%s", target, gotCode, receipt.Request.Status)
		}
		return receipt
	}
	wantRuntime(model.AssetErasureTargetFailed, model.AssetErasureCodeRuntimeUnavailable, model.AssetErasureOutstanding)
	wantRuntime(model.AssetErasureTargetFailed, model.AssetErasureCodeRuntimeThreadBusy, model.AssetErasureOutstanding)
	photo, _ := f.assets.GetOwnedByID(ctx, f.photo.ID, f.owner)
	if photo.Availability != model.UserAssetSourceRemoved {
		t.Fatalf("asset erased while runtime copies are outstanding: %s", photo.Availability)
	}
	receipt = wantRuntime(model.AssetErasureTargetVerified, "", model.AssetErasureErased)
	if target := targetState(receipt, model.AssetErasureRuntimeCheckpoints); target.ErasedCount != 7 {
		t.Fatalf("runtime erased count=%d", target.ErasedCount)
	}

	scope := runtime.scopes[len(runtime.scopes)-1]
	if !slices.Equal(scope.ThreadIDs, []uuid.UUID{attached}) || !slices.Equal(scope.AttachedThreadIDs, []uuid.UUID{attached}) ||
		scope.ContentSHA256 != f.photo.ContentSHA256 {
		t.Fatalf("scope=%+v", scope)
	}
	if n := f.count(t, `SELECT count(*) FROM messages WHERE conversation_id=?`, attached); n != 1 {
		t.Fatalf("attaching message removed: %d", n)
	}
	calls := len(runtime.scopes)
	if _, err := f.erasure.Request(ctx, f.owner, f.photo.ID, "runtime"); err != nil || len(runtime.scopes) != calls {
		t.Fatalf("erased request called the runtime again: %v", err)
	}
}
