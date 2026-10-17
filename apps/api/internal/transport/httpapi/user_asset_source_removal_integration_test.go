package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
	"github.com/bodysense/api/internal/uploadstorage"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func strPtr(value string) *string { return &value }

// assetRemovalIntegrationRouter wires the real asset/extraction/review
// repositories and services behind the generated public routes, acting as one
// fixed authenticated user.
func assetRemovalIntegrationRouter(t *testing.T, db *gorm.DB, actingUser uuid.UUID, registry *uploadstorage.Registry) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	spec, err := openapiv1.GetSpec()
	if err != nil {
		t.Fatalf("load generated OpenAPI spec: %v", err)
	}
	assetRepo := repository.NewUserAssetRepository(db)
	extractionRepo := repository.NewDocumentExtractionRunRepository(db)
	postureRepo := repository.NewPostureAnalysisRunRepository(db)
	reviewRepo := repository.NewDocumentIndicatorReviewRepository(db)
	uploadService := service.NewUploadService(assetRepo, extractionRepo, postureRepo, nil, nil, registry)
	reviewService := service.NewHealthDocumentReviewService(extractionRepo, reviewRepo, assetRepo)
	router := gin.New()
	RegisterRoutes(router, StrictHandler(NewPublicServer().WithUploads(uploadService, reviewService)), RouteSecurity{
		Auth: func(c *gin.Context) {
			c.Set("user_id", actingUser.String())
			c.Next()
		},
		Operator:  func(c *gin.Context) { c.Next() },
		Validator: RequestValidator(spec),
	})
	return router
}

func assetIntegrationForm(t *testing.T, filename, contentType string, payload []byte) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(map[string][]string)
	header["Content-Disposition"] = []string{fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename)}
	header["Content-Type"] = []string{contentType}
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
	if err := req.ParseMultipartForm(10 << 20); err != nil {
		t.Fatal(err)
	}
	return req.MultipartForm.File["file"][0]
}

const assetRemovalIndicatorSnapshot = `[{
	"indicator_id":"hemoglobin","name":"hb","value":142,"unit":"g/L","reference_range":"120-160",
	"parser_revision":"parser-v1","source_refs":["page:1:ocr-block:5"],
	"evidence_admissibility":{"status":"needs_review","policy_revision":"ocr-indicator-admissibility-v1","reason_codes":["manual_confirmation_required"]},
	"evidence_verification":{"status":"verified_consensus","revision":"ver-v1"}
}]`
const assetRemovalSourceSummary = `{"source_blocks":[{"source_ref":"page:1:ocr-block:5","page_number":1,"bbox":[1,2,3,4]}],"indicator_count":1,"source_block_count":1}`

