package httpapi

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// This test crosses the actual route/auth-context/validator/generated DTO/domain
// service/transaction/PostgreSQL/read-presenter boundary, not a fake success stub.
func TestBodyFactHTTPRecordReadRetryRoundTripPostgres(t *testing.T) {
	dsn := os.Getenv("BODYSENSE_INTEGRATION_DATABASE_URL")
	if dsn == "" {
		t.Skip("set BODYSENSE_INTEGRATION_DATABASE_URL for real PostgreSQL HTTP round trips")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	owner, other := uuid.New(), uuid.New()
	for _, userID := range []uuid.UUID{owner, other} {
		if err := db.Exec("INSERT INTO users(id,email,password_hash) VALUES (?,?,?)", userID, "fact-http-"+userID.String()+"@example.test", "synthetic-test-hash").Error; err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Exec("DELETE FROM users WHERE id = ?", userID).Error; err != nil {
				t.Errorf("delete synthetic user: %v", err)
			}
		})
	}
	application := service.NewBodyFactService(repository.NewBodyStateRepository(db), service.NewCanonicalBodyRegionIDValidator())
	r := bodyFactHTTPRouter(t, owner, application)
	response := performJSONAt(r, http.MethodPost, "/api/v1/body-state/records", bodyFactRequestJSON)
	if response.Code != http.StatusOK {
		t.Fatalf("record status=%d body=%s", response.Code, response.Body.String())
	}
	var first model.BodyFactCommit
	if err := json.Unmarshal(response.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	if first.Revision != 1 || first.UserID != owner || first.ID == uuid.Nil {
		t.Fatalf("receipt=%#v", first)
	}
	read := performJSONAt(r, http.MethodGet, "/api/v1/body-state/records", "")
	if read.Code != http.StatusOK {
		t.Fatalf("read status=%d body=%s", read.Code, read.Body.String())
	}
	var page model.BodyFactPage
	if err := json.Unmarshal(read.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Facts) != 1 || page.Facts[0].Statement != "My shoulder hurts." || page.CurrentRevision != 1 {
		t.Fatalf("roundtrip=%#v", page)
	}
	retired := performJSONAt(r, http.MethodGet, "/api/v1/body-state", "")
	if retired.Code != http.StatusNotFound {
		t.Fatalf("retired aggregate route status=%d", retired.Code)
	}

	var source model.StatementEvidence
	if err := db.First(&source, "id = ?", page.Facts[0].SourceEvidenceID).Error; err != nil {
		t.Fatal(err)
	}
	if source.SourceKind != "direct_edit" || source.UserID != owner {
		t.Fatal("untrusted source identity")
	}
	retry := performJSONAt(r, http.MethodPost, "/api/v1/body-state/records", bodyFactRequestJSON)
	var replay model.BodyFactCommit
	if err := json.Unmarshal(retry.Body.Bytes(), &replay); err != nil {
		t.Fatal(err)
	}
	if retry.Code != http.StatusOK || replay.ID != first.ID || replay.EventID != first.EventID {
		t.Fatal("HTTP retry did not preserve the original committed identity")
	}
	changed := strings.Replace(bodyFactRequestJSON, "My shoulder hurts.", "Different statement.", 1)
	conflict := performJSONAt(r, http.MethodPost, "/api/v1/body-state/records", changed)
	if conflict.Code != http.StatusConflict {
		t.Fatalf("changed idempotent request status=%d", conflict.Code)
	}
	otherRead := performJSONAt(bodyFactHTTPRouter(t, other, application), http.MethodGet, "/api/v1/body-state/records", "")
	if otherRead.Code != http.StatusOK || strings.Contains(otherRead.Body.String(), "My shoulder hurts.") {
		t.Fatal("cross-user response leaked private statement")
	}
}
