package uploadstorage

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestBuildObjectKeyIsOwnerScopedAndMimeDerived(t *testing.T) {
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	uploadID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	key, err := BuildObjectKey(userID, uploadID, "image/jpeg")
	if err != nil {
		t.Fatal(err)
	}
	want := "11111111-1111-1111-1111-111111111111/22222222-2222-2222-2222-222222222222/original.jpg"
	if key != want {
		t.Fatalf("key=%q want=%q", key, want)
	}
	if _, err := BuildObjectKey(userID, uploadID, "text/plain"); err == nil {
		t.Fatal("expected unsupported mime to fail")
	}
}

func TestLocalStoreLifecycleAndUserEraseIsolation(t *testing.T) {
	store, err := NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	userA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	userB := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	keyA := userA.String() + "/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/original.jpg"
	keyB := userB.String() + "/bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb/original.jpg"

	for key, payload := range map[string]string{keyA: "alpha", keyB: "beta"} {
		if err := store.Put(ctx, key, strings.NewReader(payload), int64(len(payload)), "image/jpeg"); err != nil {
			t.Fatalf("put %s: %v", key, err)
		}
	}
	if err := store.Put(ctx, keyA, strings.NewReader("again"), 5, "image/jpeg"); err == nil {
		t.Fatal("expected immutable put to reject overwrite")
	}
	info, err := store.Stat(ctx, keyA)
	if err != nil || info.Size != 5 {
		t.Fatalf("stat=%#v err=%v", info, err)
	}
	reader, _, err := store.Open(ctx, keyA)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := io.ReadAll(reader)
	reader.Close()
	if err != nil || string(payload) != "alpha" {
		t.Fatalf("payload=%q err=%v", payload, err)
	}
	if err := store.EraseUserObjects(ctx, userA); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Stat(ctx, keyA); err == nil {
		t.Fatal("user A object still exists")
	}
	if _, err := store.Stat(ctx, keyB); err != nil {
		t.Fatalf("user B object was affected: %v", err)
	}
	if err := store.Delete(ctx, keyA); err != nil {
		t.Fatalf("idempotent delete: %v", err)
	}
}

func TestLocalStoreRejectsEscapingKeys(t *testing.T) {
	store, err := NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"../escape", "/absolute", "a/../../escape", ""} {
		if err := store.Put(context.Background(), key, strings.NewReader("x"), 1, "text/plain"); err == nil {
			t.Fatalf("expected key %q to be rejected", key)
		}
	}
}
