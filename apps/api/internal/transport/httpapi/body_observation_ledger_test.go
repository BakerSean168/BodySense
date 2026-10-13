package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type observationHTTPFake struct {
	calls   int
	owner   uuid.UUID
	command model.RecordBodyObservationCommand
	cursor  int64
	commit  *model.BodyObservationCommit
	page    *model.BodyObservationPage
	err     error
}

func (f *observationHTTPFake) RecordDirectObservation(_ context.Context, owner uuid.UUID, command model.RecordBodyObservationCommand) (*model.BodyObservationCommit, error) {
	f.calls++
	f.owner = owner
	f.command = command
	return f.commit, f.err
}
func (f *observationHTTPFake) ListDirectObservations(_ context.Context, owner uuid.UUID, cursor int64) (*model.BodyObservationPage, error) {
	f.calls++
	f.owner = owner
	f.cursor = cursor
	return f.page, f.err
}
func observationHTTPRouter(t *testing.T, owner uuid.UUID, app bodyObservationApplication) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	spec, err := openapiv1.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	RegisterRoutes(router, StrictHandler(NewPublicServer(&fakeBodyStateFactService{}).WithBodyObservations(app)), RouteSecurity{
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

func sampleObservationReceipt(owner uuid.UUID) *model.BodyObservationCommit {
	return &model.BodyObservationCommit{
		ID: uuid.New(), UserID: owner, ObservationID: uuid.New(), SourceEvidenceID: uuid.New(),
		Revision: 1, RevisionID: uuid.New(), EventID: uuid.New(), CommittedAt: time.Now().UTC(),
	}
}

const observationJSON = `{"idempotency_key":"weight-on-my-scale","expected_revision":0,"observation":{"kind":"body_weight","statement":"I measured 67.5 kilograms today","raw_value":"67.5","raw_unit":"kg","observed_at":{"precision":"day","start":"2026-10-13"}}}`

func TestBodyObservationGeneratedHTTPStrictDirectSelfReportOnly(t *testing.T) {
	owner := uuid.New()
	fake := &observationHTTPFake{commit: sampleObservationReceipt(owner)}
	router := observationHTTPRouter(t, owner, fake)
	path := "/api/v1/body-state/observation-records"
	res := performJSONAt(router, http.MethodPost, path, observationJSON)
	if res.Code != http.StatusOK || fake.calls != 1 || fake.owner != owner ||
		fake.command.Observation.RawValue != "67.5" || fake.command.ExpectedRevision != 0 ||
		!strings.Contains(res.Body.String(), fake.commit.ObservationID.String()) ||
		strings.Contains(res.Body.String(), "67.5") || strings.Contains(res.Body.String(), "idempotency_key") {
		t.Fatalf("generated POST did not preserve strict source/ID-only receipt: %d %s", res.Code, res.Body.String())
	}
	injected := []string{
		strings.Replace(observationJSON, `"observation":{`, `"source_kind":"device","observation":{`, 1),
		strings.Replace(observationJSON, `"kind":"body_weight"`, `"user_id":"`+uuid.NewString()+`","kind":"body_weight"`, 1),
		strings.Replace(observationJSON, `"kind":"body_weight"`, `"source":{"kind":"document"},"kind":"body_weight"`, 1),
		strings.Replace(observationJSON, `"kind":"body_weight"`, `"normalized_value":67.5,"kind":"body_weight"`, 1),
		strings.Replace(observationJSON, `"kind":"body_weight"`, `"diagnosis":"hypertension","kind":"body_weight"`, 1),
		strings.Replace(observationJSON, `"expected_revision":0,`, "", 1),
		strings.Replace(observationJSON, `"expected_revision":0,`, `"expected_revision":-1,`, 1),
		strings.Replace(observationJSON, `"statement":"I measured 67.5 kilograms today"`, `"statement":""`, 1),
	}
	for i, body := range injected {
		denied := performJSONAt(router, http.MethodPost, path, body)
		if denied.Code != http.StatusBadRequest || fake.calls != 1 {
			t.Fatalf("invalid high-trust observation[%d] reached service: %d %s calls=%d", i, denied.Code, denied.Body.String(), fake.calls)
		}
	}
	fake.commit = sampleObservationReceipt(uuid.New())
	if forged := performJSONAt(router, http.MethodPost, path, observationJSON); forged.Code != http.StatusInternalServerError {
		t.Fatalf("forged owner receipt accepted as save: %d", forged.Code)
	}
	fake.err = errors.New("backend private original health statement")
	fake.commit = nil
	if failure := performJSONAt(router, http.MethodPost, path, observationJSON); failure.Code != http.StatusInternalServerError ||
		strings.Contains(failure.Body.String(), "private original health statement") {
		t.Fatalf("backend error leaked protected source: %d %s", failure.Code, failure.Body.String())
	}
	if anonymous := performJSONAt(observationHTTPRouter(t, uuid.Nil, fake), http.MethodPost, path, observationJSON); anonymous.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous observation save accepted: %d", anonymous.Code)
	}
	if missing := performJSONAt(observationHTTPRouter(t, owner, nil), http.MethodPost, path, observationJSON); missing.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing service represented success: %d", missing.Code)
	}
}

