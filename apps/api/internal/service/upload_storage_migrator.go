package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/uploadstorage"
	"github.com/google/uuid"
)

type uploadStorageMigrationRepository interface {
	uploadSourceReplicaRepository
	GetOwnedByID(context.Context, uuid.UUID, uuid.UUID) (*model.UserAsset, error)
	ListByStorageBackend(ctx context.Context, backend string, after *uuid.UUID, limit int) ([]model.UserAsset, error)
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
		assets, err := m.repo.ListByStorageBackend(ctx, fromBackend, after, 100)
		if err != nil {
			return result, fmt.Errorf("list %s uploads: %w", fromBackend, err)
		}
		if len(assets) == 0 {
			break
		}
		for i := range assets {
			asset := assets[i]
			result.Scanned++
			if dryRun {
				continue
			}
			err := m.repo.WithStorageLock(ctx, asset.ID, func(ctx context.Context) error {
				current, err := m.repo.GetOwnedByID(ctx, asset.ID, asset.UserID)
				if err != nil {
					return err
				}
				if current == nil || current.Availability != model.UserAssetAvailable || current.StorageKey != asset.StorageKey {
					return fmt.Errorf("upload %s is no longer available on the source backend", asset.ID)
				}
				if current.StorageBackend != fromBackend {
					// Resume cleanup after a committed cutover whose old-object Delete
					// failed or whose process stopped before cleanup finished.
					replicas, err := m.repo.ListStorageReplicas(ctx, current.ID, current.UserID)
					if err != nil {
						return err
					}
					for _, replica := range replicas {
						if replica.StorageBackend == fromBackend {
							if err := deleteSourceReplica(ctx, m.storage, m.repo, replica); err != nil {
								return err
							}
						}
					}
					return nil
				}
				oldReplica, targetReplica := sourceReplica(current, fromBackend), sourceReplica(current, toBackend)
				for _, replica := range []model.UserAssetStorageCleanup{oldReplica, targetReplica} {
					if err := m.repo.TrackStorageReplica(ctx, replica); err != nil {
						return err
					}
				}
				already, copyErr := migrateUploadObject(ctx, source, target, current)
				if copyErr != nil {
					// Preserve pre-existing or partially written targets on copy failure.
					// The ledger keeps them removable/retryable without deleting a
					// potentially recoverable copy when the source read failed.
					return copyErr
				}
				if already {
					result.AlreadyVerified++
				}
				swapped, swapErr := m.repo.CompareAndSwapStorageBackend(ctx, asset.ID, asset.UserID, fromBackend, asset.StorageKey, toBackend)
				if swapErr != nil {
					// Resolve a failed/ambiguous write before deleting anything that may
					// have become authoritative. Unknown outcomes retain durable intent.
					current, readErr := m.repo.GetOwnedByID(ctx, asset.ID, asset.UserID)
					if readErr == nil && current != nil && current.StorageBackend == fromBackend && current.StorageKey == asset.StorageKey {
						return errors.Join(swapErr, deleteSourceReplica(ctx, m.storage, m.repo, targetReplica))
					}
					return errors.Join(fmt.Errorf("advance upload %s storage manifest: %w", asset.ID, swapErr), readErr)
				}
				if !swapped {
					return errors.Join(fmt.Errorf("upload %s storage manifest changed during migration", asset.ID), deleteSourceReplica(ctx, m.storage, m.repo, targetReplica))
				}
				result.Migrated++
				return deleteSourceReplica(ctx, m.storage, m.repo, oldReplica)
			})
			if err != nil {
				return result, fmt.Errorf("migrate upload %s: %w", asset.ID, err)
			}
		}
		last := assets[len(assets)-1].ID
		after = &last
	}
	return result, nil
}

func migrateUploadObject(ctx context.Context, source, target uploadstorage.Store, asset *model.UserAsset) (bool, error) {
	reader, info, err := source.Open(ctx, asset.StorageKey)
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
	if int64(len(payload)) != asset.ByteSize || info.Size != asset.ByteSize {
		return false, fmt.Errorf("source object size mismatch: manifest=%d object=%d read=%d", asset.ByteSize, info.Size, len(payload))
	}
	sourceHash := sha256.Sum256(payload)
	exists, err := target.Exists(ctx, asset.StorageKey)
	if err != nil {
		return false, fmt.Errorf("check target object: %w", err)
	}
	if !exists {
		if err := target.Put(ctx, asset.StorageKey, bytes.NewReader(payload), int64(len(payload)), asset.MimeType); err != nil {
			return false, fmt.Errorf("write target object: %w", err)
		}
	} else {
		if err := verifyMigratedUploadObject(ctx, target, asset, sourceHash); err != nil {
			return false, err
		}
		return true, nil
	}
	if err := verifyMigratedUploadObject(ctx, target, asset, sourceHash); err != nil {
		return false, err
	}
	return false, nil
}

func verifyMigratedUploadObject(ctx context.Context, target uploadstorage.Store, asset *model.UserAsset, wantHash [32]byte) error {
	reader, info, err := target.Open(ctx, asset.StorageKey)
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
	if info.Size != asset.ByteSize || int64(len(payload)) != asset.ByteSize {
		return fmt.Errorf("target object size mismatch")
	}
	if got := sha256.Sum256(payload); got != wantHash {
		return fmt.Errorf("target object checksum mismatch")
	}
	return nil
}
