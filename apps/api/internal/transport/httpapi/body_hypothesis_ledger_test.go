package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
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

type hypothesisHTTPFake struct {
	calls    int
	owner    uuid.UUID
	target   uuid.UUID
	record   model.RecordBodyHypothesisCommand
	withdraw model.WithdrawBodyHypothesisCommand
	correct  model.CorrectBodyHypothesisCommand
	commit   *model.BodyHypothesisCommit
	list     *model.BodyHypothesisList
	groups   *model.DerivedConcernGroupList
	err      error
}

func (f *hypothesisHTTPFake) RecordBodyHypothesis(_ context.Context, owner uuid.UUID, c model.RecordBodyHypothesisCommand) (*model.BodyHypothesisCommit, error) {
	f.calls++
	f.owner, f.record = owner, c
	return f.commit, f.err
}
func (f *hypothesisHTTPFake) WithdrawBodyHypothesis(_ context.Context, owner, target uuid.UUID, c model.WithdrawBodyHypothesisCommand) (*model.BodyHypothesisCommit, error) {
	f.calls++
	f.owner, f.target, f.withdraw = owner, target, c
	return f.commit, f.err
}
func (f *hypothesisHTTPFake) CorrectBodyHypothesis(_ context.Context, owner, target uuid.UUID, c model.CorrectBodyHypothesisCommand) (*model.BodyHypothesisCommit, error) {
	f.calls++
	f.owner, f.target, f.correct = owner, target, c
	return f.commit, f.err
}
func (f *hypothesisHTTPFake) ListActiveBodyHypotheses(_ context.Context, owner uuid.UUID) (*model.BodyHypothesisList, error) {
	f.calls++
	f.owner = owner
	return f.list, f.err
}
func (f *hypothesisHTTPFake) ListDerivedConcernGroups(_ context.Context, owner uuid.UUID) (*model.DerivedConcernGroupList, error) {
	f.calls++
	f.owner = owner
	return f.groups, f.err
}

