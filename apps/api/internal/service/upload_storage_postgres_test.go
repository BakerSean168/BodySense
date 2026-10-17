package service

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/uploadstorage"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type pausedMigrationStore struct {
	uploadstorage.Store
	started chan struct{}
	resume  chan struct{}
}

func (s *pausedMigrationStore) Put(ctx context.Context, key string, reader io.Reader, size int64, mime string) error {
	close(s.started)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.resume:
	}
	return s.Store.Put(ctx, key, reader, size, mime)
}

func TestUserAssetStorageMigrationRemovalPostgres(t *testing.T) {
	dsn := os.Getenv("BODYSENSE_INTEGRATION_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires isolated PostgreSQL 18")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var version int
	if err := db.Raw("SHOW server_version_num").Scan(&version).Error; err != nil || version < 180000 {
		t.Fatalf("PG18 required: version=%d err=%v", version, err)
	}
	owner := uuid.New()
	if err := db.Exec("INSERT INTO users(id,email,password_hash) VALUES(?,?,?)", owner, "as-storage-"+owner.String()+"@example.test", "synthetic").Error; err != nil {
		t.Fatal(err)
	}
	defer db.Exec("DELETE FROM users WHERE id=?", owner)
	repo := repository.NewUserAssetRepository(db)
	payload := []byte("synthetic-private-source")
	asset := &model.UserAsset{ID: uuid.New(), UserID: owner, Kind: model.UserAssetKindReport, OriginalName: "report.pdf", MimeType: "application/pdf", ByteSize: int64(len(payload)), ContentSHA256: strings.Repeat("a", 64), StorageBackend: "local", Availability: model.UserAssetAvailable}
	asset.StorageKey = owner.String() + "/" + asset.ID.String() + "/original.pdf"
	if err := repo.Create(context.Background(), asset); err != nil {
		t.Fatal(err)
	}
	source, err := uploadstorage.NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	localTarget, err := uploadstorage.NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	target := &pausedMigrationStore{Store: backendAliasStore{Store: localTarget, backend: "oss"}, started: make(chan struct{}), resume: make(chan struct{})}
	registry, err := uploadstorage.NewRegistryFromStores("local", source, target)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := source.Put(ctx, asset.StorageKey, bytes.NewReader(payload), int64(len(payload)), asset.MimeType); err != nil {
		t.Fatal(err)
	}
	migrated := make(chan error, 1)
	go func() {
		_, err := NewUploadStorageMigrator(repo, registry).Migrate(ctx, "local", "oss", false)
		migrated <- err
	}()
	select {
	case <-target.started:
	case err := <-migrated:
		t.Fatalf("migration stopped before Put: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	foreign := uuid.New()
	if rows, err := repo.ListStorageReplicas(ctx, asset.ID, foreign); err != nil || len(rows) != 0 {
		close(target.resume)
		t.Fatalf("foreign inventory read rows=%v err=%v", rows, err)
	}
	if err := repo.TrackStorageReplica(ctx, model.UserAssetStorageCleanup{AssetID: asset.ID, UserID: foreign, StorageBackend: "local", StorageKey: asset.StorageKey}); err == nil {
		close(target.resume)
		t.Fatal("foreign replica owner must fail composite FK")
	}
	// Intent must already be committed and visible on another connection while
	// external copying is paused, not hidden inside a transaction.
	replicas, err := repo.ListStorageReplicas(ctx, asset.ID, owner)
	if err != nil || len(replicas) != 2 {
		close(target.resume)
		t.Fatalf("durable pre-copy intent=%v err=%v", replicas, err)
	}
	blockedCtx, stop := context.WithTimeout(ctx, 150*time.Millisecond)
	entered := false
	err = repo.WithStorageLock(blockedCtx, asset.ID, func(context.Context) error { entered = true; return nil })
	stop()
	if err == nil || entered {
		close(target.resume)
		t.Fatal("concurrent storage operation bypassed migration lock")
	}
	removed := make(chan error, 1)
	go func() {
		_, err := NewUploadService(repo, nil, nil, nil, nil, registry).RemoveSource(ctx, owner, asset.ID)
		removed <- err
	}()
	close(target.resume)
	if err := <-migrated; err != nil {
		t.Fatal(err)
	}
	if err := <-removed; err != nil {
		t.Fatal(err)
	}
	current, err := repo.GetOwnedByID(ctx, asset.ID, owner)
	if err != nil || current == nil || current.Availability != model.UserAssetSourceRemoved || current.StorageBackend != "oss" {
		t.Fatalf("current=%+v err=%v", current, err)
	}
	for _, store := range []uploadstorage.Store{source, target} {
		if exists, err := store.Exists(ctx, asset.StorageKey); err != nil || exists {
			t.Fatalf("replica=%s exists=%v err=%v", store.Backend(), exists, err)
		}
	}
	replicas, err = repo.ListStorageReplicas(ctx, asset.ID, owner)
	if err != nil || len(replicas) != 0 {
		t.Fatalf("cleanup=%v err=%v", replicas, err)
	}
	// SQL list/CAS must refuse removal regardless of a caller's stale snapshot.
	listed, err := repo.ListByStorageBackend(ctx, "oss", nil, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range listed {
		if item.ID == asset.ID {
			t.Fatal("removed source selected for migration")
		}
	}
	swapped, err := repo.CompareAndSwapStorageBackend(ctx, asset.ID, owner, "oss", asset.StorageKey, "local")
	if err != nil || swapped {
		t.Fatalf("removed source CAS swapped=%v err=%v", swapped, err)
	}
}
