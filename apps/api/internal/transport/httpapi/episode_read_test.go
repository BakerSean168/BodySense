package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type episodeReadHTTPFake struct {
	owner     uuid.UUID
	target    uuid.UUID
	before    int64
	calls     int
	page      *model.EpisodePage
	record    *model.EpisodeRecord
	relations *model.EpisodeRelationPage
	err       error
}

func (f *episodeReadHTTPFake) ListEpisodes(_ context.Context, owner uuid.UUID, before int64) (*model.EpisodePage, error) {
	f.owner = owner
	f.before = before
	f.calls++
	return f.page, f.err
}
func (f *episodeReadHTTPFake) GetEpisode(_ context.Context, owner, id uuid.UUID) (*model.EpisodeRecord, error) {
	f.owner = owner
	f.target = id
	f.calls++
	return f.record, f.err
}

func (f *episodeReadHTTPFake) ListEpisodeRelations(_ context.Context, owner, id uuid.UUID, before int64) (*model.EpisodeRelationPage, error) {
	f.owner = owner
	f.target = id
	f.before = before
	f.calls++
	return f.relations, f.err
}

func episodeHTTPRouter(t *testing.T, owner uuid.UUID, source episodeReadApplication) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	spec, err := openapiv1.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	RegisterRoutes(router, StrictHandler(NewPublicServer().WithEpisodes(source)), RouteSecurity{
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

func episodeHTTPRecord(owner uuid.UUID) *model.EpisodeRecord {
	return &model.EpisodeRecord{
		ID: uuid.New(), UserID: owner, SymptomType: "self-reported shoulder discomfort",
		BodyRegionIDs: datatypes.JSON(`[]`), Laterality: "unknown",
		IdentityStatus: "current", CreatedRevision: 1, CreatedAt: time.Now().UTC(),
		Links: []model.EpisodeFactReference{{
			FactID: uuid.New(), Role: "onset", AssociationMethod: "explicit", IsPrimary: true, LinkedRevision: 1,
		}},
		Projection: model.EpisodeStateProjection{
			State: "unknown", Reason: "insufficient_evidence",
			BasisFactIDs: []uuid.UUID{}, BasedOnRevision: 1,
		},
	}
}

func TestBodyEpisodeHTTPAuthenticatedGeneratedReadContracts(t *testing.T) {
	owner := uuid.New()
	record := episodeHTTPRecord(owner)
	fake := &episodeReadHTTPFake{
		page:   &model.EpisodePage{UserID: owner, CurrentRevision: 1, Episodes: []model.EpisodeRecord{*record}},
		record: record,
	}
	router := episodeHTTPRouter(t, owner, fake)
	list := performJSONAt(router, http.MethodGet, "/api/v1/body-state/episodes", "")
	if list.Code != http.StatusOK || fake.calls != 1 || fake.owner != owner ||
		!strings.Contains(list.Body.String(), record.ID.String()) ||
		strings.Contains(list.Body.String(), "source_excerpt") {
		t.Fatalf("read status=%d body=%s fake=%#v", list.Code, list.Body.String(), fake)
	}
	one := performJSONAt(router, http.MethodGet, "/api/v1/body-state/episodes/"+record.ID.String(), "")
	if one.Code != http.StatusOK || fake.calls != 2 || fake.target != record.ID ||
		!strings.Contains(one.Body.String(), `"state":"unknown"`) {
		t.Fatalf("detail status=%d body=%s fake=%#v", one.Code, one.Body.String(), fake)
	}
	// This slice is read-only. A public POST must not grant AI or other
	// clients the ability to create an unverified clinical Episode.
	write := performJSONAt(router, http.MethodPost, "/api/v1/body-state/episodes", `{}`)
	if write.Code == http.StatusOK || write.Code == http.StatusCreated || fake.calls != 2 {
		t.Fatalf("unsupported write was accepted: %d calls=%d", write.Code, fake.calls)
	}
}

func TestBodyEpisodeHTTPFailsClosedForUntrustedInputsAndResponse(t *testing.T) {
	owner := uuid.New()
	record := episodeHTTPRecord(owner)
	fake := &episodeReadHTTPFake{
		page:   &model.EpisodePage{UserID: owner, CurrentRevision: 1, Episodes: []model.EpisodeRecord{*record}},
		record: record,
	}
	router := episodeHTTPRouter(t, owner, fake)
	for _, uri := range []string{
		"/api/v1/body-state/episodes?before_revision=-1",
		"/api/v1/body-state/episodes?before_revision=garbage",
		"/api/v1/body-state/episodes/not-a-uuid",
	} {
		result := performJSONAt(router, http.MethodGet, uri, "")
		if result.Code != http.StatusBadRequest || fake.calls != 0 {
			t.Fatalf("untrusted cursor/id reached app: %s status=%d calls=%d", uri, result.Code, fake.calls)
		}
	}

	fake.page.UserID = uuid.New()
	denied := performJSONAt(router, http.MethodGet, "/api/v1/body-state/episodes", "")
	if denied.Code != http.StatusInternalServerError || strings.Contains(denied.Body.String(), record.SymptomType) {
		t.Fatalf("wrong-user page leaked: %d %s", denied.Code, denied.Body.String())
	}
	fake.page.UserID = owner
	fake.page.Episodes[0].UserID = uuid.New()
	denied = performJSONAt(router, http.MethodGet, "/api/v1/body-state/episodes", "")
	if denied.Code != http.StatusInternalServerError {
		t.Fatalf("foreign nested episode accepted: %d", denied.Code)
	}
	fake.page.Episodes[0].UserID = owner
	fake.page.Episodes[0].Projection.State = "ended"
	fake.page.Episodes[0].Projection.Reason = "insufficient_evidence"
	denied = performJSONAt(router, http.MethodGet, "/api/v1/body-state/episodes", "")
	if denied.Code != http.StatusInternalServerError {
		t.Fatalf("false ended episode accepted: %d", denied.Code)
	}
	fake.page.Episodes[0].Projection.State = "unknown"
	fake.page.Episodes[0].Projection.Reason = "insufficient_evidence"
	fake.record.UserID = uuid.New()
	denied = performJSONAt(router, http.MethodGet, "/api/v1/body-state/episodes/"+record.ID.String(), "")
	if denied.Code != http.StatusInternalServerError {
		t.Fatalf("foreign detail accepted: %d", denied.Code)
	}
	fake.record.UserID = owner
	fake.err = errors.New("private statement: do not send")
	denied = performJSONAt(router, http.MethodGet, "/api/v1/body-state/episodes", "")
	if denied.Code != http.StatusInternalServerError || strings.Contains(denied.Body.String(), "private statement") {
		t.Fatalf("internal error leaked raw health text: %d %s", denied.Code, denied.Body.String())
	}
}

func TestBodyEpisodeHTTPUnauthenticatedAndSourceBlockedNotFound(t *testing.T) {
	owner := uuid.New()
	fake := &episodeReadHTTPFake{err: repository.ErrEpisodeNotFound}
	denied := performJSONAt(episodeHTTPRouter(t, uuid.Nil, fake), http.MethodGet, "/api/v1/body-state/episodes", "")
	if denied.Code != http.StatusUnauthorized || fake.calls != 0 {
		t.Fatalf("anonymous read: %d", denied.Code)
	}
	id := uuid.New()
	result := performJSONAt(episodeHTTPRouter(t, owner, fake), http.MethodGet, "/api/v1/body-state/episodes/"+id.String(), "")
	if result.Code != http.StatusNotFound || !strings.Contains(result.Body.String(), "EPISODE_NOT_FOUND") ||
		fake.owner != owner || fake.target != id || strings.Contains(result.Body.String(), "symptom") {
		t.Fatalf("blocked/missing identity leak: %d %s", result.Code, result.Body.String())
	}
}
