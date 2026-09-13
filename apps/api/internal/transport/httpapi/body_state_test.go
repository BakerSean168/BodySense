package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeBodyStateFactService struct {
	calls            int
	expectedRevision *int64
	fact             model.BodyStateFact
	resultFact       *model.BodyStateFact
	resultRevision   *model.BodyStateRevision
	err              error
}

func (f *fakeBodyStateFactService) UpsertFact(
	_ context.Context,
	_ uuid.UUID,
	expectedRevision *int64,
	fact model.BodyStateFact,
) (*model.BodyStateFact, *model.BodyStateRevision, error) {
	f.calls++
	f.expectedRevision = expectedRevision
	f.fact = fact
	return f.resultFact, f.resultRevision, f.err
}

func newOpenAPITestRouter(t *testing.T, svc bodyStateFactService) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	spec, err := openapiv1.GetSwagger()
	if err != nil {
		t.Fatalf("load generated OpenAPI spec: %v", err)
	}
	r := gin.New()
	g := r.Group("")
	g.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.NewString())
		c.Next()
	})
	g.Use(RequestValidator(spec))
	openapiv1.RegisterHandlers(g, StrictHandler(NewPublicServer(svc)))
	return r
}

func performJSON(r http.Handler, body string) *httptest.ResponseRecorder {
	return performJSONAt(r, http.MethodPost, "/api/v1/body-state/facts", body)
}

func performJSONAt(r http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer validated-by-auth-middleware")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestAddBodyStateFactOpenAPIRejectsMissingExpectedRevisionBeforeService(t *testing.T) {
	svc := &fakeBodyStateFactService{}
	rec := performJSON(newOpenAPITestRouter(t, svc), `{"fact":{"kind":"symptom","value":"pain"}}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=400 body=%s", rec.Code, rec.Body.String())
	}
	if svc.calls != 0 {
		t.Fatalf("service calls=%d want=0", svc.calls)
	}
	assertErrorCode(t, rec, "INVALID_REQUEST")
}

func TestAddBodyStateFactOpenAPIRejectsUnknownFieldBeforeService(t *testing.T) {
	svc := &fakeBodyStateFactService{}
	rec := performJSON(newOpenAPITestRouter(t, svc), `{"expected_revision":3,"fact":{"kind":"symptom","value":"pain","unexpected":true}}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=400 body=%s", rec.Code, rec.Body.String())
	}
	if svc.calls != 0 {
		t.Fatalf("service calls=%d want=0", svc.calls)
	}
	assertErrorCode(t, rec, "INVALID_REQUEST")
}

func TestResolveSafetyOpenAPIRequiresExpectedRevisionBeforeAdapter(t *testing.T) {
	svc := &fakeBodyStateFactService{}
	rec := performJSONAt(
		newOpenAPITestRouter(t, svc),
		http.MethodPost,
		"/api/v1/body-state/safety/resolve",
		`{"resolution":"resolved","note":"reviewed"}`,
	)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=400 body=%s", rec.Code, rec.Body.String())
	}
	if svc.calls != 0 {
		t.Fatalf("fact service calls=%d want=0", svc.calls)
	}
	assertErrorCode(t, rec, "INVALID_REQUEST")
}

func TestAddBodyStateFactAllowsIdempotentNilRevision(t *testing.T) {
	now := time.Date(2026, 9, 13, 7, 0, 0, 0, time.UTC)
	svc := &fakeBodyStateFactService{
		resultFact: &model.BodyStateFact{
			ID: uuid.New(), UserID: uuid.New(), Kind: "symptom", Value: "pain",
			Details: []byte(`{}`), Origin: "user_reported", ReviewState: "confirmed",
			LifecycleState: "active", Trend: "unknown", Provenance: []byte(`{}`),
			CreatedRevision: 4, UpdatedRevision: 4, CreatedAt: now, UpdatedAt: now,
		},
		resultRevision: nil,
	}
	rec := performJSON(newOpenAPITestRouter(t, svc), `{"expected_revision":4,"fact":{"kind":"symptom","value":"pain"}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want=200 body=%s", rec.Code, rec.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if revision, ok := response["revision"]; !ok || revision != nil {
		t.Fatalf("revision=%#v want explicit null body=%s", response["revision"], rec.Body.String())
	}
}

func TestAddBodyStateFactOpenAPIMappingAndResponse(t *testing.T) {
	now := time.Date(2026, 9, 13, 7, 0, 0, 0, time.UTC)
	userID := uuid.New()
	factID := uuid.New()
	revisionID := uuid.New()
	regionID := "region:neck"
	svc := &fakeBodyStateFactService{
		resultFact: &model.BodyStateFact{
			ID: factID, UserID: userID, Kind: "symptom", BodyRegionID: &regionID, Value: "pain",
			Details: []byte(`{"severity":4}`), Origin: "user_reported", ReviewState: "confirmed",
			LifecycleState: "active", Trend: "unknown", Provenance: []byte(`{}`),
			CreatedRevision: 4, UpdatedRevision: 4, CreatedAt: now, UpdatedAt: now,
		},
		resultRevision: &model.BodyStateRevision{
			ID: revisionID, UserID: userID, Revision: 4, ChangeType: "fact_upserted", Source: "user_edit",
			Changes: []byte(`{"fact_id":"` + factID.String() + `"}`), CreatedAt: now,
		},
	}
	rec := performJSON(newOpenAPITestRouter(t, svc), `{"expected_revision":3,"fact":{"kind":"symptom","value":"pain","body_region_id":"region:neck","details":{"severity":4}}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want=200 body=%s", rec.Code, rec.Body.String())
	}
	if svc.calls != 1 || svc.expectedRevision == nil || *svc.expectedRevision != 3 {
		t.Fatalf("service call mismatch: calls=%d expectedRevision=%v", svc.calls, svc.expectedRevision)
	}
	if svc.fact.BodyRegionID == nil || *svc.fact.BodyRegionID != regionID || svc.fact.Kind != "symptom" || svc.fact.Value != "pain" {
		t.Fatalf("mapped fact=%+v", svc.fact)
	}
	var response openapiv1.BodyStateFactMutationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Revision.Revision != 4 || response.Fact.Id != factID {
		t.Fatalf("response=%+v", response)
	}
}

func assertErrorCode(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	var response struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode error response: %v body=%s", err, rec.Body.String())
	}
	if response.Error.Code != want {
		t.Fatalf("error.code=%q want=%q body=%s", response.Error.Code, want, rec.Body.String())
	}
}
