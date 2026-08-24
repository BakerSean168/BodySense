package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	preUploadStorageVersion = 54
	uploadStorageVersion    = 55
)

func main() {
	databaseURL := flag.String("database-url", "", "PostgreSQL URL for a disposable production-v29 fixture database")
	migrations := flag.String("migrations", "file://migrations", "golang-migrate source URL")
	expectReject := flag.Bool("expect-reject", false, "seed an unsafe legacy path and require migration 55 to fail closed")
	flag.Parse()
	if *databaseURL == "" {
		log.Fatal("-database-url is required")
	}

	m, err := migrate.New(*migrations, *databaseURL)
	if err != nil {
		log.Fatalf("create migrate instance: %v", err)
	}
	defer m.Close()
	if err := m.Migrate(preUploadStorageVersion); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("migrate fixture to version %d: %v", preUploadStorageVersion, err)
	}
	version, dirty, err := m.Version()
	if err != nil || dirty || version != preUploadStorageVersion {
		log.Fatalf("pre-upload-storage fixture mismatch: version=%d dirty=%v err=%v", version, dirty, err)
	}

	db, err := gorm.Open(postgres.Open(*databaseURL), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		log.Fatalf("open fixture database: %v", err)
	}
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	uploadID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	legacyPath := "uploads/" + userID.String() + "/photo.jpg"
	if *expectReject {
		legacyPath = "/tmp/private.jpg"
	}
	if err := db.Exec("INSERT INTO users(id,email,password_hash) VALUES (?,?,?)", userID, "upload-migration-validator@example.invalid", "validator").Error; err != nil {
		log.Fatalf("seed validator user: %v", err)
	}
	if err := db.Exec(`INSERT INTO user_uploads(id,user_id,file_type,original_name,file_path,file_size,mime_type,ocr_status)
		VALUES (?,?,?,?,?,?,?,?)`, uploadID, userID, "consultation_photo", "photo.jpg", legacyPath, 12, "image/jpeg", "pending").Error; err != nil {
		log.Fatalf("seed legacy upload: %v", err)
	}

	err = m.Migrate(uploadStorageVersion)
	if *expectReject {
		if err == nil || !strings.Contains(err.Error(), "cannot be safely converted") {
			log.Fatalf("unsafe legacy upload migration did not fail closed: %v", err)
		}
		fmt.Println("UPLOAD_STORAGE_UNSAFE_LEGACY_REJECT=PASS")
		return
	}
	if err != nil && err != migrate.ErrNoChange {
		log.Fatalf("migrate valid upload fixture to version %d: %v", uploadStorageVersion, err)
	}
	var identity string
	if err := db.Raw("SELECT storage_backend || '|' || storage_key FROM user_uploads WHERE id = ?", uploadID).Scan(&identity).Error; err != nil {
		log.Fatalf("read migrated upload identity: %v", err)
	}
	wantIdentity := "local|" + userID.String() + "/photo.jpg"
	if identity != wantIdentity {
		log.Fatalf("migrated upload identity mismatch: got=%q want=%q", identity, wantIdentity)
	}
	var legacyColumns int64
	if err := db.Raw("SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND table_name='user_uploads' AND column_name='file_path'").Scan(&legacyColumns).Error; err != nil {
		log.Fatalf("inspect migrated user_uploads: %v", err)
	}
	if legacyColumns != 0 {
		log.Fatalf("file_path authority survived migration 55")
	}
	fmt.Println("UPLOAD_STORAGE_LEGACY_BACKFILL=PASS")

	if err := m.Steps(-1); err != nil {
		log.Fatalf("downgrade migration 55 fixture: %v", err)
	}
	var restored string
	if err := db.Raw("SELECT file_path FROM user_uploads WHERE id = ?", uploadID).Scan(&restored).Error; err != nil {
		log.Fatalf("read downgraded upload path: %v", err)
	}
	wantRestored := "uploads/" + userID.String() + "/photo.jpg"
	if restored != wantRestored {
		log.Fatalf("downgraded upload path mismatch: got=%q want=%q", restored, wantRestored)
	}
	fmt.Println("UPLOAD_STORAGE_MIGRATION_DOWN=PASS")
}
