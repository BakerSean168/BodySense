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
)

type episodeCommandHTTPFake struct {
	calls             int
	owner             uuid.UUID
	target            uuid.UUID
	command           model.EpisodeAssociationCommand
	receipt           *model.EpisodeCommandReceipt
	recurrenceCommand model.EpisodeRecurrenceCommand
	recurrenceReceipt *model.EpisodeRecurrenceReceipt
	err               error
}

func (f *episodeCommandHTTPFake) CreateEpisode(_ context.Context, owner uuid.UUID, c model.EpisodeAssociationCommand) (*model.EpisodeCommandReceipt, error) {
	f.calls++
	f.owner = owner
	f.target = uuid.Nil
	f.command = c
	return f.receipt, f.err
}
func (f *episodeCommandHTTPFake) AdmitEpisodeStatus(_ context.Context, owner, episode uuid.UUID, c model.EpisodeAssociationCommand) (*model.EpisodeCommandReceipt, error) {
	f.calls++
	f.owner = owner
	f.target = episode
	f.command = c
	return f.receipt, f.err
}
func (f *episodeCommandHTTPFake) ReportEpisodeRecurrence(_ context.Context, owner, old uuid.UUID, c model.EpisodeRecurrenceCommand) (*model.EpisodeRecurrenceReceipt, error) {
	f.calls++
	f.owner = owner
	f.target = old
	f.recurrenceCommand = c
	return f.recurrenceReceipt, f.err
}
func (f *episodeCommandHTTPFake) LinkEpisodeFact(_ context.Context, owner, episode uuid.UUID, c model.EpisodeAssociationCommand) (*model.EpisodeCommandReceipt, error) {
	f.calls++
	f.owner = owner
	f.target = episode
	f.command = c
	return f.receipt, f.err
}

func episodeCommandRouter(t *testing.T, owner uuid.UUID, application episodeCommandApplication) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	spec, err := openapiv1.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	engine := gin.New()
	RegisterRoutes(engine, StrictHandler(NewPublicServer().WithEpisodeCommands(application)), RouteSecurity{
		Auth: func(c *gin.Context) {
			if owner != uuid.Nil {
				c.Set("user_id", owner.String())
			}
			c.Next()
		},
		Operator:  func(c *gin.Context) { c.Next() },
		Validator: RequestValidator(spec),
	})
	return engine
}
func testEpisodeReceipt(owner, episode, fact uuid.UUID, operation string) *model.EpisodeCommandReceipt {
	return &model.EpisodeCommandReceipt{
		ID: uuid.New(), UserID: owner, IdempotencyKey: "explicit-create", Fingerprint: strings.Repeat("f", 64),
		Operation: operation, EpisodeID: episode, FactID: fact,
		Revision: 2, RevisionID: uuid.New(), EventID: uuid.New(), CommittedAt: time.Now().UTC(),
	}
}

const episodeCommandBody = `{"idempotency_key":"explicit-create","expected_revision":1,"fact_id":"3456789a-3456-4456-9456-3456789abcde"}`

func TestBodyEpisodeHTTPGeneratedCommandAndExactReceipt(t *testing.T) {
	owner, episode, fact := uuid.New(), uuid.New(), uuid.MustParse("3456789a-3456-4456-9456-3456789abcde")
	fake := &episodeCommandHTTPFake{receipt: testEpisodeReceipt(owner, episode, fact, "create")}
	router := episodeCommandRouter(t, owner, fake)
	saved := performJSONAt(router, http.MethodPost, "/api/v1/body-state/episodes", episodeCommandBody)
	if saved.Code != http.StatusOK || fake.calls != 1 || fake.owner != owner || fake.command.FactID != fact ||
		!strings.Contains(saved.Body.String(), episode.String()) ||
		strings.Contains(saved.Body.String(), "fingerprint") || strings.Contains(saved.Body.String(), "idempotency_key") {
		t.Fatalf("missing or privacy-unsafe receipt: status=%d body=%s calls=%d", saved.Code, saved.Body.String(), fake.calls)
	}
	fake.receipt = testEpisodeReceipt(owner, episode, fact, "link")
	linked := performJSONAt(router, http.MethodPost, "/api/v1/body-state/episodes/"+episode.String()+"/facts", episodeCommandBody)
	if linked.Code != http.StatusOK || fake.calls != 2 || fake.target != episode || fake.command.FactID != fact ||
		!strings.Contains(linked.Body.String(), `"operation":"link"`) {
		t.Fatalf("missing owner-bound link receipt: status=%d body=%s", linked.Code, linked.Body.String())
	}
}

