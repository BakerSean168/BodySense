package httpapi

import (
	"context"
	"encoding/json"
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

const bodyFactRequestJSON = `{"idempotency_key":"test-key","expected_revision":0,"facts":[{"kind":"symptom","statement":"My shoulder hurts.","body_region_ids":[],"temporal":{"precision":"unknown"},"details":{"presence":"present"}}]}`

type fakeBodyFactApplication struct {
	calls   int
	owner   uuid.UUID
	command model.RecordBodyFactsCommand
	receipt *model.BodyFactCommit
	page    *model.BodyFactPage
	err     error
}

func (f *fakeBodyFactApplication) RecordDirectFacts(_ context.Context, userID uuid.UUID, command model.RecordBodyFactsCommand) (*model.BodyFactCommit, error) {
	f.calls++
	f.owner = userID
	f.command = command
	return f.receipt, f.err
}
func (f *fakeBodyFactApplication) ListFacts(_ context.Context, userID uuid.UUID, _ int64) (*model.BodyFactPage, error) {
	f.calls++
	f.owner = userID
	return f.page, f.err
}
func bodyFactHTTPReceipt(userID uuid.UUID) *model.BodyFactCommit {
	ids, _ := json.Marshal([]uuid.UUID{uuid.New()})
	return &model.BodyFactCommit{ID: uuid.New(), UserID: userID, Revision: 1, RevisionID: uuid.New(), EventID: uuid.New(), FactIDs: datatypes.JSON(ids), CommittedAt: time.Now().UTC(), IdempotencyKey: "test-key", Fingerprint: strings.Repeat("a", 64)}
}
func bodyFactHTTPRouter(t *testing.T, owner uuid.UUID, application bodyFactApplication) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	spec, err := openapiv1.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	RegisterRoutes(r, StrictHandler(NewPublicServer().WithBodyFacts(application)), RouteSecurity{
		Auth: func(c *gin.Context) {
			if owner != uuid.Nil {
				c.Set("user_id", owner.String())
			}
			c.Next()
		},
		Operator:  func(c *gin.Context) { c.Next() },
		Validator: RequestValidator(spec),
	})
	return r
}

func TestBodyFactHTTPGeneratedContractBindsOwnerAndReturnsReceipt(t *testing.T) {
	owner := uuid.New()
	application := &fakeBodyFactApplication{receipt: bodyFactHTTPReceipt(owner)}
	rec := performJSONAt(bodyFactHTTPRouter(t, owner, application), http.MethodPost, "/api/v1/body-state/records", bodyFactRequestJSON)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if application.calls != 1 || application.owner != owner || application.command.Facts[0].Kind != "symptom" {
		t.Fatalf("command mapping failed: %#v", application)
	}
	if !strings.Contains(rec.Body.String(), application.receipt.ID.String()) || strings.Contains(rec.Body.String(), "fingerprint") || strings.Contains(rec.Body.String(), "idempotency_key") {
		t.Fatalf("wrong public receipt: %s", rec.Body.String())
	}
}

func TestBodyFactHTTPRejectsUntrustedAndLegacyFieldsBeforeApplication(t *testing.T) {
	cases := map[string]string{
		"user_id":            strings.Replace(bodyFactRequestJSON, `"expected_revision":0`, `"expected_revision":0,"user_id":"`+uuid.NewString()+`"`, 1),
		"source_id":          strings.Replace(bodyFactRequestJSON, `"kind":"symptom"`, `"kind":"symptom","source_evidence_id":"`+uuid.NewString()+`"`, 1),
		"legacy_lifecycle":   strings.Replace(bodyFactRequestJSON, `"presence":"present"`, `"presence":"present","lifecycle_state":"resolved"`, 1),
		"wrong_variant":      strings.Replace(bodyFactRequestJSON, `"kind":"symptom"`, `"kind":"lifestyle"`, 1),
		"missing_revision":   strings.Replace(bodyFactRequestJSON, `"expected_revision":0,`, "", 1),
		"null_regions":       strings.Replace(bodyFactRequestJSON, `"body_region_ids":[]`, `"body_region_ids":null`, 1),
		"training_reference": strings.Replace(strings.Replace(bodyFactRequestJSON, `"kind":"symptom"`, `"kind":"training_feedback"`, 1), `"presence":"present"`, `"session_log_id":"`+uuid.NewString()+`"`, 1),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			owner := uuid.New()
			application := &fakeBodyFactApplication{receipt: bodyFactHTTPReceipt(owner)}
			rec := performJSONAt(bodyFactHTTPRouter(t, owner, application), http.MethodPost, "/api/v1/body-state/records", body)
			if rec.Code != http.StatusBadRequest || application.calls != 0 {
				t.Fatalf("status=%d calls=%d body=%s", rec.Code, application.calls, rec.Body.String())
			}
		})
	}
}

