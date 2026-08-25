package handler

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

func TestUploadResponsePreservesFilePathCompatibilityWithoutExposingStorageAuthority(t *testing.T) {
	upload := model.UserUpload{
		ID: uuid.New(), UserID: uuid.New(), FileType: "consultation_photo", OriginalName: "photo.png",
		StorageBackend: "oss", StorageKey: "user/upload/original.png", FileSize: 10, MimeType: "image/png",
	}
	payload, err := json.Marshal(newUploadResponse(upload))
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	if !strings.Contains(text, `"file_path":"user/upload/original.png"`) {
		t.Fatalf("compatibility projection missing: %s", text)
	}
	if strings.Contains(text, "storage_backend") || strings.Contains(text, "storage_key") {
		t.Fatalf("server-private storage authority leaked to HTTP response: %s", text)
	}
}
