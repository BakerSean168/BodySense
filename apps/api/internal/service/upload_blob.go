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
	upload *model.UserUpload,
) (io.ReadCloser, uploadstorage.ObjectInfo, error) {
	if storage == nil {
		return nil, uploadstorage.ObjectInfo{}, fmt.Errorf("upload storage is not configured")
	}
	if upload == nil || upload.StorageBackend == "" || upload.StorageKey == "" {
		return nil, uploadstorage.ObjectInfo{}, fmt.Errorf("upload storage identity is incomplete")
	}
	store, err := storage.Store(upload.StorageBackend)
	if err != nil {
		return nil, uploadstorage.ObjectInfo{}, err
	}
	reader, info, err := store.Open(ctx, upload.StorageKey)
	if err != nil {
		return nil, uploadstorage.ObjectInfo{}, err
	}
	if info.Size != upload.FileSize {
		reader.Close()
		return nil, uploadstorage.ObjectInfo{}, fmt.Errorf(
			"upload object size mismatch: manifest=%d object=%d",
			upload.FileSize,
			info.Size,
		)
	}
	return reader, info, nil
}

func readUploadObject(
	ctx context.Context,
	storage *uploadstorage.Registry,
	upload *model.UserUpload,
	maxBytes int64,
) ([]byte, error) {
	reader, _, err := openUploadObject(ctx, storage, upload)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	limit := maxBytes + 1
	if maxBytes <= 0 {
		limit = upload.FileSize + 1
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
