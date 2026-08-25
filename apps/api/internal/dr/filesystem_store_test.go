package dr

import (
	"bytes"
	"context"
	"io"
	"testing"
)

func TestFilesystemStoreForbidOverwriteAndMetadata(t *testing.T) {
	store, err := NewFilesystemStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	key := "backups/one.dump"
	payload := []byte("backup")
	options := PutOptions{Metadata: map[string]string{"sha256": "abc"}, ForbidOverwrite: true}
	if err := store.Put(ctx, key, bytes.NewReader(payload), int64(len(payload)), options); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(ctx, key, bytes.NewReader(payload), int64(len(payload)), options); err == nil {
		t.Fatal("expected overwrite to fail")
	}
	reader, info, err := store.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) || info.Metadata["sha256"] != "abc" {
		t.Fatalf("unexpected stored object: %#v %q", info, got)
	}
}

func TestFilesystemStoreRejectsTraversal(t *testing.T) {
	store, err := NewFilesystemStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(context.Background(), "../escape", bytes.NewReader(nil), 0, PutOptions{}); err == nil {
		t.Fatal("expected path traversal to fail")
	}
}