func hypothesisHTTPRouter(t *testing.T, owner uuid.UUID, app bodyHypothesisApplication) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	spec, err := openapiv1.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	server := NewPublicServer()
	if app != nil {
		server = server.WithBodyHypotheses(app)
	}
	RegisterRoutes(router, StrictHandler(server), RouteSecurity{
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

func sampleHypothesisReceipt(owner uuid.UUID, op, key string, revision int64) *model.BodyHypothesisCommit {
	return &model.BodyHypothesisCommit{ID: uuid.New(), UserID: owner, IdempotencyKey: key, Operation: op,
		HypothesisID: uuid.New(), SourceEvidenceID: uuid.New(), Revision: revision, RevisionID: uuid.New(),
		EventID: uuid.New(), CommittedAt: time.Now().UTC()}
}

const hypothesisPath = "/api/v1/body-state/hypothesis-records"
const hypothesisJSON = `{"idempotency_key":"guess-1","expected_revision":0,"statement":"I think it is from running too much"}`

func TestBodyHypothesisHTTPRecordStrictDirectUserOnly(t *testing.T) {
	owner := uuid.New()
	fake := &hypothesisHTTPFake{commit: sampleHypothesisReceipt(owner, "record", "guess-1", 1)}
	router := hypothesisHTTPRouter(t, owner, fake)
	res := performJSONAt(router, http.MethodPost, hypothesisPath, hypothesisJSON)
	if res.Code != http.StatusOK || fake.calls != 1 || fake.owner != owner ||
		fake.record.Statement != "I think it is from running too much" ||
		!strings.Contains(res.Body.String(), fake.commit.HypothesisID.String()) ||
		strings.Contains(res.Body.String(), "running") || strings.Contains(res.Body.String(), "idempotency_key") {
		t.Fatalf("record: %d %s", res.Code, res.Body.String())
	}
	links := make([]string, 21)
	for i := range links {
		links[i] = `"` + uuid.NewString() + `"`
	}
	dup := uuid.NewString()
	invalid := []string{
		strings.Replace(hypothesisJSON, `"statement"`, `"user_id":"`+uuid.NewString()+`","statement"`, 1),
		strings.Replace(hypothesisJSON, `"statement"`, `"confidence":0.9,"statement"`, 1),
		strings.Replace(hypothesisJSON, `"statement"`, `"source":{"actor":"agent"},"statement"`, 1),
		strings.Replace(hypothesisJSON, `"statement"`, `"actor":"ai","statement"`, 1),
		strings.Replace(hypothesisJSON, `"expected_revision":0,`, "", 1),
		strings.Replace(hypothesisJSON, `"expected_revision":0,`, `"expected_revision":-1,`, 1),
		strings.Replace(hypothesisJSON, `"I think it is from running too much"`, `""`, 1),
		strings.Replace(hypothesisJSON, `"I think it is from running too much"`, `"  padded  "`, 1),
		strings.Replace(hypothesisJSON, `}`, `,"related_fact_ids":[`+strings.Join(links, ",")+`]}`, 1),
		strings.Replace(hypothesisJSON, `}`, `,"related_fact_ids":["`+dup+`","`+dup+`"]}`, 1),
		strings.Replace(hypothesisJSON, `}`, `,"related_fact_ids":["not-a-uuid"]}`, 1),
		strings.Replace(hypothesisJSON, `}`, `,"related_observation_ids":[`+strings.Join(links, ",")+`]}`, 1),
		strings.Replace(hypothesisJSON, `}`, `,"related_observation_ids":["`+dup+`","`+dup+`"]}`, 1),
		strings.Replace(hypothesisJSON, `}`, `,"supersedes_hypothesis_id":"`+dup+`"}`, 1),
	}
	for i, body := range invalid {
		if denied := performJSONAt(router, http.MethodPost, hypothesisPath, body); denied.Code != http.StatusBadRequest || fake.calls != 1 {
			t.Fatalf("invalid hypothesis[%d] reached service: %d %s calls=%d", i, denied.Code, denied.Body.String(), fake.calls)
		}
	}
	for name, forged := range map[string]func(*model.BodyHypothesisCommit){
		"foreign owner": func(c *model.BodyHypothesisCommit) { c.UserID = uuid.New() },
		"other key":     func(c *model.BodyHypothesisCommit) { c.IdempotencyKey = "other" },
		"wrong rev":     func(c *model.BodyHypothesisCommit) { c.Revision = 2 },
		"wrong op":      func(c *model.BodyHypothesisCommit) { c.Operation = "withdraw" },
		"target set":    func(c *model.BodyHypothesisCommit) { id := uuid.New(); c.TargetHypothesisID = &id },
	} {
		fake.commit = sampleHypothesisReceipt(owner, "record", "guess-1", 1)
		forged(fake.commit)
		if r := performJSONAt(router, http.MethodPost, hypothesisPath, hypothesisJSON); r.Code != http.StatusInternalServerError {
			t.Fatalf("%s receipt accepted: %d", name, r.Code)
		}
	}
	fake.commit = nil
	for err, want := range map[error]int{
		model.ErrBodyHypothesisFactNotFound:                          http.StatusNotFound,
		model.ErrBodyHypothesisObservationNotFound:                   http.StatusNotFound,
		fmt.Errorf("%w: x", repository.ErrBodyStateRevisionConflict): http.StatusConflict,
		model.ErrBodyHypothesisIdempotencyConflict:                   http.StatusConflict,
		fmt.Errorf("%w: bad", model.ErrInvalidBodyHypothesis):        http.StatusBadRequest,
		fmt.Errorf("backend private guess statement"):                http.StatusInternalServerError,
	} {
		fake.err = err
		r := performJSONAt(router, http.MethodPost, hypothesisPath, hypothesisJSON)
		if r.Code != want || strings.Contains(r.Body.String(), "private guess") {
			t.Fatalf("%v → %d %s, want %d", err, r.Code, r.Body.String(), want)
		}
	}
	if anon := performJSONAt(hypothesisHTTPRouter(t, uuid.Nil, fake), http.MethodPost, hypothesisPath, hypothesisJSON); anon.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous record accepted: %d", anon.Code)
	}
	if missing := performJSONAt(hypothesisHTTPRouter(t, owner, nil), http.MethodPost, hypothesisPath, hypothesisJSON); missing.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing service represented success: %d", missing.Code)
	}
}

