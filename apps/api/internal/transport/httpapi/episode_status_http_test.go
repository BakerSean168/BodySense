package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestBodyEpisodeHTTPStatusStrictUserEvidenceRoute(t *testing.T) {
	owner, episode, fact := uuid.New(), uuid.New(), uuid.New()
	fake := &episodeCommandHTTPFake{
		receipt: testEpisodeReceipt(owner, episode, fact, "status"),
	}
	router := episodeCommandRouter(t, owner, fake)
	uri := "/api/v1/body-state/episodes/" + episode.String() + "/status"
	input := fmt.Sprintf(`{"idempotency_key":"status-admission","expected_revision":2,"fact_id":%q}`, fact.String())
	result := performJSONAt(router, http.MethodPost, uri, input)
	if result.Code != http.StatusOK || fake.calls != 1 || fake.owner != owner ||
		fake.target != episode || fake.command.FactID != fact ||
		!strings.Contains(result.Body.String(), `"operation":"status"`) {
		t.Fatalf("no strict direct user Episode status route: %d %s calls=%d", result.Code, result.Body.String(), fake.calls)
	}
	// Source-based status evidence, not a second client-controlled clinical
	// state store. Extra inferred or forged fields must never reach Go.
	invalid := []string{
		strings.Replace(input, `"fact_id":`, `"claim":"explicit_resolution","fact_id":`, 1),
		strings.Replace(input, `"fact_id":`, `"state":"ended","fact_id":`, 1),
		strings.Replace(input, `"fact_id":`, `"source_evidence_id":"`+uuid.NewString()+`","fact_id":`, 1),
		strings.Replace(input, `"fact_id":`, `"user_id":"`+uuid.NewString()+`","fact_id":`, 1),
		strings.Replace(input, `"expected_revision":2,`, "", 1),
		strings.Replace(input, `"expected_revision":2,`, `"expected_revision":-1,`, 1),
	}
	for _, body := range invalid {
		res := performJSONAt(router, http.MethodPost, uri, body)
		if res.Code != http.StatusBadRequest || fake.calls != 1 {
			t.Fatalf("forged status[%s] reached service %d calls=%d", body, res.Code, fake.calls)
		}
	}
	fake.receipt = testEpisodeReceipt(owner, uuid.New(), fact, "status")
	if res := performJSONAt(router, http.MethodPost, uri, input); res.Code != http.StatusInternalServerError {
		t.Fatalf("different Episode forged a receipt: %d", res.Code)
	}
	if res := performJSONAt(episodeCommandRouter(t, uuid.Nil, fake), http.MethodPost, uri, input); res.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status accepted: %d", res.Code)
	}
}

func TestBodyEpisodeHTTPExplicitStatusAndRevocationPostgres(t *testing.T) {
	dsn := os.Getenv("BODYSENSE_INTEGRATION_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated PostgreSQL 18 v9 required")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	owner, other := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{owner, other} {
		if err := db.Exec("INSERT INTO users(id,email,password_hash) VALUES(?,?,?)", id, "episode-status-"+id.String()+"@example.test", "synthetic").Error; err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Exec("DELETE FROM users WHERE id=?", id).Error })
	}
	repo := repository.NewBodyStateRepository(db)
	record := func(statement, presence, claim string, expected int64, when string) uuid.UUID {
		t.Helper()
		details, _ := json.Marshal(model.SymptomFactDetails{SymptomType: "shoulder pain", Presence: presence, EpisodeCourseReport: claim, Laterality: "right"})
		cmd := model.RecordBodyFactsCommand{IdempotencyKey: uuid.NewString(), ExpectedRevision: expected, Facts: []model.BodyFactInput{{
			Kind: "symptom", Statement: statement, BodyRegionIDs: []string{},
			Temporal: model.FactTemporal{Precision: "day", Start: when}, Details: details,
		}}}
		receipt, err := repo.RecordDirectFacts(context.Background(), owner, cmd)
		if err != nil || receipt == nil {
			t.Fatalf("Fact save: %#v %v", receipt, err)
		}
		var saved model.BodyFact
		if err := db.Where("user_id=? AND statement=?", owner, statement).First(&saved).Error; err != nil {
			t.Fatal(err)
		}
		return saved.ID
	}
	current := record("The same shoulder pain is still ongoing", "present", "explicit_current_report", 0, "2026-10-12")
	router := episodeFullPostgresHTTPRouter(t, owner, repo)
	ep := episodePOSTJSON(t, router, "/api/v1/body-state/episodes", 1, "status-http-create", current)
	target := "/api/v1/body-state/episodes/" + ep.EpisodeID.String()
	statusURI := target + "/status"
	accepted := episodePOSTJSON(t, router, statusURI, 2, "status-http-current", current)
	if accepted.Operation != "status" || accepted.Revision != 3 {
		t.Fatalf("status receipt invalid: %#v", accepted)
	}
	detail := performJSONAt(router, http.MethodGet, target, "")
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `"state":"ongoing"`) ||
		strings.Contains(detail.Body.String(), "The same shoulder pain") {
		t.Fatalf("user ongoing report not derived or health source leaked: %d %s", detail.Code, detail.Body.String())
	}
	copied := episodePOSTJSON(t, router, statusURI, 2, "status-http-current", current)
	if copied.ID != accepted.ID || copied.Revision != 3 {
		t.Fatalf("HTTP replay not exact %#v", copied)
	}

	ordinary := record("I have no shoulder pain right now", "absent", "", 3, "2026-10-14")
	_ = episodePOSTJSON(t, router, target+"/facts", 4, "status-http-generic-link", ordinary)
	generic := fmt.Sprintf(`{"idempotency_key":"status-http-generic","expected_revision":5,"fact_id":%q}`, ordinary.String())
	res := performJSONAt(router, http.MethodPost, statusURI, generic)
	if res.Code != http.StatusConflict || !strings.Contains(res.Body.String(), "EPISODE_ASSOCIATION_CONFLICT") {
		t.Fatalf("generic negative became resolution: %d %s", res.Code, res.Body.String())
	}
	foreignRouter := episodeFullPostgresHTTPRouter(t, other, repo)
	foreign := performJSONAt(foreignRouter, http.MethodPost, statusURI, generic)
	if foreign.Code != http.StatusNotFound || strings.Contains(foreign.Body.String(), "shoulder") {
		t.Fatalf("cross-owner source or Episode identity leaked: %d %s", foreign.Code, foreign.Body.String())
	}
	if err := db.Model(&model.StatementEvidence{}).Where("user_id=? AND id=(SELECT source_evidence_id FROM body_facts WHERE id=?)", owner, current).
		Update("availability", "blocked").Error; err != nil {
		t.Fatal(err)
	}
	projection := performJSONAt(router, http.MethodGet, target, "")
	if projection.Code != http.StatusOK || strings.Contains(projection.Body.String(), `"state":"ongoing"`) ||
		strings.Contains(projection.Body.String(), `"state":"ended"`) {
		t.Fatalf("blocked original source still drove course: %d %s", projection.Code, projection.Body.String())
	}
}
