package dr

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const BackupManifestSchema = "bodysense.production-postgres-backup.v1"

type BackupManifest struct {
	SchemaVersion     string    `json:"schema_version"`
	BackupID          string    `json:"backup_id"`
	CreatedAt         time.Time `json:"created_at"`
	ReleaseRevision   string    `json:"release_revision"`
	MigrationState    string    `json:"migration_state"`
	DatabaseName      string    `json:"database_name"`
	PGDumpVersion     string    `json:"pg_dump_version"`
	DumpSHA256        string    `json:"dump_sha256"`
	DumpSizeBytes     int64     `json:"dump_size_bytes"`
	DumpObjectKey     string    `json:"dump_object_key"`
	ChecksumObjectKey string    `json:"checksum_object_key"`
	ManifestObjectKey string    `json:"manifest_object_key"`
}

type BackupResult struct {
	BackupManifest
	RemoteVerified bool `json:"remote_verified"`
}

type RestoreDrillResult struct {
	ManifestObjectKey string    `json:"manifest_object_key"`
	BackupID          string    `json:"backup_id"`
	RestoredAt        time.Time `json:"restored_at"`
	MigrationState    string    `json:"migration_state"`
	DomainSemantics   string    `json:"domain_semantics"`
	DisposableDB      string    `json:"disposable_database"`
	Dropped           bool      `json:"dropped"`
}

type StatusResult struct {
	ManifestObjectKey string        `json:"manifest_object_key"`
	BackupID          string        `json:"backup_id"`
	CreatedAt         time.Time     `json:"created_at"`
	Age               time.Duration `json:"-"`
	AgeSeconds        int64         `json:"age_seconds"`
	MigrationState    string        `json:"migration_state"`
	ReleaseRevision   string        `json:"release_revision"`
	DumpSHA256        string        `json:"dump_sha256"`
	DumpSizeBytes     int64         `json:"dump_size_bytes"`
	RemoteVerified    bool          `json:"remote_verified"`
}

type Manager struct {
	cfg   Config
	store ObjectStore
	now   func() time.Time
}

func NewManager(cfg Config, store ObjectStore) *Manager {
	return &Manager{cfg: cfg, store: store, now: func() time.Time { return time.Now().UTC() }}
}

func NewStore(cfg Config) (ObjectStore, error) {
	switch cfg.Driver {
	case "filesystem":
		return NewFilesystemStore(cfg.FilesystemRoot)
	case "oss":
		return NewOSSStore(cfg)
	default:
		return nil, fmt.Errorf("unsupported object store driver %q", cfg.Driver)
	}
}

