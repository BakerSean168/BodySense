package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type nativeRunReaderFake struct {
	read                *service.NativeDiagnosisRunProjection
	list                *service.NativeDiagnosisRunPage
	err                 error
	owner, run          uuid.UUID
	before              *uuid.UUID
	limit               int
	getCalls, listCalls int
}

func (f *nativeRunReaderFake) Read(_ context.Context, owner, id uuid.UUID) (*service.NativeDiagnosisRunProjection, error) {
	f.owner = owner
	f.run = id
	f.getCalls++
	return f.read, f.err
}
func (f *nativeRunReaderFake) List(_ context.Context, owner uuid.UUID, limit int, before *uuid.UUID) (*service.NativeDiagnosisRunPage, error) {
	f.owner = owner
	f.limit = limit
	f.before = before
	f.listCalls++
	return f.list, f.err
}
func nativeRunRoute(t *testing.T, owner uuid.UUID, reader nativeDiagnosisRunReadApplication) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	spec, err := openapiv1.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	RegisterRoutes(router, StrictHandler(NewPublicServer().WithNativeDiagnosisRuns(reader)), RouteSecurity{
		Auth: func(c *gin.Context) {
			if owner != uuid.Nil {
				c.Set("user_id", owner.String())
			}
			c.Next()
		},
		Operator: func(c *gin.Context) { c.Next() }, Validator: RequestValidator(spec),
	})
	return router
}
func nativeRunGET(t *testing.T, router *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRecorder()
	router.ServeHTTP(r, httptest.NewRequest(http.MethodGet, path, nil))
	return r
}
func TestNativeDiagnosisRunHTTPProjectionExactProposalAndOwner(t *testing.T) {
	owner, run, proposal, requestMessage, requestEvidence := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	now := time.Now().UTC().Truncate(time.Microsecond)
	fingerprint := strings.Repeat("d", 64)
	ref := model.DiagnosisEvidenceRef{EvidenceType: model.DiagnosisBodyFact, EvidenceID: uuid.New(), Version: "1"}
	fake := &nativeRunReaderFake{read: &service.NativeDiagnosisRunProjection{
		OwnerID: owner,
		NativeDiagnosisRunSummary: service.NativeDiagnosisRunSummary{
			RunID: run, Trigger: model.ConditionNewAnalysis, BasedOnBodyStateRevision: 2,
			StartedAt: now, State: "open",
		},
		CurrentDiagnosisRevision: 0, RequestMessageID: requestMessage, RequestEvidenceID: requestEvidence,
		RequestSourceVersion: 1, Scope: model.ConditionAnalysisScope{
			ConditionIDs: []uuid.UUID{}, EvidenceRefs: []model.DiagnosisEvidenceRef{ref}, Description: "Synthetic user requested analysis",
		},
		InputEvidenceRefs: []model.DiagnosisEvidenceRef{ref},
		AgentHypotheses:   []model.ConditionAgentHypothesis{},
		Events:            []service.NativeDiagnosisRunEvent{},
		Proposals: []service.NativeDiagnosisRunProposal{{
			ProposalID: proposal, RunID: run, ChangeKind: model.DiagnosisCreate,
			Content:            json.RawMessage(`{"proposedEntry":{"medicalName":"Synthetic named syndrome","assessment":{"level":"suspected"}}}`),
			ContentFingerprint: fingerprint, BaseRevision: 0, EvidenceRefs: []model.DiagnosisEvidenceRef{ref},
			CounterEvidenceRefs: []model.DiagnosisEvidenceRef{}, CreatedAt: now, State: "pending",
		}},
	}}
	changedCondition, changedAt := uuid.New(), int64(1)
	fake.read.Freshness = &service.DiagnosisFreshness{
		State: service.DiagnosisFreshnessStale, BasedOnBodyStateRevision: 2, EvaluatedBodyStateRevision: 2, EvaluatedDiagnosisRevision: 1,
		Reasons: []service.DiagnosisFreshnessReason{
			{Code: "diagnosis_revision_changed", DiagnosisRevision: &changedAt},
			{Code: "condition_changed", ConditionID: &changedCondition, DiagnosisRevision: &changedAt},
		},
	}
	router := nativeRunRoute(t, owner, fake)
	r := nativeRunGET(t, router, "/api/v1/diagnosis/analysis-runs/"+run.String())
	if r.Code != 200 {
		t.Fatalf("read run failed strict schema: %d %s", r.Code, r.Body.String())
	}
	if fake.getCalls != 1 || fake.owner != owner || fake.run != run {
		t.Fatal("authenticated owner/path id not applied")
	}
	var result map[string]any
	if err := json.Unmarshal(r.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	proposals := result["proposals"].([]any)
	got := proposals[0].(map[string]any)
	if result["runId"] != run.String() || result["state"] != "open" || got["proposalId"] != proposal.String() ||
		got["contentFingerprint"] != fingerprint || got["baseRevision"] != float64(0) || got["state"] != "pending" {
		t.Fatalf("missing authentic explicit-choice fields: %#v", result)
	}
	freshness, _ := result["freshness"].(map[string]any)
	reasons, _ := freshness["reasons"].([]any)
	if freshness["state"] != "stale" || len(reasons) != 2 || reasons[1].(map[string]any)["conditionId"] != changedCondition.String() {
		t.Fatalf("open run freshness not projected: %#v", result["freshness"])
	}
	for _, secret := range [][]byte{[]byte("executionProvenance"), []byte("replayInput"), []byte("idempotencyKey"),
		[]byte(`"user_id"`), []byte(`"ownerId"`)} {
		if bytes.Contains(r.Body.Bytes(), secret) {
			t.Fatalf("internal provenance/owner leaked: %s", secret)
		}
	}
	fake.read.OwnerID = uuid.New()
	if foreign := nativeRunGET(t, router, "/api/v1/diagnosis/analysis-runs/"+run.String()); foreign.Code != 503 {
		t.Fatalf("foreign projection should fail closed: %d %s", foreign.Code, foreign.Body.String())
	}
}

func TestNativeDiagnosisRunHTTPPaginationAndFailClosed(t *testing.T) {
	owner, run, cursor := uuid.New(), uuid.New(), uuid.New()
	fake := &nativeRunReaderFake{list: &service.NativeDiagnosisRunPage{
		OwnerID: owner, HasMore: true, NextBeforeRunID: &cursor,
		Runs: []service.NativeDiagnosisRunSummary{{
			RunID: cursor, Trigger: model.ConditionPartialUpdate, StartedAt: time.Now().UTC(),
			State: "open",
		}},
	}}
	router := nativeRunRoute(t, owner, fake)
	r := nativeRunGET(t, router, "/api/v1/diagnosis/analysis-runs?limit=1&beforeRunId="+run.String())
	if r.Code != 200 {
		t.Fatalf("index read: %d %s", r.Code, r.Body.String())
	}
	if fake.owner != owner || fake.limit != 1 || fake.before == nil || *fake.before != run || fake.listCalls != 1 {
		t.Fatalf("pagination was not owner-fenced: %+v", fake)
	}
	var data map[string]any
	if err := json.Unmarshal(r.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if data["nextBeforeRunId"] != cursor.String() || data["hasMore"] != true {
		t.Fatalf("bad keyset projection: %#v", data)
	}
	for _, path := range []string{
		"/api/v1/diagnosis/analysis-runs?limit=0",
		"/api/v1/diagnosis/analysis-runs?limit=51",
		"/api/v1/diagnosis/analysis-runs?beforeRunId=not-a-uuid",
	} {
		r := nativeRunGET(t, router, path)
		if r.Code != 400 {
			t.Fatalf("invalid keyset accepted: %q %d %s", path, r.Code, r.Body.String())
		}
	}
	fake.err = model.ErrDiagnosisNotFound
	if r := nativeRunGET(t, router, "/api/v1/diagnosis/analysis-runs?beforeRunId="+run.String()); r.Code != 404 {
		t.Fatalf("foreign cursor was not masked: %d %s", r.Code, r.Body.String())
	}
	fake.err = errors.New("db down")
	if r := nativeRunGET(t, router, "/api/v1/diagnosis/analysis-runs"); r.Code != 503 {
		t.Fatalf("db failure leaked: %d %s", r.Code, r.Body.String())
	}
	fake.err = nil
	fake.list.OwnerID = uuid.New()
	if r := nativeRunGET(t, router, "/api/v1/diagnosis/analysis-runs"); r.Code != 503 {
		t.Fatalf("foreign page was not blocked: %d %s", r.Code, r.Body.String())
	}
	unauth := nativeRunRoute(t, uuid.Nil, fake)
	if r := nativeRunGET(t, unauth, "/api/v1/diagnosis/analysis-runs"); r.Code != 401 {
		t.Fatalf("unauthenticated index accepted: %d %s", r.Code, r.Body.String())
	}
}

func TestNativeDiagnosisRunHTTPMissingAndInvalidRun(t *testing.T) {
	owner := uuid.New()
	run := uuid.New()
	fake := &nativeRunReaderFake{err: model.ErrDiagnosisNotFound}
	router := nativeRunRoute(t, owner, fake)
	if r := nativeRunGET(t, router, "/api/v1/diagnosis/analysis-runs/"+run.String()); r.Code != 404 {
		t.Fatalf("foreign/missing run not owner masked: %d %s", r.Code, r.Body.String())
	}
	if r := nativeRunGET(t, router, "/api/v1/diagnosis/analysis-runs/bad-id"); r.Code != 400 {
		t.Fatalf("invalid run id accepted: %d %s", r.Code, r.Body.String())
	}
	if r := nativeRunGET(t, router, "/api/v1/diagnosis/analysis-runs/"+uuid.Nil.String()); r.Code != 400 {
		t.Fatalf("zero run id accepted: %d %s", r.Code, r.Body.String())
	}
	fake.err = nil
	if r := nativeRunGET(t, nativeRunRoute(t, owner, nil), "/api/v1/diagnosis/analysis-runs/"+run.String()); r.Code != 503 {
		t.Fatalf("missing Go service fail-open: %d %s", r.Code, r.Body.String())
	}
	unauth := nativeRunRoute(t, uuid.Nil, fake)
	if r := nativeRunGET(t, unauth, "/api/v1/diagnosis/analysis-runs/"+run.String()); r.Code != 401 {
		t.Fatalf("unauthorized run read: %d %s", r.Code, r.Body.String())
	}
}
