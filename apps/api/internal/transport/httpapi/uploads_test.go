package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeUploadApplication struct {
	uploadFile  *model.UserUploadView
	uploads     []model.UserUploadView
	one         *model.UserUploadView
	getErr      error
	posture     service.PostureAnalysisSummary
	stream      string
	mime        string
	seenFile    *multipart.FileHeader
	seenType    string
	removeCalls int
	removeAsset *model.UserAsset
	removeErr   error
	openErr     error
}

func (f *fakeUploadApplication) UploadFile(_ context.Context, _ uuid.UUID, file *multipart.FileHeader, fileType string) (*model.UserUploadView, error) {
	f.seenFile, f.seenType = file, fileType
	return f.uploadFile, nil
}
func (f *fakeUploadApplication) GetUploads(context.Context, uuid.UUID) ([]model.UserUploadView, error) {
	return f.uploads, nil
}
func (f *fakeUploadApplication) GetUpload(context.Context, uuid.UUID, uuid.UUID) (*model.UserUploadView, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.one, nil
}
func (f *fakeUploadApplication) GetPostureAnalysisSummary(context.Context, uuid.UUID) (service.PostureAnalysisSummary, error) {
	return f.posture, nil
}
func (f *fakeUploadApplication) RemoveSource(_ context.Context, _ uuid.UUID, id uuid.UUID) (*model.UserAsset, error) {
	f.removeCalls++
	if f.removeErr != nil {
		return nil, f.removeErr
	}
	if f.removeAsset != nil {
		return f.removeAsset, nil
	}
	now := time.Now().UTC()
	return &model.UserAsset{ID: id, Availability: model.UserAssetSourceRemoved, SourceRemovedAt: &now}, nil
}
func (f *fakeUploadApplication) OpenOwnedAssetSource(context.Context, uuid.UUID, uuid.UUID) (io.ReadCloser, *model.UserAsset, error) {
	if f.openErr != nil {
		return nil, nil, f.openErr
	}
	asset := &model.UserAsset{ByteSize: int64(len(f.stream)), MimeType: f.mime}
	return io.NopCloser(strings.NewReader(f.stream)), asset, nil
}

type fakeHealthDocumentReviewApplication struct {
	contextValue *service.HealthDocumentReviewContext
	ensureErr    error
	listErr      error
	applyResult  *service.DocumentIndicatorReviewRecord
}

func (f fakeHealthDocumentReviewApplication) CurrentContext(context.Context, uuid.UUID, uuid.UUID) (*service.HealthDocumentReviewContext, error) {
	if f.contextValue == nil {
		return nil, service.ErrReviewContextUnavailable
	}
	return f.contextValue, nil
}
func (f fakeHealthDocumentReviewApplication) EnsureAssetOwnsRun(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return f.ensureErr
}
func (f fakeHealthDocumentReviewApplication) ListCandidates(context.Context, uuid.UUID, uuid.UUID) ([]service.DocumentIndicatorReviewProjection, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return []service.DocumentIndicatorReviewProjection{}, nil
}
func (fakeHealthDocumentReviewApplication) ApplyReview(context.Context, uuid.UUID, service.ReviewRequest) (*service.DocumentIndicatorReviewRecord, error) {
	return nil, nil
}

func uploadTestRouter(t *testing.T, uploads uploadApplication, reviews healthDocumentReviewApplication, authenticated bool) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	spec, err := openapiv1.GetSpec()
	if err != nil {
		t.Fatalf("load generated OpenAPI spec: %v", err)
	}
	r := gin.New()
	RegisterRoutes(r, StrictHandler(NewPublicServer().WithUploads(uploads, reviews)), RouteSecurity{
		Auth: func(c *gin.Context) {
			if authenticated {
				c.Set("user_id", uuid.New().String())
			}
			c.Next()
		},
		Operator:  func(c *gin.Context) { c.Next() },
		Validator: RequestValidator(spec),
	})
	return r
}

func TestUploadPublicProjectionDoesNotExposePersistenceOrStorageAuthority(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	upload := &model.UserUploadView{
		ID: uuid.New(), UserID: uuid.New(), FileType: "consultation_photo", OriginalName: "photo.png",
		FileSize: 10, MimeType: "image/png", OCRStatus: "pending", AnalysisStatus: "none",
		Availability: model.UserAssetAvailable, CreatedAt: now, UpdatedAt: now,
	}
	projected, err := strictUserUpload(upload)
	if err != nil {
		t.Fatalf("project upload: %v", err)
	}
	payload, err := json.Marshal(projected)
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	for _, forbidden := range []string{"user_id", "file_path", "storage_backend", "storage_key", "agent_configuration_id", "availability", "private/"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("public upload leaked %q: %s", forbidden, text)
		}
	}
}

