package service

import (
	"bytes"
	"context"
	"sort"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/uploadstorage"
	"github.com/google/uuid"
)

type backendAliasStore struct {
	uploadstorage.Store
	backend string
}

func (s backendAliasStore) Backend() string { return s.backend }

type fakeUploadMigrationRepo struct {
	uploads map[uuid.UUID]*model.UserUpload
}

func (r *fakeUploadMigrationRepo) ListByStorageBackend(_ context.Context, backend string, after *uuid.UUID, limit int) ([]model.UserUpload, error) {
	ids := make([]uuid.UUID, 0, len(r.uploads))
	for id, upload := range r.uploads {
		if upload.StorageBackend == backend && (after == nil || id.String() > after.String()) {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	if len(ids) > limit {
		ids = ids[:limit]
	}
	result := make([]model.UserUpload, 0, len(ids))
	for _, id := range ids {
		result = append(result, *r.uploads[id])
	}
	return result, nil
}

func (r *fakeUploadMigrationRepo) CompareAndSwapStorageBackend(_ context.Context, id, userID uuid.UUID, fromBackend, storageKey, toBackend string) (bool, error) {
	upload := r.uploads[id]
	if upload == nil || upload.UserID != userID || upload.StorageBackend != fromBackend || upload.StorageKey != storageKey {
		return false, nil
	}
	upload.StorageBackend = toBackend
	return true, nil
}

func migrationFixture(t *testing.T, payload []byte) (*uploadstorage.Registry, uploadstorage.Store, uploadstorage.Store, *fakeUploadMigrationRepo, *model.UserUpload) {
	t.Helper()
	source, err := uploadstorage.NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	targetLocal, err := uploadstorage.NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	target := backendAliasStore{Store: targetLocal, backend: "oss"}
	registry, err := uploadstorage.NewRegistryFromStores("local", source, target)
	if err != nil {
		t.Fatal(err)
	}
	upload := &model.UserUpload{
		ID: uuid.New(), UserID: uuid.New(), StorageBackend: "local",
		StorageKey: "11111111-1111-1111-1111-111111111111/22222222-2222-2222-2222-222222222222/original.png",
		FileSize:   int64(len(payload)), MimeType: "image/png",
	}
	if err := source.Put(context.Background(), upload.StorageKey, bytes.NewReader(payload), int64(len(payload)), upload.MimeType); err != nil {
		t.Fatal(err)
	}
	repo := &fakeUploadMigrationRepo{uploads: map[uuid.UUID]*model.UserUpload{upload.ID: upload}}
	return registry, source, target, repo, upload
}

func TestUploadStorageMigratorCopiesVerifiesThenAdvancesManifest(t *testing.T) {
	payload := []byte("private-upload")
	registry, source, target, repo, upload := migrationFixture(t, payload)
	result, err := NewUploadStorageMigrator(repo, registry).Migrate(context.Background(), "local", "oss", false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Migrated != 1 || repo.uploads[upload.ID].StorageBackend != "oss" {
		t.Fatalf("result=%+v manifest=%+v", result, repo.uploads[upload.ID])
	}
	if exists, _ := source.Exists(context.Background(), upload.StorageKey); !exists {
		t.Fatal("source object must remain until post-cutover cleanup")
	}
	if exists, _ := target.Exists(context.Background(), upload.StorageKey); !exists {
		t.Fatal("target object missing")
	}
}

func TestUploadStorageMigratorAcceptsAlreadyVerifiedTarget(t *testing.T) {
	payload := []byte("private-upload")
	registry, _, target, repo, upload := migrationFixture(t, payload)
	if err := target.Put(context.Background(), upload.StorageKey, bytes.NewReader(payload), int64(len(payload)), upload.MimeType); err != nil {
		t.Fatal(err)
	}
	result, err := NewUploadStorageMigrator(repo, registry).Migrate(context.Background(), "local", "oss", false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Migrated != 1 || result.AlreadyVerified != 1 || repo.uploads[upload.ID].StorageBackend != "oss" {
		t.Fatalf("unexpected idempotent migration result: %+v", result)
	}
}

func TestUploadStorageMigratorRejectsConflictingTarget(t *testing.T) {
	payload := []byte("private-upload")
	registry, _, target, repo, upload := migrationFixture(t, payload)
	conflict := []byte("different-data")
	if len(conflict) != len(payload) {
		t.Fatal("fixture requires same-size conflicting payload")
	}
	if err := target.Put(context.Background(), upload.StorageKey, bytes.NewReader(conflict), int64(len(conflict)), upload.MimeType); err != nil {
		t.Fatal(err)
	}
	if _, err := NewUploadStorageMigrator(repo, registry).Migrate(context.Background(), "local", "oss", false); err == nil {
		t.Fatal("conflicting target object must fail closed")
	}
	if repo.uploads[upload.ID].StorageBackend != "local" {
		t.Fatal("DB manifest advanced despite target checksum mismatch")
	}
}

func TestUploadStorageMigratorDryRunDoesNotRequireTargetProvisioning(t *testing.T) {
	ctx := context.Background()
	source, err := uploadstorage.NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	registry, err := uploadstorage.NewRegistryFromStores("local", source)
	if err != nil {
		t.Fatal(err)
	}
	upload := &model.UserUpload{
		ID: uuid.New(), UserID: uuid.New(), StorageBackend: "local",
		StorageKey: "11111111-1111-1111-1111-111111111111/22222222-2222-2222-2222-222222222222/original.png",
		FileSize:   7, MimeType: "image/png",
	}
	repo := &fakeUploadMigrationRepo{uploads: map[uuid.UUID]*model.UserUpload{upload.ID: upload}}
	result, err := NewUploadStorageMigrator(repo, registry).Migrate(ctx, "local", "oss", true)
	if err != nil {
		t.Fatalf("dry run should not require target provisioning: %v", err)
	}
	if result.Scanned != 1 || result.Migrated != 0 || repo.uploads[upload.ID].StorageBackend != "local" {
		t.Fatalf("unexpected dry-run result=%+v upload=%+v", result, repo.uploads[upload.ID])
	}
}
