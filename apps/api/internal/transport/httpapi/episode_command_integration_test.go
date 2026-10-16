package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func episodeFullPostgresHTTPRouter(t *testing.T, owner uuid.UUID, repo *repository.BodyStateRepository) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	spec, err := openapiv1.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	application := NewPublicServer().
		WithEpisodes(service.NewEpisodeReadService(repo)).
		WithEpisodeCommands(service.NewEpisodeCommandService(repo))
	RegisterRoutes(router, StrictHandler(application), RouteSecurity{
		Auth: func(c *gin.Context) {
			if owner != uuid.Nil {
				c.Set("user_id", owner.String())
			}
			c.Next()
		},
		Operator:  func(c *gin.Context) { c.Next() },
		Validator: RequestValidator(spec),
	})
	return router
}
func episodePOSTJSON(t *testing.T, router *gin.Engine, uri string, expected int, key string, fact uuid.UUID) model.EpisodeCommandReceipt {
	t.Helper()
	body := fmt.Sprintf(`{"idempotency_key":%q,"expected_revision":%d,"fact_id":%q}`, key, expected, fact.String())
	result := performJSONAt(router, http.MethodPost, uri, body)
	if result.Code != http.StatusOK {
		t.Fatalf("Episode POST %s=%d: %s", uri, result.Code, result.Body.String())
	}
	var receipt model.EpisodeCommandReceipt
	if err := json.Unmarshal(result.Body.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.ID == uuid.Nil || receipt.EpisodeID == uuid.Nil || receipt.FactID != fact ||
		receipt.RevisionID == uuid.Nil || receipt.EventID == uuid.Nil {
		t.Fatalf("success without durable receipt: %#v", receipt)
	}
	return receipt
}
func TestBodyEpisodeHTTPExplicitUserCreateAndLinkPostgres(t *testing.T) {
	dsn := os.Getenv("BODYSENSE_INTEGRATION_DATABASE_URL")
	if dsn == "" {
		t.Skip("needs isolated native PostgreSQL v8, not live env")
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
		if err := db.Exec("INSERT INTO users(id,email,password_hash) VALUES(?,?,?)",
			id, "episode-command-"+id.String()+"@example.test", "synthetic").Error; err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Exec("DELETE FROM users WHERE id=?", id).Error; err != nil {
				t.Errorf("delete isolated user: %v", err)
			}
		})
	}
	repo := repository.NewBodyStateRepository(db)
	ctx := context.Background()
	facts := func(owner uuid.UUID, rev int64, statement string) (model.BodyFact, model.BodyFactCommit) {
		t.Helper()
		input := model.RecordBodyFactsCommand{IdempotencyKey: uuid.NewString(), ExpectedRevision: rev,
			Facts: []model.BodyFactInput{{
				Kind: "symptom", Statement: statement, BodyRegionIDs: []string{},
				Temporal: model.FactTemporal{Precision: "day", Start: "2026-10-13"},
				Details:  json.RawMessage(`{"symptom_type":"shoulder discomfort","presence":"present","laterality":"right"}`),
			}},
		}
		receipt, err := repo.RecordDirectFacts(ctx, owner, input)
		if err != nil {
			t.Fatal(err)
		}
		var fact model.BodyFact
		if err := db.Where("user_id=? AND statement=?", owner, statement).First(&fact).Error; err != nil {
			t.Fatal(err)
		}
		return fact, *receipt
	}
	first, recorded := facts(owner, 0, "I feel right shoulder discomfort")
	if recorded.Revision != 1 {
		t.Fatal("unexpected initial revision")
	}
	router := episodeFullPostgresHTTPRouter(t, owner, repo)
	saved := episodePOSTJSON(t, router, "/api/v1/body-state/episodes", 1, "create-episode", first.ID)
	if saved.UserID != owner || saved.Revision != 2 || saved.Operation != "create" {
		t.Fatalf("create=%#v", saved)
	}
	replay := episodePOSTJSON(t, router, "/api/v1/body-state/episodes", 1, "create-episode", first.ID)
	if replay.ID != saved.ID || replay.EpisodeID != saved.EpisodeID || replay.Revision != 2 {
		t.Fatalf("not exact idempotent replay: %#v vs %#v", replay, saved)
	}
	detail := performJSONAt(router, http.MethodGet, "/api/v1/body-state/episodes/"+saved.EpisodeID.String(), "")
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `"state":"unknown"`) ||
		strings.Contains(detail.Body.String(), `"state":"ended"`) ||
		strings.Contains(detail.Body.String(), first.Statement) {
		t.Fatalf("creation inferred medical state or leaked source: %d %s", detail.Code, detail.Body.String())
	}
	second, _ := facts(owner, 2, "Again, right shoulder discomfort today")
	linkURL := "/api/v1/body-state/episodes/" + saved.EpisodeID.String() + "/facts"
	linked := episodePOSTJSON(t, router, linkURL, 3, "link-second", second.ID)
	if linked.Revision != 4 || linked.Operation != "link" || linked.EpisodeID != saved.EpisodeID {
		t.Fatalf("bad durable link: %#v", linked)
	}
	listing := performJSONAt(router, http.MethodGet, "/api/v1/body-state/episodes", "")
	if listing.Code != http.StatusOK || !strings.Contains(listing.Body.String(), `"current_revision":4`) ||
		strings.Count(listing.Body.String(), `"role":"observation"`) != 2 ||
		!strings.Contains(listing.Body.String(), `"state":"unknown"`) {
		t.Fatalf("actual fact→episode status path failed: %d %s", listing.Code, listing.Body.String())
	}
	foreignRouter := episodeFullPostgresHTTPRouter(t, foreign, repo)
	foreignDetails := performJSONAt(foreignRouter, http.MethodGet, "/api/v1/body-state/episodes/"+saved.EpisodeID.String(), "")
	if foreignDetails.Code != http.StatusNotFound || strings.Contains(foreignDetails.Body.String(), "shoulder") {
		t.Fatalf("foreign user could read Episode: %d %s", foreignDetails.Code, foreignDetails.Body.String())
	}
	userCommand := fmt.Sprintf(`{"idempotency_key":"foreign-create","expected_revision":0,"fact_id":%q}`, first.ID.String())
	forbidden := performJSONAt(foreignRouter, http.MethodPost, "/api/v1/body-state/episodes", userCommand)
	if forbidden.Code != http.StatusNotFound || strings.Contains(forbidden.Body.String(), "shoulder") {
		t.Fatalf("other user's Fact was admitted: %d %s", forbidden.Code, forbidden.Body.String())
	}

	for _, fact := range []model.BodyFact{first, second} {
		if err := db.Model(&model.StatementEvidence{}).Where("id=? AND user_id=?", fact.SourceEvidenceID, owner).
			Update("availability", "blocked").Error; err != nil {
			t.Fatal(err)
		}
	}
	blocked := performJSONAt(router, http.MethodGet, "/api/v1/body-state/episodes/"+saved.EpisodeID.String(), "")
	if blocked.Code != http.StatusNotFound || strings.Contains(blocked.Body.String(), "shoulder") {
		t.Fatalf("blocked original user health source leaked: %d %s", blocked.Code, blocked.Body.String())
	}
	page := performJSONAt(router, http.MethodGet, "/api/v1/body-state/episodes", "")
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), `"episodes":[]`) {
		t.Fatalf("blocked all sources did not hide Episode: %d %s", page.Code, page.Body.String())
	}
}