func TestBodyHypothesisHTTPWithdrawErrors(t *testing.T) {
	owner, target := uuid.New(), uuid.New()
	path := hypothesisPath + "/" + target.String() + "/withdraw"
	body := `{"idempotency_key":"wd-1","expected_revision":4,"reason":"physio ruled it out"}`
	receipt := sampleHypothesisReceipt(owner, "withdraw", "wd-1", 5)
	receipt.HypothesisID, receipt.TargetHypothesisID = target, &target
	fake := &hypothesisHTTPFake{commit: receipt}
	router := hypothesisHTTPRouter(t, owner, fake)
	if ok := performJSONAt(router, http.MethodPost, path, body); ok.Code != http.StatusOK || fake.target != target ||
		fake.withdraw.Reason != "physio ruled it out" || strings.Contains(ok.Body.String(), "physio") {
		t.Fatalf("withdraw: %d %s", ok.Code, ok.Body.String())
	}
	fake.commit = sampleHypothesisReceipt(owner, "withdraw", "wd-1", 5) // different hypothesis id
	if forged := performJSONAt(router, http.MethodPost, path, body); forged.Code != http.StatusInternalServerError {
		t.Fatalf("receipt for another hypothesis accepted: %d", forged.Code)
	}
	fake.commit = nil
	fake.err = model.ErrBodyHypothesisNotCurrent
	if r := performJSONAt(router, http.MethodPost, path, body); r.Code != http.StatusConflict ||
		!strings.Contains(r.Body.String(), "HYPOTHESIS_NOT_CURRENT") {
		t.Fatalf("not-current: %d %s", r.Code, r.Body.String())
	}
	fake.err = model.ErrBodyHypothesisNotFound
	if r := performJSONAt(router, http.MethodPost, path, body); r.Code != http.StatusNotFound ||
		!strings.Contains(r.Body.String(), "HYPOTHESIS_NOT_FOUND") {
		t.Fatalf("not-found: %d %s", r.Code, r.Body.String())
	}
	calls := fake.calls
	for i, bad := range []struct{ path, body string }{
		{hypothesisPath + "/not-a-uuid/withdraw", body},
		{path, `{"idempotency_key":"wd-1"}`},
		{path, `{"idempotency_key":"wd-1","expected_revision":4,"reason":" padded "}`},
		{path, `{"idempotency_key":"wd-1","expected_revision":4,"reason":"` + strings.Repeat("x", 501) + `"}`},
		{path, `{"idempotency_key":"wd-1","expected_revision":4,"user_id":"` + uuid.NewString() + `"}`},
	} {
		if r := performJSONAt(router, http.MethodPost, bad.path, bad.body); r.Code != http.StatusBadRequest || fake.calls != calls {
			t.Fatalf("invalid withdrawal[%d] reached service: %d %s", i, r.Code, r.Body.String())
		}
	}
	if anon := performJSONAt(hypothesisHTTPRouter(t, uuid.Nil, fake), http.MethodPost, path, body); anon.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous withdraw accepted: %d", anon.Code)
	}
}

