package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

func TestAllowedMimeTypes(t *testing.T) {
	tests := []struct {
		mimeType string
		expected bool
	}{
		{"image/jpeg", true},
		{"image/png", true},
		{"image/webp", true},
		{"application/pdf", true},
		{"image/gif", false},
		{"text/plain", false},
		{"application/msword", false},
	}

	for _, tt := range tests {
		t.Run(tt.mimeType, func(t *testing.T) {
			result := allowedMimeTypes[tt.mimeType]
			if result != tt.expected {
				t.Errorf("allowedMimeTypes[%q] = %v, want %v", tt.mimeType, result, tt.expected)
			}
		})
	}
}

func TestAllowedFileTypes(t *testing.T) {
	tests := []struct {
		fileType string
		expected bool
	}{
		{"photo_front", true},
		{"photo_side", true},
		{"photo_back", true},
		{"report", true},
		{"invalid", false},
		{"", false},
		{"PHOTO", false},
	}

	for _, tt := range tests {
		t.Run(tt.fileType, func(t *testing.T) {
			result := allowedFileTypes[tt.fileType]
			if result != tt.expected {
				t.Errorf("allowedFileTypes[%q] = %v, want %v", tt.fileType, result, tt.expected)
			}
		})
	}
}

func TestMaxFileSize(t *testing.T) {
	expected := int64(10 << 20) // 10MB
	if MaxFileSize != expected {
		t.Errorf("MaxFileSize = %d, want %d", MaxFileSize, expected)
	}
}

func TestExecuteOCRCall_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/ocr/extract" {
			http.NotFound(w, r)
			return
		}
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"text":"OCR结果","confidence":0.95}`)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.pdf")
	if err := os.WriteFile(tmpFile, []byte("fake pdf"), 0644); err != nil {
		t.Fatalf("create temp file: %v", err)
	}

	svc := &UploadService{aiServiceURL: server.URL}
	respBody, err := svc.executeOCRCall(tmpFile, "application/pdf")
	if err != nil {
		t.Fatalf("executeOCRCall: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["text"] != "OCR结果" {
		t.Errorf("text = %q, want %q", result["text"], "OCR结果")
	}
}

func TestExecuteOCRCall_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "error")
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.pdf")
	os.WriteFile(tmpFile, []byte("content"), 0644)

	svc := &UploadService{aiServiceURL: server.URL}
	_, err := svc.executeOCRCall(tmpFile, "application/pdf")
	if err == nil {
		t.Error("expected error for 500, got nil")
	}
}

func TestExecuteOCRCall_FileNotFound(t *testing.T) {
	svc := &UploadService{aiServiceURL: "http://localhost:1"}
	_, err := svc.executeOCRCall("/nonexistent.pdf", "application/pdf")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestExecuteOCRCall_ConnectionRefused(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.pdf")
	os.WriteFile(tmpFile, []byte("content"), 0644)

	svc := &UploadService{aiServiceURL: "http://127.0.0.1:1"}
	_, err := svc.executeOCRCall(tmpFile, "application/pdf")
	if err == nil {
		t.Error("expected error for connection refused")
	}
}

func TestIdempotencyKey_Format(t *testing.T) {
	uploadID := "test-123"
	key := fmt.Sprintf("upload_ocr:%s", uploadID)
	if key != "upload_ocr:test-123" {
		t.Errorf("key = %q, want %q", key, "upload_ocr:test-123")
	}
}

func TestParseOCRJobInput(t *testing.T) {
	job := model.Job{
		Input: datatypes.JSON(`{"upload_id":"upload-1","file_path":"uploads/u/report.pdf","mime_type":"application/pdf"}`),
	}

	input, err := parseOCRJobInput(job)
	if err != nil {
		t.Fatalf("parseOCRJobInput: %v", err)
	}
	if input.UploadID != "upload-1" || input.FilePath == "" || input.MimeType != "application/pdf" {
		t.Fatalf("unexpected input: %+v", input)
	}
}

func TestParseOCRJobInputMissingFields(t *testing.T) {
	job := model.Job{Input: datatypes.JSON(`{"upload_id":"upload-1"}`)}
	if _, err := parseOCRJobInput(job); err == nil {
		t.Fatal("expected missing field error")
	}
}

func TestBuildPostureAnalysisSummaryPrefersNewestPerView(t *testing.T) {
	userID := mustParseUUID("11111111-1111-1111-1111-111111111111")
	sideOld := model.UserUpload{
		ID:             mustParseUUID("22222222-2222-2222-2222-222222222221"),
		UserID:         userID,
		FileType:       "photo_side",
		AnalysisStatus: "completed",
		AnalysisResult: json.RawMessage(`{"view":"side","findings":[{"key":"old","label":"旧发现"}],"summary_markdown":"旧摘要"}`),
	}
	sideNew := model.UserUpload{
		ID:             mustParseUUID("22222222-2222-2222-2222-222222222222"),
		UserID:         userID,
		FileType:       "photo_side",
		AnalysisStatus: "completed",
		AnalysisResult: json.RawMessage(`{"view":"side","findings":[{"key":"forward_head","label":"头前移倾向","severity":"mild","evidence":"耳垂前移"}],"summary_markdown":"新摘要"}`),
	}
	// Newest first, matching GetLatestPostureAnalyses order.
	summary := BuildPostureAnalysisSummary([]model.UserUpload{sideNew, sideOld})
	if !summary.HasAnalysis {
		t.Fatal("expected has_analysis")
	}
	if len(summary.Views) != 1 {
		t.Fatalf("expected 1 view after dedupe, got %d", len(summary.Views))
	}
	if summary.Views[0].View != "side" {
		t.Fatalf("expected side view, got %q", summary.Views[0].View)
	}
	if len(summary.Findings) != 1 {
		t.Fatalf("expected 1 finding from newest only, got %d", len(summary.Findings))
	}
	finding, _ := summary.Findings[0].(map[string]any)
	if finding["key"] != "forward_head" {
		t.Fatalf("expected newest finding, got %#v", finding)
	}
	if len(summary.Summaries) != 1 || summary.Summaries[0] != "新摘要" {
		t.Fatalf("unexpected summaries: %#v", summary.Summaries)
	}
}

func TestBuildPostureAnalysisSummaryEmpty(t *testing.T) {
	summary := BuildPostureAnalysisSummary(nil)
	if summary.HasAnalysis {
		t.Fatal("expected no analysis")
	}
	if summary.Views == nil || summary.Findings == nil {
		t.Fatal("expected non-nil empty slices for JSON")
	}
}

func mustParseUUID(s string) uuid.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		panic(err)
	}
	return id
}