func TestBodyFactHTTPRequiresAuthentication(t *testing.T) {
	application := &fakeBodyFactApplication{}
	r := bodyFactHTTPRouter(t, uuid.Nil, application)
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		rec := performJSONAt(r, method, "/api/v1/body-state/records", bodyFactRequestJSON)
		if rec.Code != http.StatusUnauthorized || application.calls != 0 {
			t.Fatalf("status=%d calls=%d", rec.Code, application.calls)
		}
	}
}

func TestBodyFactHTTPNeverReportsUncommittedSuccess(t *testing.T) {
	owner := uuid.New()
	cases := []struct {
		name    string
		receipt *model.BodyFactCommit
		err     error
		status  int
	}{
		{"missing_receipt", nil, nil, http.StatusInternalServerError},
		{"wrong_owner", bodyFactHTTPReceipt(uuid.New()), nil, http.StatusInternalServerError},
		{"failed_commit", nil, errors.New("private internal database diagnostic"), http.StatusInternalServerError},
		{"idempotency_conflict", nil, model.ErrBodyFactIdempotencyConflict, http.StatusConflict},
		{"revision_conflict", nil, repository.ErrBodyStateRevisionConflict, http.StatusConflict},
		{"invalid_fact", nil, model.ErrInvalidBodyFact, http.StatusBadRequest},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			application := &fakeBodyFactApplication{receipt: test.receipt, err: test.err}
			rec := performJSONAt(bodyFactHTTPRouter(t, owner, application), http.MethodPost, "/api/v1/body-state/records", bodyFactRequestJSON)
			if rec.Code != test.status || strings.Contains(rec.Body.String(), "private internal") {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestBodyFactHTTPValidatesReadProjectionAndCursor(t *testing.T) {
	owner := uuid.New()
	application := &fakeBodyFactApplication{page: &model.BodyFactPage{UserID: owner, Facts: []model.BodyFact{}}}
	r := bodyFactHTTPRouter(t, owner, application)
	rec := performJSONAt(r, http.MethodGet, "/api/v1/body-state/records", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"facts":[]`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	application.page.UserID = uuid.New()
	rec = performJSONAt(r, http.MethodGet, "/api/v1/body-state/records", "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatal("cross-user projection accepted")
	}
	application.calls = 0
	rec = performJSONAt(r, http.MethodGet, "/api/v1/body-state/records?before_revision=-1", "")
	if rec.Code != http.StatusBadRequest || application.calls != 0 {
		t.Fatal("invalid cursor reached application")
	}
}

func (f *fakeBodyFactApplication) CorrectDirectFact(_ context.Context, owner, target uuid.UUID, command model.AmendBodyFactCommand) (*model.BodyFactCommit, error) {
	f.calls++
	f.owner = owner
	return f.receipt, f.err
}
func (f *fakeBodyFactApplication) WithdrawDirectFact(_ context.Context, owner, target uuid.UUID, command model.AmendBodyFactCommand) (*model.BodyFactCommit, error) {
	f.calls++
	f.owner = owner
	return f.receipt, f.err
}

func TestRetiredBodyStateRoutesAbsent(t *testing.T) {
	router := newOpenAPITestRouterWith(t, NewPublicServer(), "55555555-5555-4555-8555-555555555555")
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/body-state"},
		{http.MethodPatch, "/api/v1/body-state/facts/55555555-5555-4555-8555-555555555555/temporal"},
		{http.MethodPatch, "/api/v1/body-state/facts/55555555-5555-4555-8555-555555555555/review"},
		{http.MethodPost, "/api/v1/body-state/observations"},
		{http.MethodPatch, "/api/v1/body-state/observations/55555555-5555-4555-8555-555555555555/review"},
		{http.MethodGet, "/api/v1/body-state/evidence"},
	} {
		response := performJSONAt(router, route.method, route.path, "")
		if response.Code != http.StatusNotFound {
			t.Fatalf("%s %s status=%d", route.method, route.path, response.Code)
		}
	}
}