func TestBodyHypothesisHTTPCorrectStrictReceipt(t *testing.T) {
	owner, target := uuid.New(), uuid.New()
	path := hypothesisPath + "/" + target.String() + "/correct"
	fact, obs := uuid.New(), uuid.New()
	body := `{"idempotency_key":"fix-1","expected_revision":4,"replacement":{"statement":"actually my new shoes",` +
		`"related_fact_ids":["` + fact.String() + `"],"related_observation_ids":["` + obs.String() + `"]}}`
	receipt := sampleHypothesisReceipt(owner, "correct", "fix-1", 5)
	receipt.TargetHypothesisID = &target
	fake := &hypothesisHTTPFake{commit: receipt}
	router := hypothesisHTTPRouter(t, owner, fake)
	ok := performJSONAt(router, http.MethodPost, path, body)
	if ok.Code != http.StatusOK || fake.target != target || fake.owner != owner || fake.correct.Replacement == nil ||
		fake.correct.Replacement.Statement != "actually my new shoes" ||
		len(fake.correct.Replacement.RelatedFactIDs) != 1 || fake.correct.Replacement.RelatedObservationIDs[0] != obs ||
		strings.Contains(ok.Body.String(), "shoes") || !strings.Contains(ok.Body.String(), `"target_hypothesis_id":"`+target.String()) ||
		!strings.Contains(ok.Body.String(), `"operation":"correct"`) {
		t.Fatalf("correct: %d %s", ok.Code, ok.Body.String())
	}
	for name, forged := range map[string]func(*model.BodyHypothesisCommit){
		"no target":      func(c *model.BodyHypothesisCommit) { c.TargetHypothesisID = nil },
		"other target":   func(c *model.BodyHypothesisCommit) { id := uuid.New(); c.TargetHypothesisID = &id },
		"self successor": func(c *model.BodyHypothesisCommit) { c.HypothesisID = target },
		"wrong op":       func(c *model.BodyHypothesisCommit) { c.Operation = "record" },
		"wrong rev":      func(c *model.BodyHypothesisCommit) { c.Revision = 9 },
	} {
		fake.commit = sampleHypothesisReceipt(owner, "correct", "fix-1", 5)
		fake.commit.TargetHypothesisID = &target
		forged(fake.commit)
		if r := performJSONAt(router, http.MethodPost, path, body); r.Code != http.StatusInternalServerError {
			t.Fatalf("%s correction receipt accepted: %d", name, r.Code)
		}
	}
	fake.commit = nil
	for err, want := range map[error]string{
		model.ErrBodyHypothesisNotCurrent:          "HYPOTHESIS_NOT_CURRENT",
		model.ErrBodyHypothesisNotFound:            "HYPOTHESIS_NOT_FOUND",
		model.ErrBodyHypothesisFactNotFound:        "HYPOTHESIS_FACT_NOT_FOUND",
		model.ErrBodyHypothesisObservationNotFound: "HYPOTHESIS_OBSERVATION_NOT_FOUND",
		model.ErrBodyHypothesisIdempotencyConflict: "HYPOTHESIS_IDEMPOTENCY_CONFLICT",
	} {
		fake.err = err
		if r := performJSONAt(router, http.MethodPost, path, body); !strings.Contains(r.Body.String(), want) {
			t.Fatalf("%v: %d %s", err, r.Code, r.Body.String())
		}
	}
	calls := fake.calls
	for i, bad := range []struct{ path, body string }{
		{hypothesisPath + "/not-a-uuid/correct", body},
		{path, `{"idempotency_key":"fix-1","expected_revision":4}`},
		{path, `{"idempotency_key":"fix-1","expected_revision":4,"replacement":{"statement":"x"}}`},
		{path, `{"idempotency_key":"fix-1","expected_revision":4,"replacement":{"statement":" x","related_fact_ids":[],"related_observation_ids":[]}}`},
		{path, `{"idempotency_key":"fix-1","expected_revision":4,"replacement":{"statement":"x","related_fact_ids":[],"related_observation_ids":[],"confidence":1}}`},
		{path, `{"idempotency_key":"fix-1","expected_revision":4,"actor":"agent","replacement":{"statement":"x","related_fact_ids":[],"related_observation_ids":[]}}`},
		{path, `{"idempotency_key":"fix-1","expected_revision":4,"replacement":{"statement":"x","related_fact_ids":["` + fact.String() + `","` + fact.String() + `"],"related_observation_ids":[]}}`},
	} {
		if r := performJSONAt(router, http.MethodPost, bad.path, bad.body); r.Code != http.StatusBadRequest || fake.calls != calls {
			t.Fatalf("invalid correction[%d] reached service: %d %s", i, r.Code, r.Body.String())
		}
	}
	if anon := performJSONAt(hypothesisHTTPRouter(t, uuid.Nil, fake), http.MethodPost, path, body); anon.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous correction accepted: %d", anon.Code)
	}
}

