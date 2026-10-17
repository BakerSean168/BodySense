package service

import (
	"context"
	"fmt"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/uploadstorage"
	"github.com/google/uuid"
)

type uploadSourceReplicaRepository interface {
	WithStorageLock(context.Context, uuid.UUID, func(context.Context) error) error
	TrackStorageReplica(context.Context, model.UserAssetStorageCleanup) error
	ListStorageReplicas(context.Context, uuid.UUID, uuid.UUID) ([]model.UserAssetStorageCleanup, error)
	ForgetStorageReplica(context.Context, model.UserAssetStorageCleanup) error
}

func sourceReplica(asset *model.UserAsset, backend string) model.UserAssetStorageCleanup {
	return model.UserAssetStorageCleanup{AssetID: asset.ID, UserID: asset.UserID, StorageBackend: backend, StorageKey: asset.StorageKey}
}

func deleteSourceReplica(ctx context.Context, storage *uploadstorage.Registry, repo uploadSourceReplicaRepository, replica model.UserAssetStorageCleanup) error {
	if storage == nil {
		return fmt.Errorf("upload storage is not configured")
	}
	store, err := storage.Store(replica.StorageBackend)
	if err != nil {
		return err
	}
	if err := store.Delete(ctx, replica.StorageKey); err != nil {
		return fmt.Errorf("delete source replica %s: %w", replica.StorageBackend, err)
	}
	exists, err := store.Exists(ctx, replica.StorageKey)
	if err != nil {
		return fmt.Errorf("verify source replica removal: %w", err)
	}
	if exists {
		return fmt.Errorf("source replica %s remains after deletion", replica.StorageBackend)
	}
	return repo.ForgetStorageReplica(ctx, replica)
}
