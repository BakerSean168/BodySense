package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
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
	"github.com/bodysense/api/internal/uploadstorage"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeUploadApplication struct {
	uploadFile *model.UserUpload
	uploads    []model.UserUpload
	one        *model.UserUpload
	posture    service.PostureAnalysisSummary
	stream     string
	mime       string
	seenFile   *multipart.FileHeader
	seenType   string
}

func (f *fakeUploadApplication) UploadFile(_ context.Context, _ uuid.UUID, file *multipart.FileHeader, fileType string) (*model.UserUpload, error) {
	f.seenFile, f.seenType = file, fileType
	return f.uploadFile, nil
}
func (f *fakeUploadApplication) GetUploads(context.Context, uuid.UUID) ([]model.UserUpload, error) {
	return f.uploads, nil
}
func (f *fakeUploadApplication) GetUpload(context.Context, uuid.UUID, uuid.UUID) (*model.UserUpload, error) {
	return f.one, nil
}
func (f *fakeUploadApplication) GetPostureAnalysisSummary(context.Context, uuid.UUID) (service.PostureAnalysisSummary, error) {
	return f.posture, nil
}
func (f *fakeUploadApplication) DeleteUpload(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (f *fakeUploadApplication) OpenUploadObject(context.Context, *model.UserUpload) (io.ReadCloser, uploadstorage.ObjectInfo, error) {
	return io.NopCloser(strings.NewReader(f.stream)), uploadstorage.ObjectInfo{Size: int64(len(f.stream))}, nil
}

type fakeHealthDocumentReviewApplication struct{}

func (fakeHealthDocumentReviewApplication) CurrentContext(context.Context, uuid.UUID, uuid.UUID) (*service.HealthDocumentReviewContext, error) {
	return nil, service.ErrReviewContextUnavailable
}
func (fakeHealthDocumentReviewApplication) EnsureUploadOwnsRun(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return nil
}
func (fakeHealthDocumentReviewApplication) ListCandidates(context.Context, uuid.UUID, uuid.UUID) ([]service.DocumentIndicatorReviewProjection, error) {
	return []service.DocumentIndicatorReviewProjection{}, nil
}
func (fakeHealthDocumentReviewApplication) ApplyReview(context.Context, uuid.UUID, service.ReviewRequest) (*service.DocumentIndicatorReviewRecord, error) {
	return nil, nil
}

func uploadTestRouter(t *testing.T, uploads uploadApplication, reviews healthDocumentReviewApplication, authenticated bool) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	spec, err := openapiv1.GetSwagger()
	if err != nil {
		t.Fatalf("load generated OpenAPI spec: %v", err)
	}
	r := gin.New()
	RegisterRoutes(r, StrictHandler(NewPublicServer(nil).WithUploads(uploads, reviews)), RouteSecurity{
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
	upload := &model.UserUpload{
		ID: uuid.New(), UserID: uuid.New(), FileType: "consultation_photo", OriginalName: "photo.png",
		StorageBackend: "oss", StorageKey: "private/user/upload/original.png", FileSize: 10, MimeType: "image/png",
		OCRStatus: "pending", AnalysisStatus: "none", AgentConfigurationID: "posture-v3", CreatedAt: now, UpdatedAt: now,
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
	for _, forbidden := range []string{"user_id", "file_path", "storage_backend", "storage_key", "agent_configuration_id", upload.StorageKey} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("public upload leaked %q: %s", forbidden, text)
		}
	}
}

func TestReviewPublicProjectionDoesNotExposeReviewerOrRawStorageAuthority(t *testing.T) {
	record := &service.DocumentIndicatorReviewRecord{
		ReviewID: uuid.New(), ExtractionRunID: uuid.New(), UploadID: uuid.New(), IndicatorIndex: 0,
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
	app := &fakeUploadApplication{uploadFile: &model.UserUpload{
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