func TestBodyHypothesisHTTPReadsOwnerScopedAndContractChecked(t *testing.T) {
	owner := uuid.New()
	rec := model.BodyHypothesisRecord{ID: uuid.New(), UserID: owner, Statement: "maybe posture",
		Source:         model.BodyHypothesisSource{Actor: "user", Kind: "direct_edit", ID: uuid.New(), Version: 1},
		RelatedFactIDs: []uuid.UUID{}, RelatedObservationIDs: []uuid.UUID{uuid.New()},
		AssertionStatus: "asserted", CreatedRevision: 2, RecordedAt: time.Now().UTC()}
	rec.LineageID = rec.ID
	fake := &hypothesisHTTPFake{list: &model.BodyHypothesisList{UserID: owner, CurrentRevision: 2, Hypotheses: []model.BodyHypothesisRecord{rec}}}
	router := hypothesisHTTPRouter(t, owner, fake)
	if r := performJSONAt(router, http.MethodGet, hypothesisPath, ""); r.Code != http.StatusOK || fake.owner != owner ||
		!strings.Contains(r.Body.String(), `"actor":"user"`) || !strings.Contains(r.Body.String(), "maybe posture") {
		t.Fatalf("list: %d %s", r.Code, r.Body.String())
	}
	fake.list.Hypotheses[0].LineageID = uuid.Nil
	if r := performJSONAt(router, http.MethodGet, hypothesisPath, ""); r.Code != http.StatusInternalServerError {
		t.Fatalf("record without lineage exposed: %d", r.Code)
	}
	fake.list.Hypotheses[0].LineageID = rec.ID
	fake.list.Hypotheses[0].UserID = uuid.New()
	if r := performJSONAt(router, http.MethodGet, hypothesisPath, ""); r.Code != http.StatusInternalServerError {
		t.Fatalf("foreign record leaked: %d", r.Code)
	}
	fake.list.Hypotheses[0].UserID = owner
	fake.list.Hypotheses[0].Source.Actor = "agent"
	if r := performJSONAt(router, http.MethodGet, hypothesisPath, ""); r.Code != http.StatusInternalServerError {
		t.Fatalf("agent-sourced hypothesis exposed as user's: %d", r.Code)
	}
	fake.list.Hypotheses[0].Source.Actor = "user"
	fake.list.UserID = uuid.New()
	if r := performJSONAt(router, http.MethodGet, hypothesisPath, ""); r.Code != http.StatusInternalServerError {
		t.Fatalf("foreign list leaked: %d", r.Code)
	}

	groupsPath := "/api/v1/body-state/concern-groups"
	group := model.DerivedConcernGroup{ID: uuid.New(), DisplayTitle: "maybe posture", LatestRevision: 2,
		RelatedHypothesisIDs: []uuid.UUID{rec.ID}, RelatedFactIDs: []uuid.UUID{}, RelatedEpisodeIDs: []uuid.UUID{},
		RelatedObservationIDs: []uuid.UUID{}}
	fake.groups = &model.DerivedConcernGroupList{UserID: owner, CurrentRevision: 2, Groups: []model.DerivedConcernGroup{group}}
	if r := performJSONAt(router, http.MethodGet, groupsPath, ""); r.Code != http.StatusOK ||
		!strings.Contains(r.Body.String(), `"related_observation_ids":[]`) || !strings.Contains(r.Body.String(), group.ID.String()) {
		t.Fatalf("groups: %d %s", r.Code, r.Body.String())
	}
	obs := uuid.New()
	fake.groups.Groups[0].RelatedObservationIDs = []uuid.UUID{obs}
	if r := performJSONAt(router, http.MethodGet, groupsPath, ""); r.Code != http.StatusOK || !strings.Contains(r.Body.String(), obs.String()) {
		t.Fatalf("observation-linked group: %d %s", r.Code, r.Body.String())
	}
	fake.groups.Groups[0].RelatedObservationIDs = []uuid.UUID{}
	fake.groups.Groups[0].RelatedHypothesisIDs = []uuid.UUID{}
	if r := performJSONAt(router, http.MethodGet, groupsPath, ""); r.Code != http.StatusInternalServerError {
		t.Fatalf("fact-only group exposed: %d", r.Code)
	}
	fake.groups.Groups[0].RelatedHypothesisIDs = []uuid.UUID{rec.ID}
	fake.groups.UserID = uuid.New()
	if r := performJSONAt(router, http.MethodGet, groupsPath, ""); r.Code != http.StatusInternalServerError {
		t.Fatalf("foreign groups leaked: %d", r.Code)
	}
	for _, p := range []string{hypothesisPath, groupsPath} {
		if anon := performJSONAt(hypothesisHTTPRouter(t, uuid.Nil, fake), http.MethodGet, p, ""); anon.Code != http.StatusUnauthorized {
			t.Fatalf("anonymous read %s accepted: %d", p, anon.Code)
		}
	}
}

