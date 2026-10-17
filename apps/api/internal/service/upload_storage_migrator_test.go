package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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
	uploads map[uuid.UUID]*model.UserAsset
	fakeUserAssetRepository
	beforeLock      func()
	casError        error
	commitWithError bool
	rejectCAS       bool
}

func (r *fakeUploadMigrationRepo) ListByStorageBackend(_ context.Context, backend string, after *uuid.UUID, limit int) ([]model.UserAsset, error) {
	ids := make([]uuid.UUID, 0, len(r.uploads))
	for id, upload := range r.uploads {
		hasReplica := r.replicas[sourceReplica(upload, backend)]
		if upload.Availability == model.UserAssetAvailable && (upload.StorageBackend == backend || hasReplica) && (after == nil || id.String() > after.String()) {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	if len(ids) > limit {
		ids = ids[:limit]
	}
	result := make([]model.UserAsset, 0, len(ids))
	for _, id := range ids {
		result = append(result, *r.uploads[id])
	}
	return result, nil
}

func (r *fakeUploadMigrationRepo) CompareAndSwapStorageBackend(_ context.Context, id, userID uuid.UUID, fromBackend, storageKey, toBackend string) (bool, error) {
	upload := r.uploads[id]
	if r.casError != nil && !r.commitWithError {
		return false, r.casError
	}
	if r.rejectCAS || upload == nil || upload.Availability != model.UserAssetAvailable || upload.UserID != userID || upload.StorageBackend != fromBackend || upload.StorageKey != storageKey {
		return false, nil
	}
	upload.StorageBackend = toBackend
	return true, r.casError
}

func migrationFixture(t *testing.T, payload []byte) (*uploadstorage.Registry, uploadstorage.Store, uploadstorage.Store, *fakeUploadMigrationRepo, *model.UserAsset) {
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
	upload := &model.UserAsset{
		ID: uuid.New(), UserID: uuid.New(), StorageBackend: "local", Availability: model.UserAssetAvailable,
		StorageKey: "11111111-1111-1111-1111-111111111111/22222222-2222-2222-2222-222222222222/original.png",
		ByteSize:   int64(len(payload)), MimeType: "image/png",
	}
	if err := source.Put(context.Background(), upload.StorageKey, bytes.NewReader(payload), int64(len(payload)), upload.MimeType); err != nil {
		t.Fatal(err)
	}
	repo := &fakeUploadMigrationRepo{uploads: map[uuid.UUID]*model.UserAsset{upload.ID: upload}}
	repo.byID = repo.uploads
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
	if exists, _ := source.Exists(context.Background(), upload.StorageKey); exists {
		t.Fatal("old source object must be removed after cutover")
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
	if exists, _ := target.Exists(context.Background(), upload.StorageKey); !exists {
		t.Fatal("conflicting pre-existing target must remain available for investigation")
	}
	if !repo.replicas[sourceReplica(upload, "oss")] {
		t.Fatal("copy failure lost target inventory")
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
	upload := &model.UserAsset{
		ID: uuid.New(), UserID: uuid.New(), StorageBackend: "local", Availability: model.UserAssetAvailable,
		StorageKey: "11111111-1111-1111-1111-111111111111/22222222-2222-2222-2222-222222222222/original.png",
		ByteSize:   7, MimeType: "image/png",
	}
	repo := &fakeUploadMigrationRepo{uploads: map[uuid.UUID]*model.UserAsset{upload.ID: upload}}
	result, err := NewUploadStorageMigrator(repo, registry).Migrate(ctx, "local", "oss", true)
	if err != nil {
		t.Fatalf("dry run should not require target provisioning: %v", err)
	}
	if result.Scanned != 1 || result.Migrated != 0 || repo.uploads[upload.ID].StorageBackend != "local" {
		t.Fatalf("unexpected dry-run result=%+v upload=%+v", result, repo.uploads[upload.ID])
	}
}

func (r *fakeUploadMigrationRepo) GetOwnedByID(_ context.Context, id, userID uuid.UUID) (*model.UserAsset, error) {
	asset := r.uploads[id]
	if asset == nil || asset.UserID != userID {
		return nil, nil
	}
	copy := *asset
	return &copy, nil
}
func (r *fakeUploadMigrationRepo) WithStorageLock(ctx context.Context, _ uuid.UUID, fn func(context.Context) error) error {
	if r.beforeLock != nil {
		r.beforeLock()
		r.beforeLock = nil
	}
	return fn(ctx)
}

func TestMigrationThenRemovalDeletesEverySourceReplica(t *testing.T) {
	registry, source, target, repo, asset := migrationFixture(t, []byte("private-upload"))
	if _, err := NewUploadStorageMigrator(repo, registry).Migrate(context.Background(), "local", "oss", false); err != nil {
		t.Fatal(err)
	}
	svc := NewUploadService(&repo.fakeUserAssetRepository, nil, nil, nil, nil, registry)
	removed, err := svc.RemoveSource(context.Background(), asset.UserID, asset.ID)
	if err != nil || removed.Availability != model.UserAssetSourceRemoved {
		t.Fatalf("removed=%v err=%v", removed, err)
	}
	for _, store := range []uploadstorage.Store{source, target} {
		if exists, err := store.Exists(context.Background(), asset.StorageKey); err != nil || exists {
			t.Fatalf("replica remains: %s exists=%v err=%v", store.Backend(), exists, err)
		}
	}
	if len(repo.replicas) != 0 {
		t.Fatal("cleanup inventory must be empty after verified removal")
	}
}

func TestRemovalThenMigrationCannotRecreateSource(t *testing.T) {
	registry, _, target, repo, asset := migrationFixture(t, []byte("private-upload"))
	svc := NewUploadService(&repo.fakeUserAssetRepository, nil, nil, nil, nil, registry)
	if _, err := svc.RemoveSource(context.Background(), asset.UserID, asset.ID); err != nil {
		t.Fatal(err)
	}
	result, err := NewUploadStorageMigrator(repo, registry).Migrate(context.Background(), "local", "oss", false)
	if err != nil || result.Scanned != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if exists, _ := target.Exists(context.Background(), asset.StorageKey); exists {
		t.Fatal("removed source was recreated")
	}
}

func TestMigrationRechecksAvailabilityAfterAcquiringLock(t *testing.T) {
	registry, _, target, repo, asset := migrationFixture(t, []byte("private-upload"))
	repo.beforeLock = func() { asset.Availability = model.UserAssetSourceRemoved }
	if _, err := NewUploadStorageMigrator(repo, registry).Migrate(context.Background(), "local", "oss", false); err == nil {
		t.Fatal("stale list must be rejected")
	}
	if exists, _ := target.Exists(context.Background(), asset.StorageKey); exists {
		t.Fatal("copied after removal won the lock")
	}
}

func TestMigrationCASFailureCleansTarget(t *testing.T) {
	registry, source, target, repo, asset := migrationFixture(t, []byte("private-upload"))
	repo.rejectCAS = true
	if _, err := NewUploadStorageMigrator(repo, registry).Migrate(context.Background(), "local", "oss", false); err == nil {
		t.Fatal("expected CAS failure")
	}
	if exists, _ := target.Exists(context.Background(), asset.StorageKey); exists {
		t.Fatal("orphan target remains")
	}
	if exists, _ := source.Exists(context.Background(), asset.StorageKey); !exists {
		t.Fatal("authoritative source deleted")
	}
	if repo.replicas[sourceReplica(asset, "oss")] {
		t.Fatal("verified absent target remains in inventory")
	}
}

type failingDeleteStore struct {
	uploadstorage.Store
	failures int
	pretend  bool
}

func (s *failingDeleteStore) Delete(ctx context.Context, key string) error {
	if s.failures > 0 {
		s.failures--
		return errors.New("transient delete failure")
	}
	if s.pretend {
		return nil
	}
	return s.Store.Delete(ctx, key)
}

func TestMigrationCleanupRetriesAfterCommittedCutover(t *testing.T) {
	registry, source, target, repo, asset := migrationFixture(t, []byte("private-upload"))
	failing := &failingDeleteStore{Store: source, failures: 1}
	registry, err := uploadstorage.NewRegistryFromStores("local", failing, target)
	if err != nil {
		t.Fatal(err)
	}
	migrator := NewUploadStorageMigrator(repo, registry)
	result, err := migrator.Migrate(context.Background(), "local", "oss", false)
	if err == nil || result.Migrated != 1 || asset.StorageBackend != "oss" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if !repo.replicas[sourceReplica(asset, "local")] {
		t.Fatal("failed cleanup lost durable intent")
	}
	if _, err := migrator.Migrate(context.Background(), "local", "oss", false); err != nil {
		t.Fatal(err)
	}
	if exists, _ := source.Exists(context.Background(), asset.StorageKey); exists {
		t.Fatal("old replica remains after retry")
	}
}

func TestRemovalRetriesKnownReplicasAndVerifiesAbsence(t *testing.T) {
	for _, pretend := range []bool{false, true} {
		t.Run(fmt.Sprint(pretend), func(t *testing.T) {
			_, source, target, repo, asset := migrationFixture(t, []byte("private-upload"))
			failing := &failingDeleteStore{Store: source, failures: 1, pretend: pretend}
			registry, err := uploadstorage.NewRegistryFromStores("local", failing, target)
			if err != nil {
				t.Fatal(err)
			}
			svc := NewUploadService(&repo.fakeUserAssetRepository, nil, nil, nil, nil, registry)
			if _, err := svc.RemoveSource(context.Background(), asset.UserID, asset.ID); err == nil {
				t.Fatal("must report failed cleanup")
			}
			if len(repo.replicas) != 1 {
				t.Fatal("lost retry inventory")
			}
			if pretend {
				if _, err := svc.RemoveSource(context.Background(), asset.UserID, asset.ID); err == nil {
					t.Fatal("Delete success alone is not verified absence")
				}
				failing.pretend = false
			}
			if _, err := svc.RemoveSource(context.Background(), asset.UserID, asset.ID); err != nil {
				t.Fatal(err)
			}
			if len(repo.replicas) != 0 {
				t.Fatal("cleanup remains after successful retry")
			}
		})
	}
}

func TestRemovalRetriesBothReplicasAfterCutoverCleanupFailure(t *testing.T) {
	_, source, target, repo, asset := migrationFixture(t, []byte("private-upload"))
	failing := &failingDeleteStore{Store: source, failures: 2}
	registry, err := uploadstorage.NewRegistryFromStores("local", failing, target)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewUploadStorageMigrator(repo, registry).Migrate(context.Background(), "local", "oss", false); err == nil {
		t.Fatal("expected old-source cleanup failure")
	}
	if len(repo.replicas) != 2 {
		t.Fatal("must track both old and new source replicas")
	}
	svc := NewUploadService(&repo.fakeUserAssetRepository, nil, nil, nil, nil, registry)
	if _, err := svc.RemoveSource(context.Background(), asset.UserID, asset.ID); err == nil {
		t.Fatal("must report uncleared old replica")
	}
	if exists, _ := target.Exists(context.Background(), asset.StorageKey); exists {
		t.Fatal("new replica must be cleaned even when old replica Delete fails")
	}
	if len(repo.replicas) != 1 {
		t.Fatal("only failed replica should remain in inventory")
	}
	if _, err := svc.RemoveSource(context.Background(), asset.UserID, asset.ID); err != nil {
		t.Fatal(err)
	}
	if exists, _ := source.Exists(context.Background(), asset.StorageKey); exists {
		t.Fatal("old replica remains after retry")
	}
}

func TestMigrationCASFailureRetainsFailedTargetCleanupForRetry(t *testing.T) {
	_, source, target, repo, asset := migrationFixture(t, []byte("private-upload"))
	failing := &failingDeleteStore{Store: target, failures: 1}
	registry, err := uploadstorage.NewRegistryFromStores("local", source, failing)
	if err != nil {
		t.Fatal(err)
	}
	repo.rejectCAS = true
	migrator := NewUploadStorageMigrator(repo, registry)
	if _, err := migrator.Migrate(context.Background(), "local", "oss", false); err == nil {
		t.Fatal("expected CAS and cleanup failure")
	}
	if !repo.replicas[sourceReplica(asset, "oss")] {
		t.Fatal("orphan target lost from inventory")
	}
	if _, err := migrator.Migrate(context.Background(), "local", "oss", false); err == nil {
		t.Fatal("CAS still rejected")
	}
	if exists, _ := target.Exists(context.Background(), asset.StorageKey); exists {
		t.Fatal("orphan target remains after cleanup retry")
	}
}

func TestMigrationResolvesCASWriteErrorsWithoutDeletingAuthoritativeTarget(t *testing.T) {
	for _, committed := range []bool{false, true} {
		t.Run(fmt.Sprint(committed), func(t *testing.T) {
			registry, source, target, repo, asset := migrationFixture(t, []byte("private-upload"))
			repo.casError = errors.New("database write outcome error")
			repo.commitWithError = committed
			migrator := NewUploadStorageMigrator(repo, registry)
			if _, err := migrator.Migrate(context.Background(), "local", "oss", false); err == nil {
				t.Fatal("must report DB error")
			}
			if exists, _ := target.Exists(context.Background(), asset.StorageKey); exists != committed {
				t.Fatalf("target exists=%v committed=%v", exists, committed)
			}
			if exists, _ := source.Exists(context.Background(), asset.StorageKey); !exists {
				t.Fatal("old source deleted before outcome resolved")
			}
			repo.casError = nil
			if _, err := migrator.Migrate(context.Background(), "local", "oss", false); err != nil {
				t.Fatal(err)
			}
			if exists, _ := source.Exists(context.Background(), asset.StorageKey); exists {
				t.Fatal("old source remains after retry")
			}
		})
	}
}

func TestRemovalOfErasedAssetStillVerifiesKnownSourceCleanup(t *testing.T) {
	registry, source, _, repo, asset := migrationFixture(t, []byte("private-upload"))
	asset.Availability = model.UserAssetErased
	svc := NewUploadService(&repo.fakeUserAssetRepository, nil, nil, nil, nil, registry)
	result, err := svc.RemoveSource(context.Background(), asset.UserID, asset.ID)
	if err != nil || result.Availability != model.UserAssetErased {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if exists, _ := source.Exists(context.Background(), asset.StorageKey); exists {
		t.Fatal("erased marker must not hide a known remaining source replica")
	}
	if len(repo.removed) != 0 {
		t.Fatal("erasure state must not transition through source removal")
	}
}
