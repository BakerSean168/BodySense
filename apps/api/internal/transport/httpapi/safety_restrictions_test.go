package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
)

type fakeSafetyRestrictions struct {
	calls   int
	receipt *model.SafetyResolution
	list    *model.SafetyRestrictionList
	err     error
}

func (f *fakeSafetyRestrictions) ResolveSafetyRestriction(_ context.Context, owner, target uuid.UUID, cmd model.ResolveSafetyRestrictionCommand) (*model.SafetyResolution, error) {
	f.calls++
	if f.receipt != nil {
		f.receipt.UserID = owner
		f.receipt.RestrictionID = target
		f.receipt.IdempotencyKey = cmd.IdempotencyKey
	}
	return f.receipt, f.err
}
func (f *fakeSafetyRestrictions) ListSafetyRestrictions(context.Context, uuid.UUID) (*model.SafetyRestrictionList, error) {
	f.calls++
	return f.list, f.err
}

func TestSafetyRestrictionHTTPExplicitSourceAndFailureMapping(t *testing.T) {
	owner, target, message := uuid.New(), uuid.New(), uuid.New()
	body := fmt.Sprintf(`{"expected_revision":4,"idempotency_key":"clear-one","source_message_id":%q,"statement":%q}`, message, model.ProfessionalClearanceStatement)
	receipt := &model.SafetyResolution{ID: uuid.New(), SourceEvidenceID: uuid.New(), SourceMessageID: message, SourceVersion: 1, BasisKind: "professional_clearance", SourceLevel: "user_reported", ScopeKind: "all_training", PolicyRevision: model.SafetyResolutionPolicyV1, Revision: 5, RevisionID: uuid.New(), EventID: uuid.New(), Actor: "user", LiftedAt: time.Now().UTC()}
	app := &fakeSafetyRestrictions{receipt: receipt}
	router := newOpenAPITestRouterWith(t, NewPublicServer().WithSafetyRestrictions(app), owner.String())
	path := "/api/v1/body-state/safety/restrictions/" + target.String() + "/resolve"
	rec := performJSONAt(router, http.MethodPost, path, body)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	for _, tc := range []struct {
		err    error
		status int
	}{{model.ErrSafetyRestrictionNotFound, 404}, {model.ErrSafetyResolutionSourceNotFound, 404}, {model.ErrInvalidSafetyResolution, 400}, {model.ErrSafetyResolutionIdempotencyConflict, 409}, {repository.ErrBodyStateRevisionConflict, 409}, {errors.New("database offline"), 503}} {
		app.err = tc.err
		rec = performJSONAt(router, http.MethodPost, path, body)
		if rec.Code != tc.status {
			t.Fatalf("%v: %d %s", tc.err, rec.Code, rec.Body.String())
		}
	}
	before := app.calls
	rec = performJSONAt(router, http.MethodPost, path, `{"expected_revision":4,"idempotency_key":"bad","source_message_id":"`+message.String()+`","statement":"我没事了","risk_level":"urgent"}`)
	if rec.Code != 400 || app.calls != before {
		t.Fatalf("unknown/unsourced command reached app: %d", rec.Code)
	}
	unauth := newOpenAPITestRouterWith(t, NewPublicServer().WithSafetyRestrictions(app), "")
	rec = performJSONAt(unauth, http.MethodPost, path, body)
	if rec.Code != 401 {
		t.Fatalf("unauthenticated clearance: %d", rec.Code)
	}
}

func TestSafetyRestrictionHTTPReadFailsClosed(t *testing.T) {
	owner := uuid.New()
	app := &fakeSafetyRestrictions{list: &model.SafetyRestrictionList{UserID: owner, CurrentRevision: 0, Restrictions: []model.SafetyRestrictionRecord{}}}
	router := newOpenAPITestRouterWith(t, NewPublicServer().WithSafetyRestrictions(app), owner.String())
	path := "/api/v1/body-state/safety/restrictions"
	if rec := performJSONAt(router, http.MethodGet, path, ""); rec.Code != 200 {
		t.Fatalf("empty native read: %d %s", rec.Code, rec.Body.String())
	}
	app.list.CurrentRevision = 1
	app.list.Restrictions = []model.SafetyRestrictionRecord{{SafetyRestriction: model.SafetyRestriction{
		ID: uuid.New(), UserID: owner, SafetyScreenID: uuid.New(), Concept: "numbness",
		PolicyRevision: model.SafetyScreenRestrictionPolicyV1, RiskLevel: "restricted", ScopeKind: "all_training",
		LiftPolicy: "professional_clearance", Reason: "synthetic", CreatedRevision: 1, ValidFrom: time.Now().UTC(),
	}, Active: true}}
	if rec := performJSONAt(router, http.MethodGet, path, ""); rec.Code != 200 {
		t.Fatalf("native restriction with null resolution: %d %s", rec.Code, rec.Body.String())
	}
	app.list.UserID = uuid.New()
	if rec := performJSONAt(router, http.MethodGet, path, ""); rec.Code != 503 {
		t.Fatalf("foreign read accepted: %d", rec.Code)
	}
	app.err = errors.New("read unavailable")
	if rec := performJSONAt(router, http.MethodGet, path, ""); rec.Code != 503 {
		t.Fatalf("unavailable read allowed: %d", rec.Code)
	}
}
