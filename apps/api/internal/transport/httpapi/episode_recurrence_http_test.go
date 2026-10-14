package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func validRecurrenceHTTPReceipt(owner, old, newer, fact uuid.UUID) *model.EpisodeRecurrenceReceipt {
	return &model.EpisodeRecurrenceReceipt{ID: uuid.New(), UserID: owner,
		RelationID: uuid.New(), FromEpisodeID: old, ToEpisodeID: newer,
		FactID: fact, Revision: 10, RevisionID: uuid.New(), EventID: uuid.New(), CommittedAt: time.Now().UTC()}
}
func TestBodyEpisodeHTTPRecurrenceStrictContractAndForgedReceipt(t *testing.T) {
	owner, old, newer, fact := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	fake := &episodeCommandHTTPFake{recurrenceReceipt: validRecurrenceHTTPReceipt(owner, old, newer, fact)}
	router := episodeCommandRouter(t, owner, fake)
	path := "/api/v1/body-state/episodes/" + old.String() + "/recurrences"
	body := fmt.Sprintf(`{"idempotency_key":"explicit-recurrence","expected_revision":9,"new_episode_id":%q,"fact_id":%q}`, newer, fact)
	response := performJSONAt(router, http.MethodPost, path, body)
	if response.Code != http.StatusOK || fake.calls != 1 || fake.owner != owner ||
		fake.target != old || fake.recurrenceCommand.NewEpisodeID != newer ||
		fake.recurrenceCommand.FactID != fact ||
		!strings.Contains(response.Body.String(), `"relation_id"`) ||
		strings.Contains(response.Body.String(), "statement") || strings.Contains(response.Body.String(), "fingerprint") {
		t.Fatalf("not an exact privacy-safe direct-user relation receipt: %d %s", response.Code, response.Body.String())
	}
	invalid := []string{
		strings.Replace(body, `"fact_id":`, `"user_id":"`+uuid.NewString()+`","fact_id":`, 1),
		strings.Replace(body, `"fact_id":`, `"state":"ended","fact_id":`, 1),
		strings.Replace(body, `"fact_id":`, `"clinical_etiology":"one disease","fact_id":`, 1),
		strings.Replace(body, `"fact_id":`, `"authorization_id":"`+uuid.NewString()+`","fact_id":`, 1),
		strings.Replace(body, `"new_episode_id":`, `"consent":true,"new_episode_id":`, 1),
		strings.Replace(body, `"expected_revision":9,`, "", 1),
		strings.Replace(body, `"expected_revision":9,`, `"expected_revision":-1,`, 1),
		strings.Replace(body, newer.String(), "not-a-uuid", 1),
		strings.Replace(body, newer.String(), old.String(), 1),
	}
	for i, raw := range invalid {
		res := performJSONAt(router, http.MethodPost, path, raw)
		if res.Code != http.StatusBadRequest || fake.calls != 1 {
			t.Fatalf("forged recurrence[%d] admitted: %d %s", i, res.Code, res.Body.String())
		}
	}
	fake.recurrenceReceipt = validRecurrenceHTTPReceipt(owner, uuid.New(), newer, fact)
	if res := performJSONAt(router, http.MethodPost, path, body); res.Code != http.StatusInternalServerError {
		t.Fatalf("forged old Episode receipt treated as success: %d", res.Code)
	}
	fake.recurrenceReceipt = validRecurrenceHTTPReceipt(uuid.New(), old, newer, fact)
	if res := performJSONAt(router, http.MethodPost, path, body); res.Code != http.StatusInternalServerError {
		t.Fatalf("forged owner receipt treated as success: %d", res.Code)
	}
	fake.recurrenceReceipt = nil
	fake.err = model.ErrEpisodeAssociationConflict
	if res := performJSONAt(router, http.MethodPost, path, body); res.Code != http.StatusConflict {
		t.Fatalf("domain conflict incorrectly reported: %d %s", res.Code, res.Body.String())
	}
	fake.err = errors.New("untrusted private health info")
	if res := performJSONAt(router, http.MethodPost, path, body); res.Code != http.StatusInternalServerError ||
		strings.Contains(res.Body.String(), "private health info") {
		t.Fatalf("private backend error disclosed: %d %s", res.Code, res.Body.String())
	}
	if res := performJSONAt(episodeCommandRouter(t, uuid.Nil, fake), http.MethodPost, path, body); res.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated recurrence write accepted: %d", res.Code)
	}
}