func TestBodyObservationGeneratedHTTPCurrentReadStrictSourceView(t *testing.T) {
	owner := uuid.New()
	rec := model.BodyObservationRecord{ID: uuid.New(), UserID: owner, Kind: "body_weight",
		Statement: "I weighed myself", RawValue: "65", RawUnit: "kg",
		ObservedAt: model.FactTemporal{Precision: "unknown"}, Source: model.BodyObservationSource{
			Kind: "user_report", ID: uuid.New(), Version: 1},
		AssertionStatus: "asserted", CreatedRevision: 1, RecordedAt: time.Now().UTC()}
	fake := &observationHTTPFake{page: &model.BodyObservationPage{UserID: owner, CurrentRevision: 1,
		Observations: []model.BodyObservationRecord{rec}}}
	path := "/api/v1/body-state/observation-records"
	router := observationHTTPRouter(t, owner, fake)
	response := performJSONAt(router, http.MethodGet, path, "")
	if response.Code != http.StatusOK || fake.calls != 1 || !strings.Contains(response.Body.String(), `"kind":"user_report"`) ||
		!strings.Contains(response.Body.String(), `"precision":"unknown"`) {
		t.Fatalf("read of self-report was not source-visible: %d %s", response.Code, response.Body.String())
	}
	fake.page.Observations[0].Source.Kind = "device"
	if denied := performJSONAt(router, http.MethodGet, path, ""); denied.Code != http.StatusInternalServerError {
		t.Fatalf("non-admitted device source spoofed user report: %d", denied.Code)
	}
	fake.page.Observations[0].Source.Kind = "user_report"
	fake.page.CurrentRevision = 0
	if denied := performJSONAt(router, http.MethodGet, path, ""); denied.Code != http.StatusInternalServerError {
		t.Fatalf("future revision record leaked: %d", denied.Code)
	}
	if bad := performJSONAt(router, http.MethodGet, path+"?before_revision=0", ""); bad.Code != http.StatusBadRequest {
		t.Fatalf("invalid cursor accepted: %d", bad.Code)
	}
	if anon := performJSONAt(observationHTTPRouter(t, uuid.Nil, fake), http.MethodGet, path, ""); anon.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous read accepted: %d", anon.Code)
	}
}

func TestBodyObservationHTTPRealRoundTripPostgres(t *testing.T) {
	dsn := os.Getenv("BODYSENSE_INTEGRATION_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated native PostgreSQL 18 v11 required")
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
	owner, foreign := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{owner, foreign} {
		if err := db.Exec("INSERT INTO users(id,email,password_hash) VALUES(?,?,?)", id,
			"observation-http-"+id.String()+"@example.test", "synthetic").Error; err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Exec("DELETE FROM users WHERE id=?", id).Error })
	}
	app := service.NewBodyObservationService(repository.NewBodyStateRepository(db))
	router := observationHTTPRouter(t, owner, app)
	route := "/api/v1/body-state/observation-records"
	posted := performJSONAt(router, http.MethodPost, route, observationJSON)
	if posted.Code != http.StatusOK {
		t.Fatalf("real observation POST: %d %s", posted.Code, posted.Body.String())
	}
	var receipt model.BodyObservationCommit
	if err := json.Unmarshal(posted.Body.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.UserID != owner || receipt.Revision != 1 || receipt.SourceEvidenceID == uuid.Nil ||
		receipt.ObservationID == uuid.Nil {
		t.Fatalf("not durable: %#v", receipt)
	}
	replay := performJSONAt(router, http.MethodPost, route, observationJSON)
	if replay.Code != http.StatusOK || replay.Body.String() != posted.Body.String() {
		t.Fatalf("same-key HTTP replay response must be byte-for-byte stable, including timestamp: original=%s replay=%s", posted.Body.String(), replay.Body.String())
	}
	read := performJSONAt(router, http.MethodGet, route, "")
	if read.Code != http.StatusOK || !strings.Contains(read.Body.String(), `"kind":"user_report"`) ||
		!strings.Contains(read.Body.String(), `"raw_value":"67.5"`) ||
		!strings.Contains(read.Body.String(), `"statement":"I measured 67.5 kilograms today"`) {
		t.Fatalf("real PG observation view broke: %d %s", read.Code, read.Body.String())
	}
	if foreignRead := performJSONAt(observationHTTPRouter(t, foreign, app), http.MethodGet, route, ""); foreignRead.Code != http.StatusOK || strings.Contains(foreignRead.Body.String(), "67.5") {
		t.Fatalf("cross-user reported measurement disclosed: %d %s", foreignRead.Code, foreignRead.Body.String())
	}
	alternate := strings.Replace(observationJSON, `"raw_value":"67.5"`, `"raw_value":"81"`, 1)
	if collision := performJSONAt(router, http.MethodPost, route, alternate); collision.Code != http.StatusConflict {
		t.Fatalf("changed user value reused command key: %d %s", collision.Code, collision.Body.String())
	}
	if err := db.Model(&model.StatementEvidence{}).Where("user_id=? AND id=?", owner, receipt.SourceEvidenceID).
		Update("availability", "blocked").Error; err != nil {
		t.Fatal(err)
	}
	hidden := performJSONAt(router, http.MethodGet, route, "")
	if hidden.Code != http.StatusOK || strings.Contains(hidden.Body.String(), receipt.ObservationID.String()) {
		t.Fatalf("blocked source disclosed observation: %d %s", hidden.Code, hidden.Body.String())
	}
	// The v11 write never modifies the legacy table as a second source.
	var oldCount int64
	if err := db.Table("body_state_observations").Where("user_id=?", owner).Count(&oldCount).Error; err != nil || oldCount != 0 {
		t.Fatalf("unreviewed legacy observation dual-write: %d %v", oldCount, err)
	}
	var facts int64
	if err := db.Table("body_facts").Where("user_id=?", owner).Count(&facts).Error; err != nil || facts != 0 {
		t.Fatalf("observation incorrectly persisted as self symptom Fact: %d %v", facts, err)
	}
}
