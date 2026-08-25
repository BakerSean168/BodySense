package uploadstorage

import (
	"context"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
}

type Store interface {
	Backend() string
	Validate(ctx context.Context) error
	Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
	Open(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error)
	Stat(ctx context.Context, key string) (ObjectInfo, error)
	Exists(ctx context.Context, key string) (bool, error)
	Delete(ctx context.Context, key string) error
	EraseUserObjects(ctx context.Context, userID uuid.UUID) error
}

type Registry struct {
	defaultBackend string
	stores         map[string]Store
}

func NewRegistryFromStores(defaultBackend string, stores ...Store) (*Registry, error) {
	byBackend := make(map[string]Store, len(stores))
	for _, store := range stores {
		if store == nil || strings.TrimSpace(store.Backend()) == "" {
			return nil, fmt.Errorf("upload storage registry received an invalid store")
		}
		if _, exists := byBackend[store.Backend()]; exists {
			return nil, fmt.Errorf("duplicate upload storage backend %q", store.Backend())
		}
		byBackend[store.Backend()] = store
	}
	if _, ok := byBackend[defaultBackend]; !ok {
		return nil, fmt.Errorf("default upload storage backend %q is not configured", defaultBackend)
	}
	return &Registry{defaultBackend: defaultBackend, stores: byBackend}, nil
}

func NewRegistry(cfg Config) (*Registry, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	local, err := NewLocalStore(cfg.LocalRoot)
	if err != nil {
		return nil, fmt.Errorf("configure local upload storage: %w", err)
	}
	stores := map[string]Store{"local": local}
	if cfg.Backend == "oss" || cfg.OSSBucket != "" {
		ossStore, err := NewOSSStore(cfg)
		if err != nil {
			return nil, fmt.Errorf("configure OSS upload storage: %w", err)
		}
		stores["oss"] = ossStore
	}
	configured := make([]Store, 0, len(stores))
	for _, store := range stores {
		configured = append(configured, store)
	}
	return NewRegistryFromStores(cfg.Backend, configured...)
}

func NewRegistryFromEnv() (*Registry, error) {
	cfg, err := ConfigFromEnv()
	if err != nil {
		return nil, err
	}
	return NewRegistry(cfg)
}

func (r *Registry) DefaultBackend() string { return r.defaultBackend }

func (r *Registry) DefaultStore() Store { return r.stores[r.defaultBackend] }

func (r *Registry) Store(backend string) (Store, error) {
	store, ok := r.stores[strings.TrimSpace(backend)]
	if !ok {
		return nil, fmt.Errorf("upload storage backend %q is not configured", backend)
	}
	return store, nil
}

func (r *Registry) Validate(ctx context.Context) error {
	backends := make([]string, 0, len(r.stores))
	for backend := range r.stores {
		backends = append(backends, backend)
	}
	sort.Strings(backends)
	for _, backend := range backends {
		if err := r.stores[backend].Validate(ctx); err != nil {
			return fmt.Errorf("validate %s upload storage: %w", backend, err)
		}
	}
	return nil
}

func (r *Registry) EraseUserObjects(ctx context.Context, userID uuid.UUID) error {
	backends := make([]string, 0, len(r.stores))
	for backend := range r.stores {
		backends = append(backends, backend)
	}
	sort.Strings(backends)
	for _, backend := range backends {
		if err := r.stores[backend].EraseUserObjects(ctx, userID); err != nil {
			return fmt.Errorf("erase %s upload objects: %w", backend, err)
		}
	}
	return nil
}

func BuildObjectKey(userID, uploadID uuid.UUID, mimeType string) (string, error) {
	ext, ok := map[string]string{
		"image/jpeg":      ".jpg",
		"image/png":       ".png",
		"image/webp":      ".webp",
		"application/pdf": ".pdf",
	}[mimeType]
	if !ok {
		return "", fmt.Errorf("unsupported upload mime type %q", mimeType)
	}
	return path.Join(userID.String(), uploadID.String(), "original"+ext), nil
}

func userPrefix(userID uuid.UUID) string { return userID.String() + "/" }

func validateStorageKey(key string) error {
	if key == "" || strings.Contains(key, "\\") || strings.HasPrefix(key, "/") {
		return fmt.Errorf("invalid upload storage key %q", key)
	}
	clean := path.Clean(key)
	if clean != key || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("invalid upload storage key %q", key)
	}
	return nil
}
