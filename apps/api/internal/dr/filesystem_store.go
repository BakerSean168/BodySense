package dr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type filesystemStore struct {
	root string
}

type filesystemMetadata struct {
	Metadata map[string]string `json:"metadata"`
}

func NewFilesystemStore(root string) (ObjectStore, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o700); err != nil {
		return nil, err
	}
	return &filesystemStore{root: abs}, nil
}

func (s *filesystemStore) objectPath(key string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(strings.TrimLeft(key, "/")))
	if clean == "." || strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return "", fmt.Errorf("invalid object key %q", key)
	}
	path := filepath.Join(s.root, clean)
	rel, err := filepath.Rel(s.root, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("object key escapes store root: %q", key)
	}
	return path, nil
}

func (s *filesystemStore) Put(_ context.Context, key string, body io.Reader, _ int64, options PutOptions) error {
	path, err := s.objectPath(key)
	if err != nil {
		return err
	}
	if options.ForbidOverwrite {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("object already exists: %s", key)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".object-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := io.Copy(tmp, body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	metadataBytes, err := json.Marshal(filesystemMetadata{Metadata: options.Metadata})
	if err != nil {
		return err
	}
	return os.WriteFile(path+".metadata.json", metadataBytes, 0o600)
}

func (s *filesystemStore) PutFile(ctx context.Context, key, path string, options PutOptions) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return err
	}
	return s.Put(ctx, key, file, stat.Size(), options)
}

func (s *filesystemStore) Get(_ context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	path, err := s.objectPath(key)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	info, err := s.info(key, path)
	if err != nil {
		file.Close()
		return nil, ObjectInfo{}, err
	}
	return file, info, nil
}

func (s *filesystemStore) Head(_ context.Context, key string) (ObjectInfo, error) {
	path, err := s.objectPath(key)
	if err != nil {
		return ObjectInfo{}, err
	}
	return s.info(key, path)
}

func (s *filesystemStore) List(_ context.Context, prefix string) ([]ObjectInfo, error) {
	var result []ObjectInfo
	err := filepath.WalkDir(s.root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || strings.HasSuffix(entry.Name(), ".metadata.json") {
			return nil
		}
		rel, err := filepath.Rel(s.root, path)
		if err != nil {
			return err
		}
		key := filepath.ToSlash(rel)
		if !strings.HasPrefix(key, prefix) {
			return nil
		}
		info, err := s.info(key, path)
		if err != nil {
			return err
		}
		result = append(result, info)
		return nil
	})
	sort.Slice(result, func(i, j int) bool { return result[i].Key < result[j].Key })
	return result, err
}

func (s *filesystemStore) info(key, path string) (ObjectInfo, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return ObjectInfo{}, err
	}
	metadata := map[string]string{}
	if payload, err := os.ReadFile(path + ".metadata.json"); err == nil {
		var stored filesystemMetadata
		if json.Unmarshal(payload, &stored) == nil && stored.Metadata != nil {
			metadata = stored.Metadata
		}
	}
	return ObjectInfo{Key: key, Size: stat.Size(), LastModified: stat.ModTime().UTC(), Metadata: metadata}, nil
}

var _ ObjectStore = (*filesystemStore)(nil)
