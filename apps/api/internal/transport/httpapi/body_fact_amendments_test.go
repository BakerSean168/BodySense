package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

func TestBodyFactAmendmentHTTPRequiresExactAuthenticatedCommand(t *testing.T) {
	owner, target := uuid.New(), uuid.New()
	validCorrection := `{"idempotency_key":"correction","expected_revision":1,"replacement":{"kind":"symptom","statement":"I correct my previous description","body_region_ids":[],"temporal":{"precision":"unknown"},"details":{}}}`
	for _, tc := range []struct {
		name, path, body string
		want             int
	}{
		{"missing replacement", "correct", `{"idempotency_key":"x","expected_revision":1}`, 400},
		{"untrusted owner", "correct", strings.TrimSuffix(validCorrection, "}") + `,"user_id":"` + owner.String() + `"}`, 400},
		{"withdraw cannot replace", "withdraw", validCorrection, 400},
		{"missing revision", "withdraw", `{"idempotency_key":"x"}`, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := &fakeBodyFactApplication{receipt: bodyFactHTTPReceipt(owner)}
			response := performJSONAt(bodyFactHTTPRouter(t, owner, app), http.MethodPost, "/api/v1/body-state/records/"+target.String()+"/"+tc.path, tc.body)
			if response.Code != tc.want || app.calls != 0 {
				t.Fatalf("unvalidated amendment: %d calls=%d %s", response.Code, app.calls, response.Body.String())
			}
		})
	}
	app := &fakeBodyFactApplication{receipt: bodyFactHTTPReceipt(owner)}
	unauthenticated := performJSONAt(bodyFactHTTPRouter(t, uuid.Nil, app), http.MethodPost, "/api/v1/body-state/records/"+target.String()+"/correct", validCorrection)
	if unauthenticated.Code != 401 || app.calls != 0 {
		t.Fatal("unauthenticated amendment reached domain")
	}
	confirmed := performJSONAt(bodyFactHTTPRouter(t, owner, app), http.MethodPost, "/api/v1/body-state/records/"+target.String()+"/correct", validCorrection)
	if confirmed.Code != 200 || app.owner != owner || app.calls != 1 {
		t.Fatalf("valid command did not return receipt: %d %s", confirmed.Code, confirmed.Body.String())
	}
	var receipt model.BodyFactCommit
	if err := json.Unmarshal(confirmed.Body.Bytes(), &receipt); err != nil || receipt.ID != app.receipt.ID {
		t.Fatal("missing exact receipt")
	}
}

func TestBodyFactAmendmentHTTPMapsConflictUnavailableAndMissingReceipt(t *testing.T) {
	owner, target := uuid.New(), uuid.New()
	for _, tc := range []struct {
		name    string
		err     error
		receipt *model.BodyFactCommit
		want    int
	}{
		{"not found", model.ErrBodyFactNotFound, nil, 404},
		{"not current", model.ErrBodyFactNotCurrent, nil, 409},
		{"rebound key", model.ErrBodyFactIdempotencyConflict, nil, 409},
		{"missing receipt", nil, nil, 500},
		{"wrong owner", nil, bodyFactHTTPReceipt(uuid.New()), 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := &fakeBodyFactApplication{err: tc.err, receipt: tc.receipt}
			response := performJSONAt(bodyFactHTTPRouter(t, owner, app), http.MethodPost, "/api/v1/body-state/records/"+target.String()+"/withdraw", `{"idempotency_key":"withdraw","expected_revision":1}`)
			if response.Code != tc.want {
				t.Fatalf("wrong failure contract: %d %s", response.Code, response.Body.String())
			}
		})
	}
}
