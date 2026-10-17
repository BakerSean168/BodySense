package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"mime/multipart"
	"net/http/httptest"
	"net/textproto"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/uploadstorage"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// fakeUserAssetRepository is the in-memory stand-in for the asset identity
// repository, including the available -> source_removed compare-and-set.
type fakeUserAssetRepository struct {
	created   *model.UserAsset
	createErr error
	byID      map[uuid.UUID]*model.UserAsset
	removed   []uuid.UUID
	replicas  map[model.UserAssetStorageCleanup]bool
}

func (r *fakeUserAssetRepository) Create(_ context.Context, asset *model.UserAsset) error {
	if r.createErr != nil {
		return r.createErr
	}
	copyValue := *asset
	r.created = &copyValue
	if r.byID == nil {
		r.byID = map[uuid.UUID]*model.UserAsset{}
	}
	r.byID[asset.ID] = &copyValue
	return nil
}

func (r *fakeUserAssetRepository) GetOwnedByID(_ context.Context, id, userID uuid.UUID) (*model.UserAsset, error) {
	asset := r.byID[id]
	if asset == nil || asset.UserID != userID {
		return nil, nil
	}
	return asset, nil
}

func (r *fakeUserAssetRepository) ListByUserID(_ context.Context, userID uuid.UUID) ([]model.UserAsset, error) {
	var result []model.UserAsset
	for _, asset := range r.byID {
		if asset.UserID == userID {
			result = append(result, *asset)
		}
	}
	return result, nil
}

func (r *fakeUserAssetRepository) MarkSourceRemoved(_ context.Context, id, userID uuid.UUID) (bool, error) {
	asset := r.byID[id]
	if asset == nil || asset.UserID != userID || asset.Availability != model.UserAssetAvailable {
		return false, nil
	}
	now := time.Now().UTC()
	asset.Availability = model.UserAssetSourceRemoved
	asset.SourceRemovedAt = &now
	r.removed = append(r.removed, id)
	return true, nil
}

// fakeExtractionRunRepository records derived extraction-run persistence.
type fakeExtractionRunRepository struct {
	created []*model.DocumentExtractionRun
	byAsset map[uuid.UUID]*model.DocumentExtractionRun
}

func (r *fakeExtractionRunRepository) Create(_ context.Context, run *model.DocumentExtractionRun) error {
	copyValue := *run
	r.created = append(r.created, &copyValue)
	if r.byAsset == nil {
		r.byAsset = map[uuid.UUID]*model.DocumentExtractionRun{}
	}
	r.byAsset[run.AssetID] = &copyValue
	return nil
}

func (r *fakeExtractionRunRepository) GetOwnedByID(_ context.Context, id, userID uuid.UUID) (*model.DocumentExtractionRun, error) {
	for _, run := range r.byAsset {
		if run.ID == id && run.UserID == userID {
			return run, nil
		}
	}
	return nil, gormErrRecordNotFound()
}

func (r *fakeExtractionRunRepository) GetLatestOwnedByAsset(_ context.Context, assetID, userID uuid.UUID) (*model.DocumentExtractionRun, error) {
	run := r.byAsset[assetID]
	if run == nil || run.UserID != userID {
		return nil, nil
	}
	return run, nil
}

func (r *fakeExtractionRunRepository) ListByUser(_ context.Context, userID uuid.UUID) ([]model.DocumentExtractionRun, error) {
	var runs []model.DocumentExtractionRun
	for _, run := range r.byAsset {
		if run.UserID == userID {
			runs = append(runs, *run)
		}
	}
	return runs, nil
}

