package httpapi

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const amendmentOriginalJSON = `{"idempotency_key":"e2e-observation","expected_revision":0,"observation":{"kind":"body_weight","statement":"My home scale read 67.5 kg","raw_value":"67.5","raw_unit":"kg","observed_at":{"precision":"day","start":"2026-10-13"}}}`
const correctionJSON = `{"idempotency_key":"e2e-observation-correct","expected_revision":1,"replacement":{"kind":"body_weight","statement":"I misread the scale, it read 65.7 kg","raw_value":"65.7","raw_unit":"kg","observed_at":{"precision":"day","start":"2026-10-13"}}}`
const withdrawalJSON = `{"idempotency_key":"e2e-observation-withdraw","expected_revision":2}`

func TestBodyObservationGeneratedAmendmentHTTPStrictRejectsForgedSources(t *testing.T) {
	owner, target := uuid.New(), uuid.New()
	fake := &observationHTTPFake{commit: sampleObservationReceipt(owner)}
	fake.commit.TargetObservationID = &target
	fake.commit.Revision = 2 // Expected revision 1 must produce the next durable revision.
	fake.commit.IdempotencyKey = "e2e-observation-correct"
	fake.commit.Operation = "correct"
	fake.commit.ObservationID = uuid.New()
	router := observationHTTPRouter(t, owner, fake)
	path := "/api/v1/body-state/observation-records/" + target.String() + "/correct"
	res := performJSONAt(router, http.MethodPost, path, correctionJSON)
	if res.Code != 200 || fake.calls != 1 || fake.owner != owner {
		t.Fatalf("valid correction did not reach authenticated app: %d %s", res.Code, res.Body.String())
	}
	injected := []string{
		strings.Replace(correctionJSON, `"replacement":{`, `"source_kind":"device","replacement":{`, 1),
		strings.Replace(correctionJSON, `"replacement":{`, `"user_id":"`+uuid.NewString()+`","replacement":{`, 1),
		strings.Replace(correctionJSON, `"kind":"body_weight"`, `"kind":"body_weight","normalized_value":65.7`, 1),
		strings.Replace(correctionJSON, `"replacement":{`, `"replacement":{"source_evidence_id":"`+uuid.NewString()+`",`, 1),
		strings.Replace(correctionJSON, `"statement":"I misread the scale, it read 65.7 kg"`, `"statement":""`, 1),
		strings.Replace(correctionJSON, `"expected_revision":1,`, `"expected_revision":-1,`, 1),
	}
	for i, body := range injected {
		res := performJSONAt(router, http.MethodPost, path, body)
		if res.Code != 400 || fake.calls != 1 {
			t.Fatalf("malformed original correction[%d] entered app: %d %s calls=%d", i, res.Code, res.Body.String(), fake.calls)
		}
	}
	if res := performJSONAt(router, http.MethodPost, path, withdrawalJSON); res.Code != 400 || fake.calls != 1 {
		t.Fatalf("correction accepted no corrective report: %d", res.Code)
	}
	fake.commit.Operation = "withdraw"
	fake.commit.IdempotencyKey = "withdraw"
	fake.commit.ObservationID = target
	withdrawPath := "/api/v1/body-state/observation-records/" + target.String() + "/withdraw"
	if res := performJSONAt(router, http.MethodPost, withdrawPath, `{"idempotency_key":"withdraw","expected_revision":1}`); res.Code != 200 || fake.calls != 2 {
		t.Fatalf("exact withdrawal fake declined: %d %s", res.Code, res.Body.String())
	}
	if res := performJSONAt(router, http.MethodPost, withdrawPath,
		`{"idempotency_key":"withdraw","expected_revision":1,"replacement":{}}`); res.Code != 400 || fake.calls != 2 {
		t.Fatalf("withdraw silently smuggled corrective source: %d %s", res.Code, res.Body.String())
	}
	if res := performJSONAt(observationHTTPRouter(t, uuid.Nil, fake), http.MethodPost, path, correctionJSON); res.Code != 401 {
		t.Fatalf("anonymous correction allowed: %d", res.Code)
	}
	if res := performJSONAt(observationHTTPRouter(t, owner, nil), http.MethodPost, path, correctionJSON); res.Code != 503 {
		t.Fatalf("no application service returned false success: %d", res.Code)
	}
	fake.commit.UserID = uuid.New()
	fake.commit.IdempotencyKey = "e2e-observation-correct"
	fake.commit.Operation = "correct"
	fake.commit.ObservationID = uuid.New()
	if res := performJSONAt(router, http.MethodPost, path, correctionJSON); res.Code != 500 {
		t.Fatalf("forged foreign owner receipt confirmed: %d", res.Code)
	}
	fake.commit.UserID = owner
	fake.commit.IdempotencyKey = "other-key"
	if res := performJSONAt(router, http.MethodPost, path, correctionJSON); res.Code != 500 {
		t.Fatalf("correction confirmed a receipt for another idempotency key: %d", res.Code)
	}
	fake.commit.IdempotencyKey = "e2e-observation-correct"
	fake.commit.Revision = 1
	if res := performJSONAt(router, http.MethodPost, path, correctionJSON); res.Code != 500 {
		t.Fatalf("correction confirmed a receipt that did not advance the authorized revision: %d", res.Code)
	}
	fake.commit.Revision = 3
	fake.commit.IdempotencyKey = "withdraw"
	fake.commit.Operation = "withdraw"
	fake.commit.ObservationID = target
	if res := performJSONAt(router, http.MethodPost, withdrawPath, `{"idempotency_key":"withdraw","expected_revision":1}`); res.Code != 500 {
		t.Fatalf("withdrawal confirmed a receipt for a different revision: %d", res.Code)
	}
}