func TestReviewPublicProjectionDoesNotExposeReviewerOrRawStorageAuthority(t *testing.T) {
	record := &service.DocumentIndicatorReviewRecord{
		ReviewID: uuid.New(), ExtractionRunID: uuid.New(), AssetID: uuid.New(), IndicatorIndex: 0,
		IndicatorID: "hemoglobin", Action: "confirm", ReviewerUserID: uuid.New(), CreatedAt: time.Now().UTC(), IdempotencyKey: "idem-1",
	}
	projected, err := strictReviewRecord(record)
	if err != nil {
		t.Fatalf("project review record: %v", err)
	}
	payload, err := json.Marshal(projected)
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	for _, forbidden := range []string{"reviewer_user_id", "storage_backend", "storage_key", "ocr_result", "raw_text"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("public review record leaked %q: %s", forbidden, text)
		}
	}
	if !strings.Contains(text, `"upload_id"`) {
		t.Fatalf("review record wire must keep the documented upload_id identity: %s", text)
	}
}

func TestUploadGeneratedRoutesRequireAuthenticatedUser(t *testing.T) {
	r := uploadTestRouter(t, &fakeUploadApplication{}, fakeHealthDocumentReviewApplication{}, false)
	for _, path := range []string{
		"/api/v1/uploads",
		"/api/v1/uploads/posture-analysis",
		"/api/v1/uploads/11111111-1111-4111-8111-111111111111",
		"/api/v1/uploads/11111111-1111-4111-8111-111111111111/health-document-review",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("GET %s status=%d want=401 body=%s", path, rec.Code, rec.Body.String())
		}
	}
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/assets/11111111-1111-4111-8111-111111111111", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("DELETE /api/v1/assets status=%d want=401", rec.Code)
	}
}

func TestUploadGeneratedRouteRejectsMalformedUUIDBeforeApplication(t *testing.T) {
	r := uploadTestRouter(t, &fakeUploadApplication{}, fakeHealthDocumentReviewApplication{}, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/uploads/not-a-uuid", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=400 body=%s", rec.Code, rec.Body.String())
	}
}

func TestCreateUploadMultipartBoundaryDelegatesFileHeaderAndType(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	app := &fakeUploadApplication{uploadFile: &model.UserUploadView{
		ID: uuid.New(), UserID: uuid.New(), FileType: "consultation_photo", OriginalName: "photo.png",
		FileSize: 3, MimeType: "image/png", OCRStatus: "pending", AnalysisStatus: "none", CreatedAt: now, UpdatedAt: now,
	}}
	r := uploadTestRouter(t, app, fakeHealthDocumentReviewApplication{}, true)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "photo.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("png")); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("file_type", "consultation_photo"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d want=201 body=%s", rec.Code, rec.Body.String())
	}
	if app.seenFile == nil || app.seenFile.Filename != "photo.png" || app.seenType != "consultation_photo" {
		t.Fatalf("multipart adapter did not delegate expected file/type: file=%#v type=%q", app.seenFile, app.seenType)
	}
}

func TestHealthDocumentSourcePreservesStoredMimeAndNosniff(t *testing.T) {
	body := io.NopCloser(strings.NewReader("pdf-bytes"))
	response := healthDocumentSourceResponse{body: body, length: 9, mime: "application/pdf"}
	rec := httptest.NewRecorder()
	if err := response.VisitGetHealthDocumentSourceResponse(rec); err != nil {
		t.Fatalf("stream source: %v", err)
	}
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/pdf" {
		t.Fatalf("status/content-type=%d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("nosniff missing: %#v", rec.Header())
	}
	if rec.Body.String() != "pdf-bytes" {
		t.Fatalf("body=%q", rec.Body.String())
	}
}

