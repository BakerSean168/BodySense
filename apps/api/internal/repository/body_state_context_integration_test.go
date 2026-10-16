package repository

import (
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"os"
	"testing"
)

func openBodyStateContextIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("BODYSENSE_INTEGRATION_DATABASE_URL")
	if dsn == "" {
		t.Skip("set BODYSENSE_INTEGRATION_DATABASE_URL to run BodyState context integration tests")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open integration database: %v", err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatalf("get integration database pool: %v", err)
	}
	// User fixtures register later and are removed before the pool closes.
	t.Cleanup(func() { _ = pool.Close() })
	return db
}

func seedBodyStateContextUser(t *testing.T, db *gorm.DB) uuid.UUID {
	t.Helper()
	userID := uuid.New()
	if err := db.Exec(
		"INSERT INTO users(id,email,password_hash) VALUES (?,?,?)",
		userID, "body-state-context-"+userID.String()+"@example.test", "synthetic-hash",
	).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() { _ = db.Exec("DELETE FROM users WHERE id = ?", userID).Error })
	return userID
}

// The baseline must have only native body persistence: no legacy read or write
// can silently survive by continuing to use an empty old table.
func TestBodyStateLegacyTablesAbsentPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	for _, table := range []string{"body_state_facts", "body_state_observations", "body_state_evidence"} {
		var exists bool
		if err := db.Raw("SELECT to_regclass(?) IS NOT NULL", "public."+table).Scan(&exists).Error; err != nil {
			t.Fatal(err)
		}
		if exists {
			t.Fatalf("retired persistence remains: %s", table)
		}
	}
}