func TestBodyEpisodeHTTPRecurrenceSourceReadAndResponseValidation(t *testing.T) {
	owner, old, newer, fact := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	relation := model.EpisodeRelationReference{ID: uuid.New(), FromEpisodeID: old,
		ToEpisodeID: newer, Kind: "recurrence_reported", BasisFactID: fact,
		AuthorizationID: uuid.New(), CreatedRevision: 10, ReviewState: "needs_review"}
	fake := &episodeReadHTTPFake{
		relations: &model.EpisodeRelationPage{UserID: owner, EpisodeID: old, CurrentRevision: 11,
			Relations: []model.EpisodeRelationReference{relation}},
	}
	router := episodeHTTPRouter(t, owner, fake)
	path := "/api/v1/body-state/episodes/" + old.String() + "/relations"
	got := performJSONAt(router, http.MethodGet, path, "")
	if got.Code != http.StatusOK || fake.calls != 1 || fake.target != old ||
		!strings.Contains(got.Body.String(), `"review_state":"needs_review"`) {
		t.Fatalf("source review projection dropped: %d %s", got.Code, got.Body.String())
	}
	fake.relations.EpisodeID = uuid.New()
	if res := performJSONAt(router, http.MethodGet, path, ""); res.Code != http.StatusInternalServerError {
		t.Fatalf("foreign target page admitted: %d", res.Code)
	}
	fake.relations.EpisodeID = old
	fake.relations.Relations[0].ToEpisodeID = uuid.New()
	fake.relations.Relations[0].FromEpisodeID = uuid.New()
	if res := performJSONAt(router, http.MethodGet, path, ""); res.Code != http.StatusInternalServerError {
		t.Fatalf("unrelated relation leaked: %d", res.Code)
	}
	if res := performJSONAt(router, http.MethodGet, path+"?before_revision=0", ""); res.Code != http.StatusBadRequest {
		t.Fatalf("invalid cursor accepted: %d", res.Code)
	}
	if res := performJSONAt(episodeHTTPRouter(t, uuid.Nil, fake), http.MethodGet, path, ""); res.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous relation read accepted: %d", res.Code)
	}
}