// TestGetUploadForeignAssetIsOwnerMaskedNotFound pins the AS-001 acceptance:
// a foreign upload must answer 404 exactly like a missing one, never 403.
func TestGetUploadForeignAssetIsOwnerMaskedNotFound(t *testing.T) {
	r := uploadTestRouter(t, &fakeUploadApplication{getErr: service.ErrUploadNotFound}, fakeHealthDocumentReviewApplication{}, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/uploads/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("foreign upload status=%d want=404 body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "Forbidden") || strings.Contains(rec.Body.String(), "forbidden") {
		t.Fatalf("foreign upload leaked ownership through status text: %s", rec.Body.String())
	}
}

// TestDeleteAssetRemovalLifecycle pins the source-removal command surface:
// foreign 404, idempotent success with the surviving availability state, and
// the legacy /uploads delete alias returning the same projection.
func TestDeleteAssetRemovalLifecycle(t *testing.T) {
	assetID := uuid.New()
	removedAt := time.Now().UTC()

	app := &fakeUploadApplication{removeErr: service.ErrUploadNotFound}
	r := uploadTestRouter(t, app, fakeHealthDocumentReviewApplication{}, true)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/assets/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("foreign asset removal status=%d want=404 body=%s", rec.Code, rec.Body.String())
	}

	app = &fakeUploadApplication{removeAsset: &model.UserAsset{
		ID: assetID, Availability: model.UserAssetSourceRemoved, SourceRemovedAt: &removedAt,
	}}
	r = uploadTestRouter(t, app, fakeHealthDocumentReviewApplication{}, true)
	uri := "/api/v1/assets/" + assetID.String()
	for attempt := 0; attempt < 2; attempt++ {
		req := httptest.NewRequest(http.MethodDelete, uri, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("removal attempt %d status=%d want=200 body=%s", attempt, rec.Code, rec.Body.String())
		}
		var payload struct {
			Availability    string     `json:"availability"`
			SourceRemovedAt *time.Time `json:"source_removed_at"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode removal response: %v", err)
		}
		if payload.Availability != "source_removed" {
			t.Fatalf("availability=%q want=source_removed body=%s", payload.Availability, rec.Body.String())
		}
		if payload.SourceRemovedAt == nil {
			t.Fatal("source_removed_at missing from removal response")
		}
	}
	if app.removeCalls != 2 {
		t.Fatalf("idempotent replay must reach the command twice, got %d", app.removeCalls)
	}

	// The legacy /uploads delete alias returns the same removal projection.
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/uploads/"+assetID.String(), nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"source_removed"`) {
		t.Fatalf("legacy delete alias status=%d body=%s", rec.Code, rec.Body.String())
	}
}

// TestGetHealthDocumentSourceFailsClosedAfterSourceRemoval pins CD-D4: the
// original is not streamable anymore, and the response names the cause
// instead of pretending the file was merely misplaced.
func TestGetHealthDocumentSourceFailsClosedAfterSourceRemoval(t *testing.T) {
	app := &fakeUploadApplication{openErr: service.ErrUploadSourceUnavailable}
	r := uploadTestRouter(t, app, fakeHealthDocumentReviewApplication{}, true)
	uri := "/api/v1/uploads/" + uuid.New().String() + "/extractions/" + uuid.New().String() + "/source"
	req := httptest.NewRequest(http.MethodGet, uri, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("removed source status=%d want=404 body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "SOURCE_UNAVAILABLE") {
		t.Fatalf("response must name the source-unavailable cause: %s", rec.Body.String())
	}
}

// TestHealthDocumentReviewContextMarksSourceUnavailable pins the AS-001
// acceptance: after source removal the evidence stays readable and is marked
// source_unavailable (never silently downgraded nor revoked).
func TestHealthDocumentReviewContextMarksSourceUnavailable(t *testing.T) {
	assetID := uuid.New()
	runID := uuid.New()
	reviews := fakeHealthDocumentReviewApplication{contextValue: &service.HealthDocumentReviewContext{
		ExtractionRunID:    runID,
		AssetID:            assetID,
		SourceAvailability: service.SourceAvailability(model.UserAssetSourceRemoved),
		ReviewCandidates:   []service.DocumentIndicatorReviewProjection{},
	}}
	r := uploadTestRouter(t, &fakeUploadApplication{}, reviews, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/uploads/"+assetID.String()+"/health-document-review", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("review context status=%d want=200 body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"source_availability":"source_unavailable"`) {
		t.Fatalf("review context must carry the source_unavailable marker: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"extraction_run_id":"`+runID.String()+`"`) {
		t.Fatalf("review context lost the extraction run identity: %s", rec.Body.String())
	}
}

// TestReviewTargetGuardIsOwnerMasked pins that probing a foreign extraction
// run (or binding one to the wrong asset) cannot reveal existence.
func TestReviewTargetGuardIsOwnerMasked(t *testing.T) {
	for _, tc := range []struct {
		name      string
		ensureErr error
	}{
		{"foreign run", service.ErrReviewAccessDenied},
		{"wrong asset binding", errors.Join(service.ErrReviewCandidateMismatch, errors.New("binding"))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reviews := fakeHealthDocumentReviewApplication{ensureErr: tc.ensureErr}
			r := uploadTestRouter(t, &fakeUploadApplication{}, reviews, true)
			uri := "/api/v1/uploads/" + uuid.New().String() + "/extractions/" + uuid.New().String() + "/reviews"
			req := httptest.NewRequest(http.MethodGet, uri, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("status=%d want=404 body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}
