package uploadstorage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

type localStore struct {
	root string
}

func NewLocalStore(root string) (Store, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if abs == string(filepath.Separator) {
		return nil, errors.New("upload storage root may not be filesystem root")
	}
	if err := os.MkdirAll(abs, 0o700); err != nil {
		return nil, err
	}
	return &localStore{root: abs}, nil
}

func (s *localStore) Backend() string { return "local" }

func (s *localStore) Validate(context.Context) error { return nil }

func (s *localStore) objectPath(key string) (string, error) {
	if err := validateStorageKey(key); err != nil {
		return "", err
	}
	candidate := filepath.Join(s.root, filepath.FromSlash(key))
	rel, err := filepath.Rel(s.root, candidate)
	if err != nil || rel == "." || rel == ".." || filepath.IsAbs(rel) || len(rel) >= 3 && rel[:3] == "../" {
		return "", fmt.Errorf("upload storage key escapes local root: %q", key)
	}
	return candidate, nil
}

func (s *localStore) Put(_ context.Context, key string, body io.Reader, size int64, _ string) error {
	objectPath, err := s.objectPath(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(objectPath), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(objectPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("upload object already exists: %s", key)
		}
		return err
	}
	committed := false
	defer func() {
		file.Close()
		if !committed {
			_ = os.Remove(objectPath)
		}
	}()
	written, err := io.Copy(file, body)
	if err != nil {
		return err
	}
	if size >= 0 && written != size {
		return fmt.Errorf("upload object size mismatch: expected=%d wrote=%d", size, written)
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	committed = true
	return nil
}

func (s *localStore) Open(_ context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	objectPath, err := s.objectPath(key)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	file, err := os.Open(objectPath)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, ObjectInfo{}, err
	}
	return file, ObjectInfo{Key: key, Size: stat.Size(), LastModified: stat.ModTime().UTC()}, nil
}

func (s *localStore) Stat(_ context.Context, key string) (ObjectInfo, error) {
	objectPath, err := s.objectPath(key)
	if err != nil {
		return ObjectInfo{}, err
	}
	stat, err := os.Stat(objectPath)
	if err != nil {
		return ObjectInfo{}, err
	}
	return ObjectInfo{Key: key, Size: stat.Size(), LastModified: stat.ModTime().UTC()}, nil
}

func (s *localStore) Exists(_ context.Context, key string) (bool, error) {
	objectPath, err := s.objectPath(key)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(objectPath)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func (s *localStore) Delete(_ context.Context, key string) error {
	objectPath, err := s.objectPath(key)
	if err != nil {
		return err
	}
	if err := os.Remove(objectPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *localStore) EraseUserObjects(_ context.Context, userID uuid.UUID) error {
	userRoot, err := s.objectPath(userID.String() + "/placeholder")
	if err != nil {
		return err
	}
	userRoot = filepath.Dir(userRoot)
	if err := os.RemoveAll(userRoot); err != nil {
		return fmt.Errorf("erase local user upload prefix: %w", err)
	}
	return nil
}

var _ Store = (*localStore)(nil)