func TestBodyObservationGeneratedAmendmentHTTPRoundTripPostgres(t *testing.T) {
	dsn := os.Getenv("BODYSENSE_INTEGRATION_DATABASE_URL")
	if dsn == "" {
		t.Skip("native PostgreSQL 18 v12 isolated integration required")
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
			"observation-v12-http-"+id.String()+"@example.test", "synthetic").Error; err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Exec("DELETE FROM users WHERE id=?", id).Error })
	}
	app := service.NewBodyObservationService(repository.NewBodyStateRepository(db))
	route := "/api/v1/body-state/observation-records"
	router := observationHTTPRouter(t, owner, app)
	posted := performJSONAt(router, http.MethodPost, route, amendmentOriginalJSON)
	if posted.Code != 200 {
		t.Fatalf("no atomic source-backed first report: %d %s", posted.Code, posted.Body.String())
	}
	var original model.BodyObservationCommit
	if err := json.Unmarshal(posted.Body.Bytes(), &original); err != nil {
		t.Fatal(err)
	}
	correctionPath := route + "/" + original.ObservationID.String() + "/correct"
	corrected := performJSONAt(router, http.MethodPost, correctionPath, correctionJSON)
	if corrected.Code != 200 {
		t.Fatalf("no authenticated correction: %d %s", corrected.Code, corrected.Body.String())
	}
	var receipt model.BodyObservationCommit
	if err := json.Unmarshal(corrected.Body.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.Revision != 2 || receipt.ObservationID == original.ObservationID ||
		receipt.SourceEvidenceID == original.SourceEvidenceID {
		t.Fatalf("correction modified original in place: %#v", receipt)
	}
	retry := performJSONAt(router, http.MethodPost, correctionPath, correctionJSON)
	if retry.Code != 200 || retry.Body.String() != corrected.Body.String() {
		t.Fatalf("corrective replay changed original receipt: %d %s != %s", retry.Code, retry.Body.String(), corrected.Body.String())
	}
	if other := performJSONAt(observationHTTPRouter(t, foreign, app), http.MethodPost, correctionPath, correctionJSON); other.Code != 404 {
		t.Fatalf("foreign owner amended observation: %d %s", other.Code, other.Body.String())
	}
	// A foreign observation must remain indistinguishable from a missing one,
	// even when the attacker supplies an arbitrarily stale revision. Check
	// both mutation types before revealing any optimistic revision conflict.
	foreignRouter := observationHTTPRouter(t, foreign, app)
	for _, attempt := range []struct{ path, payload string }{
		{correctionPath, strings.Replace(correctionJSON, `"expected_revision":1`, `"expected_revision":999`, 1)},
		{route + "/" + original.ObservationID.String() + "/withdraw", `{"idempotency_key":"foreign-withdraw","expected_revision":999}`},
	} {
		rejected := performJSONAt(foreignRouter, http.MethodPost, attempt.path, attempt.payload)
		if rejected.Code != http.StatusNotFound {
			t.Fatalf("foreign observation with stale revision disclosed state or accepted mutation: %d %s", rejected.Code, rejected.Body.String())
		}
		// Missing and foreign-owned targets must expose identical public errors.
		// Neither a forged revision nor the HTTP error body may enumerate health history.
		missingPath := strings.Replace(attempt.path, original.ObservationID.String(), uuid.NewString(), 1)
		missing := performJSONAt(foreignRouter, http.MethodPost, missingPath, attempt.payload)
		if missing.Code != rejected.Code || missing.Body.String() != rejected.Body.String() {
			t.Fatalf("missing versus foreign observation exposed different public errors: foreign=%d %s missing=%d %s",
				rejected.Code, rejected.Body.String(), missing.Code, missing.Body.String())
		}
	}
	read := performJSONAt(router, http.MethodGet, route, "")
	if read.Code != 200 || strings.Contains(read.Body.String(), "67.5") ||
		!strings.Contains(read.Body.String(), `"raw_value":"65.7"`) ||
		!strings.Contains(read.Body.String(), `"current_revision":2`) {
		t.Fatalf("current projection did not exclude superseded value: %d %s", read.Code, read.Body.String())
	}
	if bad := performJSONAt(router, http.MethodPost, correctionPath, correctionJSON[:len(correctionJSON)-1]); bad.Code != 400 {
		t.Fatalf("malformed correction accepted: %d", bad.Code)
	}
	if old := performJSONAt(router, http.MethodPost, correctionPath,
		strings.Replace(correctionJSON, `"e2e-observation-correct"`, `"correct-again"`, 1)); old.Code != 409 {
		t.Fatalf("non-current observation corrected: %d %s", old.Code, old.Body.String())
	}
	withdrawalPath := route + "/" + receipt.ObservationID.String() + "/withdraw"
	withdrawn := performJSONAt(router, http.MethodPost, withdrawalPath, withdrawalJSON)
	if withdrawn.Code != 200 {
		t.Fatalf("withdrawal denied: %d %s", withdrawn.Code, withdrawn.Body.String())
	}
	again := performJSONAt(router, http.MethodPost, withdrawalPath, withdrawalJSON)
	if again.Code != 200 || again.Body.String() != withdrawn.Body.String() {
		t.Fatalf("withdrawal retry broke ID/time stability: %d %s", again.Code, again.Body.String())
	}
	after := performJSONAt(router, http.MethodGet, route, "")
	if after.Code != 200 || !strings.Contains(after.Body.String(), `"current_revision":3`) ||
		!strings.Contains(after.Body.String(), `"observations":[]`) {
		t.Fatalf("withdrawn source still appears current: %d %s", after.Code, after.Body.String())
	}
}
