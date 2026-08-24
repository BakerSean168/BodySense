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

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/uploadstorage"
	"github.com/google/uuid"
)

type fakeUploadRepository struct {
	created   *model.UserUpload
	createErr error
	byID      map[uuid.UUID]*model.UserUpload
	deleted   []uuid.UUID
}

func (r *fakeUploadRepository) Create(_ context.Context, upload *model.UserUpload) error {
	if r.createErr != nil {
		return r.createErr
	}
	copy := *upload
	r.created = &copy
	if r.byID == nil {
		r.byID = map[uuid.UUID]*model.UserUpload{}
	}
	r.byID[upload.ID] = &copy
	return nil
}
func (r *fakeUploadRepository) GetByID(_ context.Context, id uuid.UUID) (*model.UserUpload, error) {
	return r.byID[id], nil
}
func (r *fakeUploadRepository) GetByUserID(_ context.Context, userID uuid.UUID) ([]model.UserUpload, error) {
	var result []model.UserUpload
	for _, upload := range r.byID {
		if upload.UserID == userID {
			result = append(result, *upload)
		}
	}
	return result, nil
}
func (r *fakeUploadRepository) Delete(_ context.Context, id, userID uuid.UUID) error {
	upload := r.byID[id]
	if upload == nil || upload.UserID != userID {
		return nil
	}
	delete(r.byID, id)
	r.deleted = append(r.deleted, id)
	return nil
}
func (*fakeUploadRepository) UpdateOCRResult(context.Context, uuid.UUID, uuid.UUID, string, json.RawMessage) error {
	return nil
}
func (*fakeUploadRepository) UpdateOCRStatus(context.Context, uuid.UUID, uuid.UUID, string) error {
	return nil
}
func (*fakeUploadRepository) UpdateAnalysisStatus(context.Context, uuid.UUID, uuid.UUID, string) error {
	return nil
}
func (*fakeUploadRepository) UpdateAnalysisResult(context.Context, uuid.UUID, uuid.UUID, string, json.RawMessage) error {
	return nil
}
func (*fakeUploadRepository) UpdateAgentConfiguration(context.Context, uuid.UUID, string) error {
	return nil
}
func (*fakeUploadRepository) GetLatestPostureAnalyses(context.Context, uuid.UUID) ([]model.UserUpload, error) {
	return nil, nil
}

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

func TestUploadStorageVerticalWriteReadDelete(t *testing.T) {
	ctx := context.Background()
	registry, _ := testUploadRegistry(t)
	repo := &fakeUploadRepository{byID: map[uuid.UUID]*model.UserUpload{}}
	svc := NewUploadService(repo, nil, nil, registry)
	userID := uuid.New()
	payload := append([]byte("\x89PNG\r\n\x1a\n"), []byte("private-image-payload")...)
	file := multipartUploadHeader(t, "posture.png", "image/png", payload)

	upload, err := svc.UploadFile(ctx, userID, file, "consultation_photo")
	if err != nil {
		t.Fatalf("UploadFile: %v", err)
	}
	if upload.StorageBackend != "local" || !strings.HasPrefix(upload.StorageKey, userID.String()+"/") || !strings.HasSuffix(upload.StorageKey, "/original.png") {
		t.Fatalf("unexpected storage identity: %#v", upload)
	}
	if repo.created == nil || repo.created.StorageKey != upload.StorageKey {
		t.Fatal("DB manifest did not receive committed storage identity")
	}
	if _, err := registry.DefaultStore().Stat(ctx, upload.StorageKey); err != nil {
		t.Fatalf("stored object is missing: %v", err)
	}
	dataURL, mime, err := svc.ReadImageDataURL(ctx, userID, upload.ID)
	if err != nil {
		t.Fatalf("ReadImageDataURL: %v", err)
	}
	if mime != "image/png" || !strings.HasPrefix(dataURL, "data:image/png;base64,") {
		t.Fatalf("unexpected data URL: mime=%q url=%q", mime, dataURL)
	}
	if err := svc.DeleteUpload(ctx, userID, upload.ID); err != nil {
		t.Fatalf("DeleteUpload: %v", err)
	}
	if _, err := registry.DefaultStore().Stat(ctx, upload.StorageKey); err == nil {
		t.Fatal("storage object survived delete")
	}
	if len(repo.deleted) != 1 || repo.byID[upload.ID] != nil {
		t.Fatal("DB manifest was not deleted after object deletion")
	}
}

func TestUploadStorageDBFailureRollsBackObject(t *testing.T) {
	registry, root := testUploadRegistry(t)
	repo := &fakeUploadRepository{createErr: errors.New("db unavailable")}
	svc := NewUploadService(repo, nil, nil, registry)
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
	repo := &fakeUploadRepository{byID: map[uuid.UUID]*model.UserUpload{}}
	svc := NewUploadService(repo, nil, nil, registry)
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
	if files != 0 || repo.created != nil {
		t.Fatalf("MIME mismatch crossed storage/DB boundary files=%d created=%v", files, repo.created != nil)
	}
}
