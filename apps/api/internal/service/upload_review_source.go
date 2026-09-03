package service

import (
	"context"
	"fmt"
	"io"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/uploadstorage"
)

// OpenUploadObject streams the authenticated private upload bytes through the
// upload-storage boundary. The upload manifest must already have been
// ownership-checked by GetUpload before this helper is reached. Callers never
// receive storage_backend or storage_key.
func (s *UploadService) OpenUploadObject(
	ctx context.Context,
	upload *model.UserUpload,
) (io.ReadCloser, uploadstorage.ObjectInfo, error) {
	if upload == nil {
		return nil, uploadstorage.ObjectInfo{}, fmt.Errorf("upload is nil")
	}
	return openUploadObject(ctx, s.storage, upload)
}
