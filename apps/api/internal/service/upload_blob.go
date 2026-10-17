package service

import (
	"context"
	"fmt"
	"io"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/uploadstorage"
)

func openUploadObject(
	ctx context.Context,
	storage *uploadstorage.Registry,
	asset *model.UserAsset,
) (io.ReadCloser, uploadstorage.ObjectInfo, error) {
	if storage == nil {
		return nil, uploadstorage.ObjectInfo{}, fmt.Errorf("upload storage is not configured")
	}
	if asset == nil || asset.StorageBackend == "" || asset.StorageKey == "" {
		return nil, uploadstorage.ObjectInfo{}, fmt.Errorf("upload storage identity is incomplete")
	}
	store, err := storage.Store(asset.StorageBackend)
	if err != nil {
		return nil, uploadstorage.ObjectInfo{}, err
	}
	reader, info, err := store.Open(ctx, asset.StorageKey)
	if err != nil {
		return nil, uploadstorage.ObjectInfo{}, err
	}
	if info.Size != asset.ByteSize {
		reader.Close()
		return nil, uploadstorage.ObjectInfo{}, fmt.Errorf(
			"upload object size mismatch: manifest=%d object=%d",
			asset.ByteSize,
			info.Size,
		)
	}
	return reader, info, nil
}

func readUploadObject(
	ctx context.Context,
	storage *uploadstorage.Registry,
	asset *model.UserAsset,
	maxBytes int64,
) ([]byte, error) {
	reader, _, err := openUploadObject(ctx, storage, asset)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	limit := maxBytes + 1
	if maxBytes <= 0 {
		limit = asset.ByteSize + 1
	}
	payload, err := io.ReadAll(io.LimitReader(reader, limit))
	if err != nil {
		return nil, err
	}
	if maxBytes > 0 && int64(len(payload)) > maxBytes {
		return nil, fmt.Errorf("upload object exceeds read limit")
	}
	return payload, nil
}