func TestBodyHypothesisHTTPRealRoundTripPostgres(t *testing.T) {
	dsn := os.Getenv("BODYSENSE_INTEGRATION_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated native PostgreSQL 18 required")
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
			"hypothesis-http-"+id.String()+"@example.test", "synthetic").Error; err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Exec("DELETE FROM users WHERE id=?", id).Error })
	}
	app := service.NewBodyHypothesisService(repository.NewBodyStateRepository(db))
	ownerRouter := hypothesisHTTPRouter(t, owner, app)
	foreignRouter := hypothesisHTTPRouter(t, foreign, app)

	posted := performJSONAt(ownerRouter, http.MethodPost, hypothesisPath, hypothesisJSON)
	if posted.Code != http.StatusOK {
		t.Fatalf("record: %d %s", posted.Code, posted.Body.String())
	}
	var commit openapiv1.BodyHypothesisCommit
	if err := json.Unmarshal(posted.Body.Bytes(), &commit); err != nil || commit.Revision != 1 || commit.Operation != "record" {
		t.Fatalf("commit: %#v %v", commit, err)
	}
	if replay := performJSONAt(ownerRouter, http.MethodPost, hypothesisPath, hypothesisJSON); replay.Code != http.StatusOK ||
		!strings.Contains(replay.Body.String(), commit.Id.String()) {
		t.Fatalf("replay: %d %s", replay.Code, replay.Body.String())
	}
	if stale := performJSONAt(ownerRouter, http.MethodPost, hypothesisPath,
		strings.Replace(hypothesisJSON, "guess-1", "guess-2", 1)); stale.Code != http.StatusConflict {
		t.Fatalf("stale revision accepted: %d", stale.Code)
	}
	if foreignLink := performJSONAt(foreignRouter, http.MethodPost, hypothesisPath,
		`{"idempotency_key":"f","expected_revision":0,"statement":"x","related_fact_ids":["`+uuid.NewString()+`"]}`); foreignLink.Code != http.StatusNotFound {
		t.Fatalf("unknown fact link: %d", foreignLink.Code)
	}
	if list := performJSONAt(foreignRouter, http.MethodGet, hypothesisPath, ""); list.Code != http.StatusOK ||
		strings.Contains(list.Body.String(), commit.HypothesisId.String()) {
		t.Fatalf("foreign owner read hypothesis: %d %s", list.Code, list.Body.String())
	}
	if groups := performJSONAt(foreignRouter, http.MethodGet, "/api/v1/body-state/concern-groups", ""); groups.Code != http.StatusOK ||
		strings.Contains(groups.Body.String(), commit.HypothesisId.String()) {
		t.Fatalf("foreign owner read groups: %d %s", groups.Code, groups.Body.String())
	}
	if groups := performJSONAt(ownerRouter, http.MethodGet, "/api/v1/body-state/concern-groups", ""); groups.Code != http.StatusOK ||
		!strings.Contains(groups.Body.String(), commit.HypothesisId.String()) {
		t.Fatalf("owner groups: %d %s", groups.Code, groups.Body.String())
	}
	withdrawPath := hypothesisPath + "/" + commit.HypothesisId.String() + "/withdraw"
	if r := performJSONAt(foreignRouter, http.MethodPost, withdrawPath, `{"idempotency_key":"fw","expected_revision":0}`); r.Code != http.StatusNotFound {
		t.Fatalf("foreign withdraw: %d %s", r.Code, r.Body.String())
	}
	if r := performJSONAt(ownerRouter, http.MethodPost, withdrawPath, `{"idempotency_key":"wd","expected_revision":1}`); r.Code != http.StatusOK {
		t.Fatalf("withdraw: %d %s", r.Code, r.Body.String())
	}
	if r := performJSONAt(ownerRouter, http.MethodPost, withdrawPath, `{"idempotency_key":"wd-again","expected_revision":2}`); r.Code != http.StatusConflict ||
		!strings.Contains(r.Body.String(), "HYPOTHESIS_NOT_CURRENT") {
		t.Fatalf("double withdraw: %d %s", r.Code, r.Body.String())
	}
	if list := performJSONAt(ownerRouter, http.MethodGet, hypothesisPath, ""); list.Code != http.StatusOK ||
		strings.Contains(list.Body.String(), commit.HypothesisId.String()) || !strings.Contains(list.Body.String(), `"current_revision":2`) {
		t.Fatalf("withdrawn still listed: %d %s", list.Code, list.Body.String())
	}

	// Correction: rev3 record, rev4 correct (same lineage), withdrawn target 409.
	second := performJSONAt(ownerRouter, http.MethodPost, hypothesisPath,
		`{"idempotency_key":"guess-3","expected_revision":2,"statement":"maybe the new mattress"}`)
	var secondCommit openapiv1.BodyHypothesisCommit
	if second.Code != http.StatusOK || json.Unmarshal(second.Body.Bytes(), &secondCommit) != nil {
		t.Fatalf("second record: %d %s", second.Code, second.Body.String())
	}
	correctBody := `{"idempotency_key":"fix","expected_revision":3,"replacement":{"statement":"actually the old pillow",` +
		`"related_fact_ids":[],"related_observation_ids":[]}}`
	correctPath := hypothesisPath + "/" + secondCommit.HypothesisId.String() + "/correct"
	if r := performJSONAt(foreignRouter, http.MethodPost, correctPath, correctBody); r.Code != http.StatusNotFound {
		t.Fatalf("foreign correction: %d %s", r.Code, r.Body.String())
	}
	fixed := performJSONAt(ownerRouter, http.MethodPost, correctPath, correctBody)
	var fixedCommit openapiv1.BodyHypothesisCommit
	if fixed.Code != http.StatusOK || json.Unmarshal(fixed.Body.Bytes(), &fixedCommit) != nil || fixedCommit.Revision != 4 ||
		fixedCommit.TargetHypothesisId == nil || *fixedCommit.TargetHypothesisId != secondCommit.HypothesisId {
		t.Fatalf("correct: %d %s", fixed.Code, fixed.Body.String())
	}
	if r := performJSONAt(ownerRouter, http.MethodPost, correctPath, correctBody); r.Code != http.StatusOK ||
		!strings.Contains(r.Body.String(), fixedCommit.Id.String()) {
		t.Fatalf("correction replay: %d %s", r.Code, r.Body.String())
	}
	if r := performJSONAt(ownerRouter, http.MethodPost, correctPath,
		strings.Replace(strings.Replace(correctBody, `"fix"`, `"fix-2"`, 1), `:3,`, `:4,`, 1)); r.Code != http.StatusConflict {
		t.Fatalf("superseded target corrected again: %d %s", r.Code, r.Body.String())
	}
	list := performJSONAt(ownerRouter, http.MethodGet, hypothesisPath, "")
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), "old pillow") || strings.Contains(list.Body.String(), "mattress") ||
		!strings.Contains(list.Body.String(), `"lineage_id":"`+secondCommit.HypothesisId.String()) ||
		!strings.Contains(list.Body.String(), `"supersedes_hypothesis_id":"`+secondCommit.HypothesisId.String()) {
		t.Fatalf("corrected list: %d %s", list.Code, list.Body.String())
	}
}
