package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/uploadstorage"
	"github.com/google/uuid"
)

type uploadStorageMigrationRepository interface {
	ListByStorageBackend(ctx context.Context, backend string, after *uuid.UUID, limit int) ([]model.UserUpload, error)
	CompareAndSwapStorageBackend(ctx context.Context, id, userID uuid.UUID, fromBackend, storageKey, toBackend string) (bool, error)
}

type UploadStorageMigrationResult struct {
	Scanned         int `json:"scanned"`
	Migrated        int `json:"migrated"`
	AlreadyVerified int `json:"already_verified"`
}

type UploadStorageMigrator struct {
	repo    uploadStorageMigrationRepository
	storage *uploadstorage.Registry
}

func NewUploadStorageMigrator(repo uploadStorageMigrationRepository, storage *uploadstorage.Registry) *UploadStorageMigrator {
	return &UploadStorageMigrator{repo: repo, storage: storage}
}

func (m *UploadStorageMigrator) Migrate(ctx context.Context, fromBackend, toBackend string, dryRun bool) (UploadStorageMigrationResult, error) {
	var result UploadStorageMigrationResult
	if m.repo == nil || m.storage == nil {
		return result, fmt.Errorf("upload storage migrator is not configured")
	}
	if fromBackend == "" || toBackend == "" || fromBackend == toBackend {
		return result, fmt.Errorf("distinct source and target upload storage backends are required")
	}
	source, err := m.storage.Store(fromBackend)
	if err != nil {
		return result, err
	}
	var target uploadstorage.Store
	if !dryRun {
		target, err = m.storage.Store(toBackend)
		if err != nil {
			return result, err
		}
	}
	var after *uuid.UUID
	for {
		uploads, err := m.repo.ListByStorageBackend(ctx, fromBackend, after, 100)
		if err != nil {
			return result, fmt.Errorf("list %s uploads: %w", fromBackend, err)
		}
		if len(uploads) == 0 {
			break
		}
		for i := range uploads {
			upload := uploads[i]
			result.Scanned++
			if dryRun {
				continue
			}
			already, err := migrateUploadObject(ctx, source, target, &upload)
			if err != nil {
				return result, fmt.Errorf("migrate upload %s: %w", upload.ID, err)
			}
			if already {
				result.AlreadyVerified++
			}
			swapped, err := m.repo.CompareAndSwapStorageBackend(
				ctx, upload.ID, upload.UserID, fromBackend, upload.StorageKey, toBackend,
			)
			if err != nil {
				return result, fmt.Errorf("advance upload %s storage manifest: %w", upload.ID, err)
			}
			if !swapped {
				return result, fmt.Errorf("upload %s storage manifest changed during migration", upload.ID)
			}
			result.Migrated++
		}
		last := uploads[len(uploads)-1].ID
		after = &last
	}
	return result, nil
}

func migrateUploadObject(ctx context.Context, source, target uploadstorage.Store, upload *model.UserUpload) (bool, error) {
	reader, info, err := source.Open(ctx, upload.StorageKey)
	if err != nil {
		return false, fmt.Errorf("open source object: %w", err)
	}
	payload, readErr := io.ReadAll(io.LimitReader(reader, MaxFileSize+1))
	closeErr := reader.Close()
	if readErr != nil {
		return false, readErr
	}
	if closeErr != nil {
		return false, closeErr
	}
	if int64(len(payload)) != upload.FileSize || info.Size != upload.FileSize {
		return false, fmt.Errorf("source object size mismatch: manifest=%d object=%d read=%d", upload.FileSize, info.Size, len(payload))
	}
	sourceHash := sha256.Sum256(payload)
	exists, err := target.Exists(ctx, upload.StorageKey)
	if err != nil {
		return false, fmt.Errorf("check target object: %w", err)
	}
	if !exists {
		if err := target.Put(ctx, upload.StorageKey, bytes.NewReader(payload), int64(len(payload)), upload.MimeType); err != nil {
			return false, fmt.Errorf("write target object: %w", err)
		}
	} else {
		if err := verifyMigratedUploadObject(ctx, target, upload, sourceHash); err != nil {
			return false, err
		}
		return true, nil
	}
	if err := verifyMigratedUploadObject(ctx, target, upload, sourceHash); err != nil {
		return false, err
	}
	return false, nil
}

func verifyMigratedUploadObject(ctx context.Context, target uploadstorage.Store, upload *model.UserUpload, wantHash [32]byte) error {
	reader, info, err := target.Open(ctx, upload.StorageKey)
	if err != nil {
		return fmt.Errorf("open target object: %w", err)
	}
	payload, readErr := io.ReadAll(io.LimitReader(reader, MaxFileSize+1))
	closeErr := reader.Close()
	if readErr != nil {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	if info.Size != upload.FileSize || int64(len(payload)) != upload.FileSize {
		return fmt.Errorf("target object size mismatch")
	}
	if got := sha256.Sum256(payload); got != wantHash {
		return fmt.Errorf("target object checksum mismatch")
	}
	return nil
}
