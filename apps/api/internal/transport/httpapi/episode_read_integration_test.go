package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestBodyEpisodeHTTPSourceVisibilityRoundTripPostgres(t *testing.T) {
	dsn := os.Getenv("BODYSENSE_INTEGRATION_DATABASE_URL")
	if dsn == "" {
		t.Skip("set BODYSENSE_INTEGRATION_DATABASE_URL for real PostgreSQL Episode HTTP reads")
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
	for _, id := range []uuid.UUID{owner, other} {
		if err := db.Exec("INSERT INTO users(id,email,password_hash) VALUES(?,?,?)",
			id, "episode-http-"+id.String()+"@example.test", "synthetic").Error; err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Exec("DELETE FROM users WHERE id=?", id).Error; err != nil {
				t.Errorf("delete seeded owner: %v", err)
			}
		})
	}

	repo := repository.NewBodyStateRepository(db)
	sourceText := "My shoulder episode has completely ended"
	command := model.RecordBodyFactsCommand{
		IdempotencyKey: "episode-http-source", ExpectedRevision: 0,
		Facts: []model.BodyFactInput{{
			Kind: "symptom", Statement: sourceText, BodyRegionIDs: []string{},
			Temporal: model.FactTemporal{Precision: "day", Start: "2026-10-13"},
			Details:  json.RawMessage(`{"symptom_type":"shoulder pain","presence":"absent"}`),
		}},
	}
	receipt, err := repo.RecordDirectFacts(context.Background(), owner, command)
	if err != nil {
		t.Fatal(err)
	}
	var fact model.BodyFact
	if err := db.Where("user_id = ? AND kind = ?", owner, "symptom").First(&fact).Error; err != nil {
		t.Fatal(err)
	}
	episode := uuid.New()
	// Synthetic accepted evidence is inserted below only to test the
	// read transport; production association writes remain separately gated.
	if err := db.Exec(`INSERT INTO public.symptom_episodes
        (id,user_id,symptom_type,body_region_ids,laterality,identity_status,created_revision,created_at)
        VALUES (?,?,?,?::jsonb,'unknown','current',?,?)`,
		episode, owner, "shoulder pain", "[]", receipt.Revision, time.Now().UTC()).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO public.episode_fact_links
        (user_id,episode_id,fact_id,fact_kind,basis_evidence_id,role,association_method,is_primary,created_revision)
        VALUES (?,?,?,'symptom',?,'resolution','explicit',true,?)`,
		owner, episode, fact.ID, fact.SourceEvidenceID, receipt.Revision).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO public.episode_state_evidence
        (user_id,episode_id,fact_id,source_evidence_id,claim,as_of,created_revision)
        VALUES (?,?,?,?,?,?::jsonb,?)`,
		owner, episode, fact.ID, fact.SourceEvidenceID, "explicit_resolution",
		string(fact.Temporal), receipt.Revision).Error; err != nil {
		t.Fatal(err)
	}

	app := service.NewEpisodeReadService(repo)
	router := episodeHTTPRouter(t, owner, app)
	listed := performJSONAt(router, http.MethodGet, "/api/v1/body-state/episodes", "")
	if listed.Code != http.StatusOK ||
		!strings.Contains(listed.Body.String(), episode.String()) ||
		!strings.Contains(listed.Body.String(), `"state":"ended"`) ||
		strings.Contains(listed.Body.String(), sourceText) {
		t.Fatalf("actual SQL-to-generated-public page status=%d body=%s", listed.Code, listed.Body.String())
	}
	detail := performJSONAt(router, http.MethodGet, "/api/v1/body-state/episodes/"+episode.String(), "")
	if detail.Code != http.StatusOK ||
		!strings.Contains(detail.Body.String(), `"state_as_of":"2026-10-13"`) {
		t.Fatalf("source-backed detail status=%d body=%s", detail.Code, detail.Body.String())
	}
	foreign := performJSONAt(episodeHTTPRouter(t, other, app), http.MethodGet, "/api/v1/body-state/episodes/"+episode.String(), "")
	if foreign.Code != http.StatusNotFound || strings.Contains(foreign.Body.String(), "shoulder") {
		t.Fatalf("foreign owner received health data: %d %s", foreign.Code, foreign.Body.String())
	}
	if err := db.Model(&model.StatementEvidence{}).Where("user_id = ? AND id = ?", owner, fact.SourceEvidenceID).
		Update("availability", "blocked").Error; err != nil {
		t.Fatal(err)
	}
	blocked := performJSONAt(router, http.MethodGet, "/api/v1/body-state/episodes/"+episode.String(), "")
	if blocked.Code != http.StatusNotFound || strings.Contains(blocked.Body.String(), "shoulder") {
		t.Fatalf("source-blocked detail leaked: %d %s", blocked.Code, blocked.Body.String())
	}
	empty := performJSONAt(router, http.MethodGet, "/api/v1/body-state/episodes", "")
	if empty.Code != http.StatusOK || !strings.Contains(empty.Body.String(), `"episodes":[]`) ||
		strings.Contains(empty.Body.String(), "shoulder") {
		t.Fatalf("source-blocked list leaked: %d %s", empty.Code, empty.Body.String())
	}
}