func (r *fakeExtractionRunRepository) BeginProcessing(_ context.Context, runID, userID, jobID uuid.UUID) (bool, error) {
	for _, run := range r.byAsset {
		if run.ID == runID && run.UserID == userID && run.Status == model.DerivedRunPending {
			run.Status = model.DerivedRunProcessing
			id := jobID
			run.JobID = &id
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeExtractionRunRepository) CompleteRun(_ context.Context, completion *model.DocumentExtractionRun) (bool, error) {
	for _, run := range r.byAsset {
		if run.ID == completion.ID && run.UserID == completion.UserID && run.Status == model.DerivedRunProcessing {
			run.Status = model.DerivedRunCompleted
			run.MechanismRevision = completion.MechanismRevision
			run.DocumentSHA256 = completion.DocumentSHA256
			run.ResultSHA256 = completion.ResultSHA256
			run.RawTextSHA256 = completion.RawTextSHA256
			run.ResultPayload = completion.ResultPayload
			run.IndicatorSnapshot = completion.IndicatorSnapshot
			run.SourceSummary = completion.SourceSummary
			run.MechanismProvenance = completion.MechanismProvenance
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeExtractionRunRepository) FailRun(_ context.Context, runID, userID uuid.UUID, payload json.RawMessage) (bool, error) {
	for _, run := range r.byAsset {
		if run.ID == runID && run.UserID == userID && !run.Status.IsTerminal() {
			run.Status = model.DerivedRunFailed
			run.ResultPayload = datatypes.JSON(payload)
			return true, nil
		}
	}
	return false, nil
}

// fakePostureRunRepository records derived posture-run persistence.
type fakePostureRunRepository struct {
	created []*model.PostureAnalysisRun
	byAsset map[uuid.UUID]*model.PostureAnalysisRun
}

func (r *fakePostureRunRepository) Create(_ context.Context, run *model.PostureAnalysisRun) error {
	copyValue := *run
	r.created = append(r.created, &copyValue)
	if r.byAsset == nil {
		r.byAsset = map[uuid.UUID]*model.PostureAnalysisRun{}
	}
	r.byAsset[run.AssetID] = &copyValue
	return nil
}

func (r *fakePostureRunRepository) GetOwnedByAsset(_ context.Context, assetID, userID uuid.UUID) (*model.PostureAnalysisRun, error) {
	run := r.byAsset[assetID]
	if run == nil || run.UserID != userID {
		return nil, nil
	}
	return run, nil
}

func (r *fakePostureRunRepository) ListCompletedByUser(_ context.Context, userID uuid.UUID) ([]model.PostureAnalysisRun, error) {
	var runs []model.PostureAnalysisRun
	for _, run := range r.byAsset {
		if run.UserID == userID && run.Status == model.DerivedRunCompleted {
			runs = append(runs, *run)
		}
	}
	return runs, nil
}

func (r *fakePostureRunRepository) ListByUser(_ context.Context, userID uuid.UUID) ([]model.PostureAnalysisRun, error) {
	var runs []model.PostureAnalysisRun
	for _, run := range r.byAsset {
		if run.UserID == userID {
			runs = append(runs, *run)
		}
	}
	return runs, nil
}

func (r *fakePostureRunRepository) BeginProcessing(_ context.Context, runID, userID, jobID uuid.UUID) (bool, error) {
	for _, run := range r.byAsset {
		if run.ID == runID && run.UserID == userID && run.Status == model.DerivedRunPending {
			run.Status = model.DerivedRunProcessing
			id := jobID
			run.JobID = &id
			return true, nil
		}
	}
	return false, nil
}

func (r *fakePostureRunRepository) CompleteRun(_ context.Context, runID, userID uuid.UUID, agentConfigurationID string, payload json.RawMessage) (bool, error) {
	for _, run := range r.byAsset {
		if run.ID == runID && run.UserID == userID && run.Status == model.DerivedRunProcessing {
			run.Status = model.DerivedRunCompleted
			run.AgentConfigurationID = agentConfigurationID
			run.ResultPayload = datatypes.JSON(payload)
			return true, nil
		}
	}
	return false, nil
}

func (r *fakePostureRunRepository) FailRun(_ context.Context, runID, userID uuid.UUID, payload json.RawMessage) (bool, error) {
	for _, run := range r.byAsset {
		if run.ID == runID && run.UserID == userID && !run.Status.IsTerminal() {
			run.Status = model.DerivedRunFailed
			run.ResultPayload = datatypes.JSON(payload)
			return true, nil
		}
	}
	return false, nil
}

func gormErrRecordNotFound() error { return errors.New("record not found") }

func testUploadRegistry(t *testing.T) (*uploadstorage.Registry, string) {
	t.Helper()
	root := t.TempDir()
	registry, err := uploadstorage.NewRegistry(uploadstorage.Config{
		Environment: "test",
		Backend:     "local",
		LocalRoot:   root,
	})
	if err != nil {
		t.Fatal(err)
	}
	return registry, root
}

func multipartUploadHeader(t *testing.T, filename, contentType string, payload []byte) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if err := req.ParseMultipartForm(MaxFileSize); err != nil {
		t.Fatal(err)
	}
	return req.MultipartForm.File["file"][0]
}

func newTestUploadService(assets *fakeUserAssetRepository, extractionRuns *fakeExtractionRunRepository, postureRuns *fakePostureRunRepository, registry *uploadstorage.Registry) *UploadService {
	return NewUploadService(assets, extractionRuns, postureRuns, nil, nil, registry)
}

func TestUploadStorageVerticalWriteReadRemoveSource(t *testing.T) {
	ctx := context.Background()
	registry, _ := testUploadRegistry(t)
	assets := &fakeUserAssetRepository{}
	svc := newTestUploadService(assets, &fakeExtractionRunRepository{}, &fakePostureRunRepository{}, registry)
	userID := uuid.New()
	payload := append([]byte("\x89PNG\r\n\x1a\n"), []byte("private-image-payload")...)
	file := multipartUploadHeader(t, "posture.png", "image/png", payload)

	upload, err := svc.UploadFile(ctx, userID, file, "consultation_photo")
	if err != nil {
		t.Fatalf("UploadFile: %v", err)
	}
	if upload.FileType != "consultation_photo" || upload.OCRStatus != "pending" || upload.AnalysisStatus != "none" {
		t.Fatalf("unexpected projection: %#v", upload)
	}
	if assets.created == nil || assets.created.StorageBackend != "local" ||
		!strings.HasPrefix(assets.created.StorageKey, userID.String()+"/") ||
		!strings.HasSuffix(assets.created.StorageKey, "/original.png") {
		t.Fatalf("unexpected storage identity: %#v", assets.created)
	}
	if len(assets.created.ContentSHA256) != 64 || assets.created.Availability != model.UserAssetAvailable {
		t.Fatalf("asset manifest misses content hash or availability: %#v", assets.created)
	}
	if _, err := registry.DefaultStore().Stat(ctx, assets.created.StorageKey); err != nil {
		t.Fatalf("stored object is missing: %v", err)
	}
	dataURL, mime, err := svc.ReadImageDataURL(ctx, userID, upload.ID)
	if err != nil {
		t.Fatalf("ReadImageDataURL: %v", err)
	}
	if mime != "image/png" || !strings.HasPrefix(dataURL, "data:image/png;base64,") {
		t.Fatalf("unexpected data URL: mime=%q url=%q", mime, dataURL)
	}

	// Source removal deletes the private original but keeps the asset row.
	asset, err := svc.RemoveSource(ctx, userID, upload.ID)
	if err != nil {
		t.Fatalf("RemoveSource: %v", err)
	}
	if asset.Availability != model.UserAssetSourceRemoved || asset.SourceRemovedAt == nil {
		t.Fatalf("asset not marked source_removed: %#v", asset)
	}
	if _, err := registry.DefaultStore().Stat(ctx, assets.created.StorageKey); err == nil {
		t.Fatal("storage object survived source removal")
	}
	if assets.byID[upload.ID] == nil {
		t.Fatal("source removal must not delete the asset identity")
	}
	// The historical view stays readable.
	view, err := svc.GetUpload(ctx, userID, upload.ID)
	if err != nil || view.ID != upload.ID {
		t.Fatalf("evidence view must stay readable after source removal: %v %#v", err, view)
	}
	// The private original is no longer readable: fail closed.
	if _, _, err := svc.ReadImageDataURL(ctx, userID, upload.ID); !errors.Is(err, ErrUploadSourceUnavailable) {
		t.Fatalf("post-removal read must fail closed, got %v", err)
	}
	if _, _, err := svc.OpenOwnedAssetSource(ctx, userID, upload.ID); !errors.Is(err, ErrUploadSourceUnavailable) {
		t.Fatalf("post-removal source open must fail closed, got %v", err)
	}

	// Repeating the command is idempotent and still succeeds.
	again, err := svc.RemoveSource(ctx, userID, upload.ID)
	if err != nil {
		t.Fatalf("idempotent RemoveSource: %v", err)
	}
	if again.Availability != model.UserAssetSourceRemoved || len(assets.removed) != 1 {
		t.Fatalf("repeated removal must not create a second transition: %#v removed=%v", again, assets.removed)
	}
}

func TestRemoveSourceForeignAssetIsOwnerMaskedNotFound(t *testing.T) {
	registry, _ := testUploadRegistry(t)
	assets := &fakeUserAssetRepository{}
	svc := newTestUploadService(assets, &fakeExtractionRunRepository{}, &fakePostureRunRepository{}, registry)
	owner := uuid.New()
	asset := &model.UserAsset{
		ID: uuid.New(), UserID: owner, Kind: model.UserAssetKindPhoto,
		OriginalName: "p.png", MimeType: "image/png", ByteSize: 4,
		ContentSHA256: strings.Repeat("a", 64), StorageBackend: "local", StorageKey: owner.String() + "/k/original.png",
		Availability: model.UserAssetAvailable,
	}
	if err := assets.Create(context.Background(), asset); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RemoveSource(context.Background(), uuid.New(), asset.ID); !errors.Is(err, ErrUploadNotFound) {
		t.Fatalf("foreign removal must be owner-masked not found, got %v", err)
	}
	if _, err := svc.RemoveSource(context.Background(), uuid.New(), uuid.New()); !errors.Is(err, ErrUploadNotFound) {
		t.Fatalf("missing removal must be not found, got %v", err)
	}
	if len(assets.removed) != 0 {
		t.Fatal("no removal may happen for foreign callers")
	}
}

func TestUploadStorageDBFailureRollsBackObject(t *testing.T) {
	registry, root := testUploadRegistry(t)
	assets := &fakeUserAssetRepository{createErr: errors.New("db unavailable")}
	svc := newTestUploadService(assets, &fakeExtractionRunRepository{}, &fakePostureRunRepository{}, registry)
	payload := append([]byte("\x89PNG\r\n\x1a\n"), []byte("rollback-me")...)
	file := multipartUploadHeader(t, "photo.png", "image/png", payload)

	if _, err := svc.UploadFile(context.Background(), uuid.New(), file, "consultation_photo"); err == nil || !strings.Contains(err.Error(), "failed to save upload record") {
		t.Fatalf("expected DB failure, got %v", err)
	}
	var files int
	if err := filepath.WalkDir(root, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			files++
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if files != 0 {
		t.Fatalf("DB failure left %d orphan upload objects", files)
	}
}

func TestUploadStorageRejectsDeclaredMimeMismatchBeforeObjectWrite(t *testing.T) {
	registry, root := testUploadRegistry(t)
	assets := &fakeUserAssetRepository{}
	svc := newTestUploadService(assets, &fakeExtractionRunRepository{}, &fakePostureRunRepository{}, registry)
	payload := append([]byte("\x89PNG\r\n\x1a\n"), []byte("private-image")...)
	file := multipartUploadHeader(t, "spoofed.jpg", "image/jpeg", payload)
	if _, err := svc.UploadFile(context.Background(), uuid.New(), file, "consultation_photo"); err == nil || !strings.Contains(err.Error(), "does not match detected content") {
		t.Fatalf("expected MIME mismatch rejection, got %v", err)
	}
	var files int
	_ = filepath.WalkDir(root, func(_ string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			files++
		}
		return err
	})
	if files != 0 || assets.created != nil {
		t.Fatalf("MIME mismatch crossed storage/DB boundary files=%d created=%v", files, assets.created != nil)
	}
}

// TestUploadCreatesDerivedRunManifests pins the rewire: uploading a report
// creates a pending document-extraction run, and uploading a posture photo
// creates a pending posture run, both bound to the asset identity.
func TestUploadCreatesDerivedRunManifests(t *testing.T) {
	registry, _ := testUploadRegistry(t)
	assets := &fakeUserAssetRepository{}
	extractionRuns := &fakeExtractionRunRepository{}
	postureRuns := &fakePostureRunRepository{}
	svc := newTestUploadService(assets, extractionRuns, postureRuns, registry)
	userID := uuid.New()

	pdf := append([]byte("%PDF-1.4\n"), []byte("%report-bytes")...)
	reportFile := multipartUploadHeader(t, "report.pdf", "application/pdf", pdf)
	report, err := svc.UploadFile(context.Background(), userID, reportFile, "report")
	if err != nil {
		t.Fatalf("upload report: %v", err)
	}
	if len(extractionRuns.created) != 1 {
		t.Fatalf("report upload must create exactly one extraction run, got %d", len(extractionRuns.created))
	}
	if extractionRuns.created[0].AssetID != report.ID || extractionRuns.created[0].Status != model.DerivedRunPending {
		t.Fatalf("unexpected extraction run manifest: %#v", extractionRuns.created[0])
	}
	if extractionRuns.created[0].UserID != userID {
		t.Fatalf("extraction run owner mismatch: %#v", extractionRuns.created[0])
	}
	if len(postureRuns.created) != 0 {
		t.Fatalf("report upload must not create posture runs, got %d", len(postureRuns.created))
	}

	photoFile := multipartUploadHeader(t, "front.png", "image/png", append([]byte("\x89PNG\r\n\x1a\n"), []byte("photo")...))
	photo, err := svc.UploadFile(context.Background(), userID, photoFile, "photo_front")
	if err != nil {
		t.Fatalf("upload photo: %v", err)
	}
	if len(postureRuns.created) != 1 {
		t.Fatalf("posture photo upload must create exactly one posture run, got %d", len(postureRuns.created))
	}
	if postureRuns.created[0].AssetID != photo.ID || postureRuns.created[0].View != "front" || postureRuns.created[0].Status != model.DerivedRunPending {
		t.Fatalf("unexpected posture run manifest: %#v", postureRuns.created[0])
	}
	if len(extractionRuns.created) != 1 {
		t.Fatalf("photo upload must not create extraction runs, got %d", len(extractionRuns.created))
	}
}

func (r *fakeUserAssetRepository) WithStorageLock(ctx context.Context, _ uuid.UUID, fn func(context.Context) error) error {
	return fn(ctx)
}
func (r *fakeUserAssetRepository) TrackStorageReplica(_ context.Context, replica model.UserAssetStorageCleanup) error {
	if r.replicas == nil {
		r.replicas = map[model.UserAssetStorageCleanup]bool{}
	}
	r.replicas[replica] = true
	return nil
}
func (r *fakeUserAssetRepository) ListStorageReplicas(_ context.Context, id, userID uuid.UUID) ([]model.UserAssetStorageCleanup, error) {
	var result []model.UserAssetStorageCleanup
	for replica := range r.replicas {
		if replica.AssetID == id && replica.UserID == userID {
			result = append(result, replica)
		}
	}
	return result, nil
}
func (r *fakeUserAssetRepository) ForgetStorageReplica(_ context.Context, replica model.UserAssetStorageCleanup) error {
	delete(r.replicas, replica)
	return nil
}