func (m *Manager) Backup(ctx context.Context) (BackupResult, error) {
	stamp := m.now().UTC()
	shortRevision := m.cfg.ReleaseRevision
	if len(shortRevision) > 12 {
		shortRevision = shortRevision[:12]
	}
	backupID := fmt.Sprintf("%s-%s", stamp.Format("20060102T150405Z"), shortRevision)
	keyRoot := fmt.Sprintf("%s/%s/%s", m.cfg.OSSPrefix, stamp.Format("2006/01/02"), backupID)
	manifest := BackupManifest{
		SchemaVersion:     BackupManifestSchema,
		BackupID:          backupID,
		CreatedAt:         stamp,
		ReleaseRevision:   m.cfg.ReleaseRevision,
		DatabaseName:      m.cfg.DBName,
		DumpObjectKey:     keyRoot + "/backup.dump",
		ChecksumObjectKey: keyRoot + "/backup.dump.sha256",
		ManifestObjectKey: keyRoot + "/manifest.json",
	}

	tmpDir, err := os.MkdirTemp(m.cfg.TempRoot, "bodysense-dr-backup-*")
	if err != nil {
		return BackupResult{}, fmt.Errorf("create backup temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)
	if err := os.Chmod(tmpDir, 0o700); err != nil {
		return BackupResult{}, err
	}
	dumpPath := filepath.Join(tmpDir, "backup.dump")

	manifest.MigrationState, err = m.migrationState(ctx, m.cfg.DBName)
	if err != nil {
		return BackupResult{}, fmt.Errorf("read source migration state: %w", err)
	}
	manifest.PGDumpVersion, err = m.commandOutput(ctx, m.cfg.DBName, "pg_dump", "--version")
	if err != nil {
		return BackupResult{}, fmt.Errorf("read pg_dump version: %w", err)
	}
	if _, err := m.commandOutput(ctx, m.cfg.DBName, "pg_dump", "--format=custom", "--no-owner", "--no-acl", "--file", dumpPath); err != nil {
		return BackupResult{}, fmt.Errorf("pg_dump: %w", err)
	}
	if _, err := m.commandOutput(ctx, m.cfg.DBName, "pg_restore", "--list", dumpPath); err != nil {
		return BackupResult{}, fmt.Errorf("validate pg_dump archive: %w", err)
	}
	manifest.DumpSHA256, manifest.DumpSizeBytes, err = sha256File(dumpPath)
	if err != nil {
		return BackupResult{}, err
	}
	if manifest.DumpSizeBytes <= 0 {
		return BackupResult{}, errors.New("database dump is empty")
	}

	metadata := map[string]string{
		"sha256":           manifest.DumpSHA256,
		"backup-id":        manifest.BackupID,
		"release-revision": manifest.ReleaseRevision,
		"migration-state":  manifest.MigrationState,
		"kind":             "postgres-dump",
	}
	if err := m.putFile(ctx, manifest.DumpObjectKey, dumpPath, "application/octet-stream", metadata); err != nil {
		return BackupResult{}, fmt.Errorf("upload database dump: %w", err)
	}
	checksum := []byte(fmt.Sprintf("%s  backup.dump\n", manifest.DumpSHA256))
	if err := m.putBytes(ctx, manifest.ChecksumObjectKey, checksum, "text/plain", map[string]string{"backup-id": manifest.BackupID, "kind": "sha256"}); err != nil {
		return BackupResult{}, fmt.Errorf("upload checksum: %w", err)
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return BackupResult{}, err
	}
	manifestBytes = append(manifestBytes, '\n')
	// The manifest is the commit marker and is intentionally uploaded last.
	if err := m.putBytes(ctx, manifest.ManifestObjectKey, manifestBytes, "application/json", map[string]string{"backup-id": manifest.BackupID, "kind": "manifest"}); err != nil {
		return BackupResult{}, fmt.Errorf("upload manifest: %w", err)
	}
	if err := m.verifyRemoteManifest(ctx, manifest); err != nil {
		return BackupResult{}, err
	}
	return BackupResult{BackupManifest: manifest, RemoteVerified: true}, nil
}

func (m *Manager) Status(ctx context.Context) (StatusResult, error) {
	manifest, err := m.latestManifest(ctx)
	if err != nil {
		return StatusResult{}, err
	}
	if err := m.verifyRemoteManifest(ctx, manifest); err != nil {
		return StatusResult{}, err
	}
	age := m.now().UTC().Sub(manifest.CreatedAt)
	if age < 0 {
		return StatusResult{}, fmt.Errorf("latest backup timestamp is in the future: %s", manifest.CreatedAt)
	}
	if age > m.cfg.MaxBackupAge {
		return StatusResult{}, fmt.Errorf("latest backup is stale: age=%s max=%s", age.Round(time.Second), m.cfg.MaxBackupAge)
	}
	return StatusResult{
		ManifestObjectKey: manifest.ManifestObjectKey,
		BackupID:          manifest.BackupID, CreatedAt: manifest.CreatedAt, Age: age, AgeSeconds: int64(age.Seconds()),
		MigrationState: manifest.MigrationState, ReleaseRevision: manifest.ReleaseRevision,
		DumpSHA256: manifest.DumpSHA256, DumpSizeBytes: manifest.DumpSizeBytes, RemoteVerified: true,
	}, nil
}

func (m *Manager) RestoreDrill(ctx context.Context) (result RestoreDrillResult, returnErr error) {
	manifest, err := m.latestManifest(ctx)
	if err != nil {
		return result, err
	}
	if m.now().UTC().Sub(manifest.CreatedAt) > m.cfg.MaxBackupAge {
		return result, fmt.Errorf("refusing stale backup restore drill: %s", manifest.CreatedAt)
	}
	tmpDir, err := os.MkdirTemp(m.cfg.TempRoot, "bodysense-dr-restore-*")
	if err != nil {
		return result, err
	}
	defer os.RemoveAll(tmpDir)
	dumpPath := filepath.Join(tmpDir, "backup.dump")
	if err := m.downloadToFile(ctx, manifest.DumpObjectKey, dumpPath); err != nil {
		return result, fmt.Errorf("download database dump: %w", err)
	}
	sha, size, err := sha256File(dumpPath)
	if err != nil {
		return result, err
	}
	if sha != manifest.DumpSHA256 || size != manifest.DumpSizeBytes {
		return result, fmt.Errorf("downloaded dump identity mismatch: sha=%s size=%d", sha, size)
	}
	if _, err := m.commandOutput(ctx, m.cfg.DBName, "pg_restore", "--list", dumpPath); err != nil {
		return result, fmt.Errorf("pg_restore archive validation: %w", err)
	}
	drillDB, err := randomDatabaseName()
	if err != nil {
		return result, err
	}
	result = RestoreDrillResult{ManifestObjectKey: manifest.ManifestObjectKey, BackupID: manifest.BackupID, RestoredAt: m.now().UTC(), DisposableDB: drillDB}
	created := false
	defer func() {
		if !created {
			return
		}
		if _, err := m.commandOutput(context.Background(), m.cfg.DBName, "dropdb", "--if-exists", "--force", drillDB); err != nil {
			result.Dropped = false
			if returnErr == nil {
				returnErr = fmt.Errorf("drop disposable restore database: %w", err)
			}
			return
		}
		result.Dropped = true
	}()
	if _, err := m.commandOutput(ctx, m.cfg.DBName, "createdb", "--maintenance-db", m.cfg.DBName, "--template", "template0", drillDB); err != nil {
		return result, fmt.Errorf("create disposable restore database: %w", err)
	}
	created = true
	if _, err := m.commandOutput(ctx, drillDB, "pg_restore", "--exit-on-error", "--no-owner", "--no-acl", "--dbname", drillDB, dumpPath); err != nil {
		return result, fmt.Errorf("restore disposable database: %w", err)
	}
	result.MigrationState, err = m.migrationState(ctx, drillDB)
	if err != nil {
		return result, err
	}
	if result.MigrationState != manifest.MigrationState {
		return result, fmt.Errorf("restored migration state mismatch: want=%s got=%s", manifest.MigrationState, result.MigrationState)
	}
	validatorOutput, err := m.runDomainValidator(ctx, drillDB)
	if err != nil {
		return result, fmt.Errorf("domain validator: %w", err)
	}
	if !strings.Contains(validatorOutput, "DOMAIN_SEMANTICS=PASS") {
		return result, fmt.Errorf("domain validator did not report DOMAIN_SEMANTICS=PASS: %s", validatorOutput)
	}
	result.DomainSemantics = "PASS"
	return result, nil
}

func (m *Manager) latestManifest(ctx context.Context) (BackupManifest, error) {
	objects, err := m.store.List(ctx, m.cfg.OSSPrefix+"/")
	if err != nil {
		return BackupManifest{}, fmt.Errorf("list backup objects: %w", err)
	}
	keys := make([]string, 0)
	for _, object := range objects {
		if strings.HasSuffix(object.Key, "/manifest.json") {
			keys = append(keys, object.Key)
		}
	}
	if len(keys) == 0 {
		return BackupManifest{}, errors.New("no committed off-host PostgreSQL backup manifest found")
	}
	sort.Strings(keys)
	latestKey := keys[len(keys)-1]
	reader, _, err := m.store.Get(ctx, latestKey)
	if err != nil {
		return BackupManifest{}, err
	}
	defer reader.Close()
	payload, err := io.ReadAll(io.LimitReader(reader, 1<<20))
	if err != nil {
		return BackupManifest{}, err
	}
	var manifest BackupManifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return BackupManifest{}, fmt.Errorf("decode backup manifest: %w", err)
	}
	if err := validateManifest(manifest, latestKey); err != nil {
		return BackupManifest{}, err
	}
	return manifest, nil
}

func validateManifest(manifest BackupManifest, manifestKey string) error {
	if manifest.SchemaVersion != BackupManifestSchema {
		return fmt.Errorf("unsupported backup manifest schema %q", manifest.SchemaVersion)
	}
	if manifest.BackupID == "" || manifest.ReleaseRevision == "" || manifest.MigrationState == "" || manifest.DatabaseName == "" {
		return errors.New("backup manifest is missing identity fields")
	}
	if manifest.DumpSHA256 == "" || len(manifest.DumpSHA256) != sha256.Size*2 || manifest.DumpSizeBytes <= 0 {
		return errors.New("backup manifest has invalid dump identity")
	}
	if manifest.ManifestObjectKey != manifestKey || manifest.DumpObjectKey == "" || manifest.ChecksumObjectKey == "" {
		return errors.New("backup manifest object identity mismatch")
	}
	if manifest.CreatedAt.IsZero() {
		return errors.New("backup manifest has no created_at")
	}
	return nil
}

func (m *Manager) verifyRemoteManifest(ctx context.Context, manifest BackupManifest) error {
	if err := validateManifest(manifest, manifest.ManifestObjectKey); err != nil {
		return err
	}
	dump, err := m.store.Head(ctx, manifest.DumpObjectKey)
	if err != nil {
		return fmt.Errorf("head remote dump: %w", err)
	}
	if dump.Size != manifest.DumpSizeBytes {
		return fmt.Errorf("remote dump size mismatch: want=%d got=%d", manifest.DumpSizeBytes, dump.Size)
	}
	remoteSHA := dump.Metadata["sha256"]
	if remoteSHA != manifest.DumpSHA256 {
		return fmt.Errorf("remote dump sha metadata mismatch: want=%s got=%s", manifest.DumpSHA256, remoteSHA)
	}
	checksumReader, checksumInfo, err := m.store.Get(ctx, manifest.ChecksumObjectKey)
	if err != nil {
		return fmt.Errorf("read remote checksum: %w", err)
	}
	checksumPayload, readErr := io.ReadAll(io.LimitReader(checksumReader, 1<<20))
	closeErr := checksumReader.Close()
	if readErr != nil {
		return fmt.Errorf("read remote checksum payload: %w", readErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close remote checksum payload: %w", closeErr)
	}
	expectedChecksum := fmt.Sprintf("%s  backup.dump\n", manifest.DumpSHA256)
	if checksumInfo.Size != int64(len(expectedChecksum)) || string(checksumPayload) != expectedChecksum {
		return errors.New("remote checksum content mismatch")
	}
	if _, err := m.store.Head(ctx, manifest.ManifestObjectKey); err != nil {
		return fmt.Errorf("head remote manifest: %w", err)
	}
	return nil
}

func (m *Manager) putFile(ctx context.Context, key, path, contentType string, metadata map[string]string) error {
	return m.store.PutFile(ctx, key, path, PutOptions{
		Metadata: metadata, ContentType: contentType, ForbidOverwrite: true,
		ServerSideEncryption: m.cfg.OSSServerEncryption,
	})
}

func (m *Manager) putBytes(ctx context.Context, key string, payload []byte, contentType string, metadata map[string]string) error {
	return m.store.Put(ctx, key, bytes.NewReader(payload), int64(len(payload)), PutOptions{Metadata: metadata, ContentType: contentType, ForbidOverwrite: true, ServerSideEncryption: m.cfg.OSSServerEncryption})
}

func (m *Manager) downloadToFile(ctx context.Context, key, path string) error {
	reader, _, err := m.store.Get(ctx, key)
	if err != nil {
		return err
	}
	defer reader.Close()
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(file, reader); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func (m *Manager) migrationState(ctx context.Context, database string) (string, error) {
	exists, err := m.commandOutput(
		ctx, database, "psql", "--no-psqlrc", "-At", "-v", "ON_ERROR_STOP=1",
		"-c", "SELECT to_regclass('public.schema_migrations') IS NOT NULL;",
	)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(exists) != "t" {
		return "uninitialized", nil
	}
	output, err := m.commandOutput(
		ctx, database, "psql", "--no-psqlrc", "-At", "-v", "ON_ERROR_STOP=1",
		"-c", "SELECT version::text || ':' || dirty::text FROM schema_migrations ORDER BY version DESC LIMIT 1;",
	)
	if err != nil {
		return "", err
	}
	state := strings.TrimSpace(output)
	if state == "" {
		return "", errors.New("empty migration state")
	}
	return state, nil
}

func (m *Manager) runDomainValidator(ctx context.Context, database string) (string, error) {
	if _, err := os.Stat(m.cfg.DomainValidatorPath); err != nil {
		return "", fmt.Errorf("domain validator unavailable at %s: %w", m.cfg.DomainValidatorPath, err)
	}
	command := exec.CommandContext(ctx, m.cfg.DomainValidatorPath)
	command.Env = append(os.Environ(), "DATABASE_URL="+m.databaseURL(database))
	output, err := command.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func (m *Manager) commandOutput(ctx context.Context, database, name string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, name, args...)
	command.Env = append(os.Environ(), m.postgresEnv(database)...)
	output, err := command.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%s %v: %w: %s", name, args, err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func (m *Manager) postgresEnv(database string) []string {
	return []string{
		"PGHOST=" + m.cfg.DBHost,
		"PGPORT=" + m.cfg.DBPort,
		"PGDATABASE=" + database,
		"PGUSER=" + m.cfg.DBUser,
		"PGPASSWORD=" + m.cfg.DBPassword,
		"PGSSLMODE=" + m.cfg.DBSSLMode,
	}
}

func (m *Manager) databaseURL(database string) string {
	u := &url.URL{Scheme: "postgresql", Host: m.cfg.DBHost + ":" + m.cfg.DBPort, Path: "/" + database}
	u.User = url.UserPassword(m.cfg.DBUser, m.cfg.DBPassword)
	query := u.Query()
	query.Set("sslmode", m.cfg.DBSSLMode)
	u.RawQuery = query.Encode()
	return u.String()
}

func sha256File(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}

func randomDatabaseName() (string, error) {
	random := make([]byte, 4)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return fmt.Sprintf("bodysense_dr_%s_%s", time.Now().UTC().Format("20060102_150405"), hex.EncodeToString(random)), nil
}