func TestBodyEpisodeHTTPRecurrenceRealPostgresAndSourceRetraction(t *testing.T) {
	dsn := os.Getenv("BODYSENSE_INTEGRATION_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires isolated v10 native PostgreSQL")
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
	owner, foreign := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{owner, foreign} {
		if err := db.Exec("INSERT INTO users(id,email,password_hash) VALUES(?,?,?)", id, "recurrence-http-"+id.String()+"@example.test", "synthetic").Error; err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Exec("DELETE FROM users WHERE id=?", id).Error })
	}
	repo := repository.NewBodyStateRepository(db)
	router := episodeFullPostgresHTTPRouter(t, owner, repo)
	ctx := context.Background()
	record := func(rev int64, statement, presence, claim, date string) uuid.UUID {
		t.Helper()
		data, _ := json.Marshal(model.SymptomFactDetails{SymptomType: "shoulder pain", Presence: presence, EpisodeCourseReport: claim, Laterality: "right"})
		command := model.RecordBodyFactsCommand{IdempotencyKey: uuid.NewString(), ExpectedRevision: rev, Facts: []model.BodyFactInput{{
			Kind: "symptom", Statement: statement, BodyRegionIDs: []string{},
			Temporal: model.FactTemporal{Precision: "day", Start: date}, Details: data,
		}}}
		commit, err := repo.RecordDirectFacts(ctx, owner, command)
		if err != nil || commit == nil {
			t.Fatalf("record basis failed: %#v %v", commit, err)
		}
		var fact model.BodyFact
		if err := db.Where("user_id=? AND statement=?", owner, statement).First(&fact).Error; err != nil {
			t.Fatal(err)
		}
		return fact.ID
	}
	oldPositive := record(0, "Earlier occurrence of pain", "present", "explicit_current_report", "2026-10-01")
	old := episodePOSTJSON(t, router, "/api/v1/body-state/episodes", 1, uuid.NewString(), oldPositive)
	episodePOSTJSON(t, router, "/api/v1/body-state/episodes/"+old.EpisodeID.String()+"/status", 2, uuid.NewString(), oldPositive)
	oldResolved := record(3, "First occurrence fully ended", "absent", "explicit_resolution", "2026-10-04")
	episodePOSTJSON(t, router, "/api/v1/body-state/episodes/"+old.EpisodeID.String()+"/facts", 4, uuid.NewString(), oldResolved)
	episodePOSTJSON(t, router, "/api/v1/body-state/episodes/"+old.EpisodeID.String()+"/status", 5, uuid.NewString(), oldResolved)
	newPositive := record(6, "New distinct pain occurrence", "present", "explicit_current_report", "2026-10-08")
	next := episodePOSTJSON(t, router, "/api/v1/body-state/episodes", 7, uuid.NewString(), newPositive)
	episodePOSTJSON(t, router, "/api/v1/body-state/episodes/"+next.EpisodeID.String()+"/status", 8, uuid.NewString(), newPositive)
	route := "/api/v1/body-state/episodes/" + old.EpisodeID.String() + "/recurrences"
	raw := fmt.Sprintf(`{"idempotency_key":"real-http-recurrence","expected_revision":9,"new_episode_id":%q,"fact_id":%q}`,
		next.EpisodeID.String(), newPositive.String())
	res := performJSONAt(router, http.MethodPost, route, raw)
	if res.Code != http.StatusOK {
		t.Fatalf("real recurrence HTTP POST=%d %s", res.Code, res.Body.String())
	}
	var authorized model.EpisodeRecurrenceReceipt
	if err := json.Unmarshal(res.Body.Bytes(), &authorized); err != nil {
		t.Fatal(err)
	}
	if authorized.UserID != owner || authorized.RelationID == uuid.Nil || authorized.FromEpisodeID != old.EpisodeID ||
		authorized.ToEpisodeID != next.EpisodeID || authorized.Revision != 10 {
		t.Fatalf("receipt does not bind exact identities: %#v", authorized)
	}
	replay := performJSONAt(router, http.MethodPost, route, raw)
	if replay.Code != http.StatusOK || !strings.Contains(replay.Body.String(), authorized.RelationID.String()) {
		t.Fatalf("HTTP retry duplicated recurrence: %d %s", replay.Code, replay.Body.String())
	}
	oldRelations := "/api/v1/body-state/episodes/" + old.EpisodeID.String() + "/relations"
	got := performJSONAt(router, http.MethodGet, oldRelations, "")
	if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), authorized.RelationID.String()) ||
		!strings.Contains(got.Body.String(), `"review_state":"supported"`) ||
		strings.Contains(got.Body.String(), "Earlier occurrence") {
		t.Fatalf("source-backed read not private/supported: %d %s", got.Code, got.Body.String())
	}
	newRead := performJSONAt(router, http.MethodGet, "/api/v1/body-state/episodes/"+next.EpisodeID.String()+"/relations", "")
	if newRead.Code != http.StatusOK || !strings.Contains(newRead.Body.String(), authorized.RelationID.String()) {
		t.Fatalf("new Episode cannot view its own recurrence: %d %s", newRead.Code, newRead.Body.String())
	}
	foreignRouter := episodeFullPostgresHTTPRouter(t, foreign, repo)
	if foreignRead := performJSONAt(foreignRouter, http.MethodGet, oldRelations, ""); foreignRead.Code != http.StatusNotFound {
		t.Fatalf("foreign read disclosed history: %d", foreignRead.Code)
	}
	if foreignWrite := performJSONAt(foreignRouter, http.MethodPost, route, raw); foreignWrite.Code != http.StatusNotFound {
		t.Fatalf("foreign recurrence created or disclosed: %d %s", foreignWrite.Code, foreignWrite.Body.String())
	}
	if err := db.Model(&model.BodyFact{}).Where("user_id=? AND id=?", owner, oldResolved).Update("assertion_status", "withdrawn").Error; err != nil {
		t.Fatal(err)
	}
	needsReview := performJSONAt(router, http.MethodGet, oldRelations, "")
	if needsReview.Code != http.StatusOK || !strings.Contains(needsReview.Body.String(), `"review_state":"needs_review"`) {
		t.Fatalf("ended source withdrawal did not downgrade relation: %d %s", needsReview.Code, needsReview.Body.String())
	}
	var source model.BodyFact
	if err := db.Where("id=? AND user_id=?", newPositive, owner).First(&source).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.StatementEvidence{}).Where("user_id=? AND id=?", owner, source.SourceEvidenceID).
		Update("availability", "blocked").Error; err != nil {
		t.Fatal(err)
	}
	hidden := performJSONAt(router, http.MethodGet, oldRelations, "")
	if hidden.Code != http.StatusOK || strings.Contains(hidden.Body.String(), authorized.RelationID.String()) {
		t.Fatalf("blocked original still yielded link: %d %s", hidden.Code, hidden.Body.String())
	}
}
