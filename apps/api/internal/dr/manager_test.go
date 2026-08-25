package dr

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestStatusUsesCommittedLatestManifestAndValidatesRemoteIdentity(t *testing.T) {
	store, err := NewFilesystemStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		Environment: "test", Driver: "filesystem", FilesystemRoot: t.TempDir(), OSSPrefix: "bodysense/production/postgres",
		DBPassword: "secret", ReleaseRevision: strings.Repeat("c", 40), MaxBackupAge: 30 * time.Hour,
	}
	manager := NewManager(cfg, store)
	now := time.Date(2026, 8, 24, 3, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }
	ctx := context.Background()

	payload := []byte("fixture-dump")
	sum := sha256.Sum256(payload)
	sha := hex.EncodeToString(sum[:])
	root := cfg.OSSPrefix + "/2026/08/24/20260824T010000Z-cccccccccccc"
	manifest := BackupManifest{
		SchemaVersion: BackupManifestSchema, BackupID: "20260824T010000Z-cccccccccccc",
		CreatedAt: now.Add(-2 * time.Hour), ReleaseRevision: cfg.ReleaseRevision, MigrationState: "54:false", DatabaseName: "bodysense",
		PGDumpVersion: "pg_dump 16", DumpSHA256: sha, DumpSizeBytes: int64(len(payload)),
		DumpObjectKey: root + "/backup.dump", ChecksumObjectKey: root + "/backup.dump.sha256", ManifestObjectKey: root + "/manifest.json",
	}
	put := func(key string, body []byte, metadata map[string]string) {
		t.Helper()
		if err := store.Put(ctx, key, bytes.NewReader(body), int64(len(body)), PutOptions{Metadata: metadata, ForbidOverwrite: true}); err != nil {
			t.Fatal(err)
		}
	}
	put(manifest.DumpObjectKey, payload, map[string]string{"sha256": sha})
	put(manifest.ChecksumObjectKey, []byte(sha+"  backup.dump\n"), nil)
	encoded, _ := json.Marshal(manifest)
	put(manifest.ManifestObjectKey, encoded, nil)
	// An incomplete newer backup without manifest must not become authoritative.
	put(cfg.OSSPrefix+"/2026/08/24/20260824T020000Z-dddddddddddd/backup.dump", payload, nil)

	status, err := manager.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.BackupID != manifest.BackupID || !status.RemoteVerified || status.AgeSeconds != int64((2*time.Hour).Seconds()) {
		t.Fatalf("unexpected status: %#v", status)
	}

	fsStore := store.(*filesystemStore)
	dumpPath, err := fsStore.objectPath(manifest.DumpObjectKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dumpPath+".metadata.json", []byte(`{"metadata":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Status(ctx); err == nil || !strings.Contains(err.Error(), "sha metadata mismatch") {
		t.Fatalf("expected missing sha metadata to fail closed, got %v", err)
	}
	metadataPayload, err := json.Marshal(filesystemMetadata{Metadata: map[string]string{"sha256": sha}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dumpPath+".metadata.json", metadataPayload, 0o600); err != nil {
		t.Fatal(err)
	}
	checksumPath, err := fsStore.objectPath(manifest.ChecksumObjectKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(checksumPath, []byte("corrupt checksum\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Status(ctx); err == nil || !strings.Contains(err.Error(), "checksum content mismatch") {
		t.Fatalf("expected checksum corruption to fail closed, got %v", err)
	}
}

func TestStatusRejectsStaleBackup(t *testing.T) {
	store, err := NewFilesystemStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{Environment: "test", Driver: "filesystem", FilesystemRoot: t.TempDir(), OSSPrefix: "p", DBPassword: "secret", ReleaseRevision: "rev", MaxBackupAge: time.Hour}
	manager := NewManager(cfg, store)
	now := time.Date(2026, 8, 24, 3, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }
	payload := []byte("x")
	sum := sha256.Sum256(payload)
	sha := hex.EncodeToString(sum[:])
	root := "p/2026/08/24/20260824T010000Z-rev"
	manifest := BackupManifest{SchemaVersion: BackupManifestSchema, BackupID: "old", CreatedAt: now.Add(-2 * time.Hour), ReleaseRevision: "rev", MigrationState: "54:false", DatabaseName: "db", PGDumpVersion: "pg_dump", DumpSHA256: sha, DumpSizeBytes: 1, DumpObjectKey: root + "/backup.dump", ChecksumObjectKey: root + "/backup.dump.sha256", ManifestObjectKey: root + "/manifest.json"}
	ctx := context.Background()
	for key, body := range map[string][]byte{manifest.DumpObjectKey: payload, manifest.ChecksumObjectKey: []byte(sha + "  backup.dump\n"), manifest.ManifestObjectKey: mustJSON(t, manifest)} {
		meta := map[string]string{}
		if key == manifest.DumpObjectKey {
			meta["sha256"] = sha
		}
		if err := store.Put(ctx, key, bytes.NewReader(body), int64(len(body)), PutOptions{Metadata: meta}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := manager.Status(ctx); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("expected stale error, got %v", err)
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}