// TestUserAssetSourceRemovalPostgres is the AS-001 slice-1 acceptance against
// a real PostgreSQL: owner-masked 404, idempotent source removal that keeps
// adopted evidence readable with the source_unavailable marker, fail-closed
// source streaming, and composite-FK owner scoping on the derived tables.
func TestUserAssetSourceRemovalPostgres(t *testing.T) {
	dsn := os.Getenv("BODYSENSE_INTEGRATION_DATABASE_URL")
	if dsn == "" {
		t.Skip("needs isolated native PostgreSQL, not live env")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	owner, foreign := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{owner, foreign} {
		if err := db.Exec("INSERT INTO users(id,email,password_hash) VALUES(?,?,?)",
			id, "asset-removal-"+id.String()+"@example.test", "synthetic").Error; err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Exec("DELETE FROM users WHERE id=?", id).Error; err != nil {
				t.Errorf("delete isolated user: %v", err)
			}
		})
	}

	registry, err := uploadstorage.NewRegistry(uploadstorage.Config{
		Environment: "test", Backend: "local", LocalRoot: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	assetRepo := repository.NewUserAssetRepository(db)
	extractionRepo := repository.NewDocumentExtractionRunRepository(db)
	postureRepo := repository.NewPostureAnalysisRunRepository(db)
	reviewRepo := repository.NewDocumentIndicatorReviewRepository(db)
	uploadService := service.NewUploadService(assetRepo, extractionRepo, postureRepo, nil, nil, registry)
	reviewService := service.NewHealthDocumentReviewService(extractionRepo, reviewRepo, assetRepo)
	ownerRouter := assetRemovalIntegrationRouter(t, db, owner, registry)
	foreignRouter := assetRemovalIntegrationRouter(t, db, foreign, registry)

	// 1. The owner uploads a report; a derived pending extraction run appears.
	pdf := append([]byte("%PDF-1.4\n"), []byte("synthetic report bytes")...)
	view, err := uploadService.UploadFile(ctx, owner, assetIntegrationForm(t, "report.pdf", "application/pdf", pdf), "report")
	if err != nil {
		t.Fatalf("upload report: %v", err)
	}
	assetID := view.ID
	pendingRun, err := extractionRepo.GetLatestOwnedByAsset(ctx, assetID, owner)
	if err != nil || pendingRun == nil || pendingRun.Status != model.DerivedRunPending {
		t.Fatalf("pending extraction run missing: run=%#v err=%v", pendingRun, err)
	}

	// 2. The OCR lifecycle completes with the durable evidence snapshot. The
	// job row exists first, exactly like the real worker's flow.
	jobID := uuid.New()
	if err := db.Exec("INSERT INTO jobs(id,user_id,job_type,status,input) VALUES(?,?,?,?,?)",
		jobID, owner, "upload.ocr_extract", "pending", []byte(`{"asset_id":"`+assetID.String()+`"}`)).Error; err != nil {
		t.Fatalf("seed job row: %v", err)
	}
	if started, err := extractionRepo.BeginProcessing(ctx, pendingRun.ID, owner, jobID); err != nil || !started {
		t.Fatalf("begin extraction: started=%v err=%v", started, err)
	}
	completion := &model.DocumentExtractionRun{
		ID: pendingRun.ID, UserID: owner, AssetID: assetID,
		MechanismRevision:   strPtr("m1"),
		DocumentSHA256:      strPtr("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		ResultSHA256:        strPtr("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
		RawTextSHA256:       strPtr("cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"),
		ResultPayload:       datatypes.JSON(`{"status":"completed","result":{"raw_text":"private report text","indicators":[]}}`),
		IndicatorSnapshot:   datatypes.JSON(assetRemovalIndicatorSnapshot),
		SourceSummary:       datatypes.JSON(assetRemovalSourceSummary),
		MechanismProvenance: datatypes.JSON(`{}`),
	}
	if completed, err := extractionRepo.CompleteRun(ctx, completion); err != nil || !completed {
		t.Fatalf("complete extraction: completed=%v err=%v", completed, err)
	}

	// 3. The user admits one indicator: the adopted evidence history.
	review, err := reviewService.ApplyReview(ctx, owner, service.ReviewRequest{
		ExtractionRunID: pendingRun.ID, IndicatorIndex: 0, IndicatorID: "hemoglobin",
		Action: string(model.ReviewActionConfirm), SourceRefs: []string{"page:1:ocr-block:5"},
		IdempotencyKey: "asset-removal-confirm-1",
	})
	if err != nil {
		t.Fatalf("apply review: %v", err)
	}

	// 4. Evidence is readable while the source is available.
	contextURI := "/api/v1/uploads/" + assetID.String() + "/health-document-review"
	rec := performJSONAt(ownerRouter, http.MethodGet, contextURI, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("review context before removal status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !json.Valid(rec.Body.Bytes()) || !bytes.Contains(rec.Body.Bytes(), []byte(`"source_availability":"available"`)) {
		t.Fatalf("context must be available before removal: %s", rec.Body.String())
	}

	// 5. Composite FK: a foreign owner cannot attach rows to this asset.
	foreignReview := &model.DocumentIndicatorReview{
		UserID: foreign, AssetID: assetID, ExtractionRunID: pendingRun.ID,
		IndicatorIndex: 1, Action: "confirm", IdempotencyKey: "fk-probe",
		ReviewerUserID: foreign,
	}
	if err := reviewRepo.Create(ctx, foreignReview); err == nil {
		t.Fatal("composite FK must reject a foreign (user_id, asset_id) review row")
	}
	foreignRun := &model.DocumentExtractionRun{
		UserID: foreign, AssetID: assetID, Status: model.DerivedRunPending, ConfigurationID: "c1",
	}
	if err := extractionRepo.Create(ctx, foreignRun); err == nil {
		t.Fatal("composite FK must reject a foreign (user_id, asset_id) extraction run")
	}

	// 6. A foreign caller gets the owner-masked 404 on source removal.
	removalURI := "/api/v1/assets/" + assetID.String()
	if rec := performJSONAt(foreignRouter, http.MethodDelete, removalURI, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("foreign removal status=%d want=404 body=%s", rec.Code, rec.Body.String())
	}

	// 7. The owner removes the source; repeating the command is idempotent.
	for attempt := 1; attempt <= 2; attempt++ {
		rec := performJSONAt(ownerRouter, http.MethodDelete, removalURI, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("owner removal attempt %d status=%d body=%s", attempt, rec.Code, rec.Body.String())
		}
		var payload struct {
			Availability string `json:"availability"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil || payload.Availability != "source_removed" {
			t.Fatalf("removal attempt %d availability=%q body=%s", attempt, payload.Availability, rec.Body.String())
		}
	}
	var asset model.UserAsset
	if err := db.Where("id = ?", assetID).First(&asset).Error; err != nil {
		t.Fatalf("asset identity must survive source removal: %v", err)
	}
	if asset.Availability != model.UserAssetSourceRemoved || asset.SourceRemovedAt == nil {
		t.Fatalf("asset must be source_removed, not erased or deleted: %#v", asset)
	}
	var reviewCount int64
	if err := db.Model(&model.DocumentIndicatorReview{}).Where("asset_id = ?", assetID).Count(&reviewCount).Error; err != nil {
		t.Fatal(err)
	}
	if reviewCount != 1 {
		t.Fatalf("source removal must retain adopted evidence, reviews=%d", reviewCount)
	}

	// 8. Evidence stays readable and is explicitly source_unavailable (CD-D4).
	rec = performJSONAt(ownerRouter, http.MethodGet, contextURI, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("review context after removal status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"source_availability":"source_unavailable"`)) {
		t.Fatalf("post-removal context must be marked source_unavailable: %s", body)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"effective_review"`)) || !bytes.Contains(rec.Body.Bytes(), []byte(review.ReviewID.String())) {
		t.Fatalf("adopted review must remain in the post-removal projection: %s", body)
	}

	// 9. The original is not streamable anymore: fail closed (CD-D4).
	sourceURI := "/api/v1/uploads/" + assetID.String() + "/extractions/" + pendingRun.ID.String() + "/source"
	rec = performJSONAt(ownerRouter, http.MethodGet, sourceURI, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("post-removal source stream status=%d want=404 body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("SOURCE_UNAVAILABLE")) {
		t.Fatalf("source stream must name the source-unavailable cause: %s", rec.Body.String())
	}
	if _, err := registry.DefaultStore().Stat(ctx, asset.StorageKey); err == nil {
		t.Fatal("source removal must delete the private original object")
	}
}
