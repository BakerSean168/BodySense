package dr

import (
	"context"
	"io"
	"time"
)

type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
	Metadata     map[string]string
}

type PutOptions struct {
	Metadata             map[string]string
	ContentType          string
	ForbidOverwrite      bool
	ServerSideEncryption string
}

type ObjectStore interface {
	Put(ctx context.Context, key string, body io.Reader, size int64, options PutOptions) error
	PutFile(ctx context.Context, key, path string, options PutOptions) error
	Get(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error)
	Head(ctx context.Context, key string) (ObjectInfo, error)
	List(ctx context.Context, prefix string) ([]ObjectInfo, error)
}
