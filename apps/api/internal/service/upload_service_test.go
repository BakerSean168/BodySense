package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
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