func TestBodyEpisodeHTTPRejectsForgedClinicalAndSourceFieldsBeforeCommand(t *testing.T) {
	owner := uuid.New()
	fake := &episodeCommandHTTPFake{}
	router := episodeCommandRouter(t, owner, fake)
	invalid := []string{
		strings.Replace(episodeCommandBody, `"fact_id":`, `"user_id":"`+uuid.NewString()+`","fact_id":`, 1),
		strings.Replace(episodeCommandBody, `"fact_id":`, `"source_evidence_id":"`+uuid.NewString()+`","fact_id":`, 1),
		strings.Replace(episodeCommandBody, `"fact_id":`, `"state":"ended","fact_id":`, 1),
		strings.Replace(episodeCommandBody, `"fact_id":`, `"symptom_type":"diagnosed migraine","fact_id":`, 1),
		strings.Replace(episodeCommandBody, `"fact_id":`, `"authorization_id":"`+uuid.NewString()+`","fact_id":`, 1),
		strings.Replace(episodeCommandBody, `"fact_id":`, `"role":"resolution","fact_id":`, 1),
		strings.Replace(episodeCommandBody, `"expected_revision":1,`, "", 1),
		strings.Replace(episodeCommandBody, `"expected_revision":1,`, `"expected_revision":-1,`, 1),
		strings.Replace(episodeCommandBody, `"fact_id":"3456789a-3456-4456-9456-3456789abcde"`, `"fact_id":"not-uuid"`, 1),
	}
	for i, body := range invalid {
		response := performJSONAt(router, http.MethodPost, "/api/v1/body-state/episodes", body)
		if response.Code != http.StatusBadRequest || fake.calls != 0 {
			t.Fatalf("forged Episode request[%d] allowed: %d %s", i, response.Code, response.Body.String())
		}
	}
	malformed := performJSONAt(router, http.MethodPost, "/api/v1/body-state/episodes/not-a-uuid/facts", episodeCommandBody)
	if malformed.Code != http.StatusBadRequest || fake.calls != 0 {
		t.Fatalf("invalid Episode target reached command: %d", malformed.Code)
	}
	anon := performJSONAt(episodeCommandRouter(t, uuid.Nil, fake), http.MethodPost, "/api/v1/body-state/episodes", episodeCommandBody)
	if anon.Code != http.StatusUnauthorized || fake.calls != 0 {
		t.Fatalf("anonymous command executed: %d calls=%d", anon.Code, fake.calls)
	}
	unavailable := performJSONAt(episodeCommandRouter(t, owner, nil), http.MethodPost, "/api/v1/body-state/episodes", episodeCommandBody)
	if unavailable.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing service accepted: %d", unavailable.Code)
	}
}

func TestBodyEpisodeHTTPDenialsAndNoUncommittedSuccess(t *testing.T) {
	owner, episode, fact := uuid.New(), uuid.New(), uuid.MustParse("3456789a-3456-4456-9456-3456789abcde")
	fake := &episodeCommandHTTPFake{}
	router := episodeCommandRouter(t, owner, fake)
	cases := []struct {
		label   string
		err     error
		receipt *model.EpisodeCommandReceipt
		status  int
	}{
		{"failed_tx", errors.New("private diagnosis payload 567"), nil, http.StatusInternalServerError},
		{"nil_receipt", nil, nil, http.StatusInternalServerError},
		{"wrong_owner", nil, testEpisodeReceipt(uuid.New(), episode, fact, "create"), http.StatusInternalServerError},
		{"wrong_fact", nil, testEpisodeReceipt(owner, episode, uuid.New(), "create"), http.StatusInternalServerError},
		{"wrong_operation", nil, testEpisodeReceipt(owner, episode, fact, "link"), http.StatusInternalServerError},
		{"stale", repository.ErrBodyStateRevisionConflict, nil, http.StatusConflict},
		{"same_key_other_payload", model.ErrEpisodeIdempotencyConflict, nil, http.StatusConflict},
		{"incompatible", model.ErrEpisodeAssociationConflict, nil, http.StatusConflict},
		{"foreign_or_unavailable", model.ErrEpisodeFactUnavailable, nil, http.StatusNotFound},
		{"invalid", model.ErrInvalidEpisodeEvidence, nil, http.StatusBadRequest},
	}
	for _, item := range cases {
		t.Run(item.label, func(t *testing.T) {
			fake.receipt, fake.err = item.receipt, item.err
			response := performJSONAt(router, http.MethodPost, "/api/v1/body-state/episodes", episodeCommandBody)
			if response.Code != item.status || strings.Contains(response.Body.String(), "private diagnosis payload") {
				t.Fatalf("untrusted result reported success or leaked text: %d %s", response.Code, response.Body.String())
			}
		})
	}
	fake.err = nil
	fake.receipt = testEpisodeReceipt(owner, uuid.New(), fact, "link")
	mismatched := performJSONAt(router, http.MethodPost, "/api/v1/body-state/episodes/"+episode.String()+"/facts", episodeCommandBody)
	if mismatched.Code != http.StatusInternalServerError {
		t.Fatalf("receipt for a different target accepted: %d", mismatched.Code)
	}
}
