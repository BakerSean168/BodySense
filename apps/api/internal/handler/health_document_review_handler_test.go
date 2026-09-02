package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func reviewHandlerContext(method, path, body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx.Request = req
	return ctx, recorder
}

func TestHealthDocumentReviewHandlerRequiresAuthentication(t *testing.T) {
	h := NewHealthDocumentReviewHandler(nil, nil)
	runID := uuid.New()
	cases := []struct {
		name   string
		method string
		body   string
		fn     func(*gin.Context)
	}{
		{"ListCandidates", http.MethodGet, "", h.ListCandidates},
		{"AppendReview", http.MethodPost, `{}`, h.AppendReview},
		{"SourceContext", http.MethodGet, "", h.SourceContext},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, recorder := reviewHandlerContext(tc.method, "/api/v1/uploads/"+runID.String()+"/extractions/"+runID.String(), tc.body)
			// No user_id in context -> rejected before any service call.
			tc.fn(ctx)
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status=%d want %d body=%s", recorder.Code, http.StatusUnauthorized, recorder.Body.String())
			}
		})
	}
}

func TestHealthDocumentReviewHandlerRejectsMalformedRunID(t *testing.T) {
	userID := uuid.New()
	h := NewHealthDocumentReviewHandler(nil, nil)
	ctx, recorder := reviewHandlerContext(http.MethodGet, "/api/v1/uploads/not-an-id/extractions/not-a-run", "")
	ctx.Set("user_id", userID.String())
	h.ListCandidates(ctx)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400 body=%s", recorder.Code, recorder.Body.String())
	}
}

// The immutable review record projection must never leak storage authority.
func TestDocumentIndicatorReviewRecordNeverLeaksStorageAuthority(t *testing.T) {
	record := service.DocumentIndicatorReviewRecord{
		ReviewID:        uuid.New(),
		ExtractionRunID: uuid.New(),
		UploadID:        uuid.New(),
		IndicatorIndex:  0,
		IndicatorID:     "hemoglobin",
		Action:          "confirm",
		ReviewerUserID:  uuid.New(),
		CreatedAt:       time.Now(),
		IdempotencyKey:  "idem-1",
	}
	payload, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	if strings.Contains(text, "storage_backend") || strings.Contains(text, "storage_key") {
		t.Fatalf("review record leaked storage authority: %s", text)
	}
	if strings.Contains(text, "ocr_result") {
		t.Fatalf("review record leaked raw OCR payload: %s", text)
	}
}
