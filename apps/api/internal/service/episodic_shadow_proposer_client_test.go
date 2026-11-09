package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"github.com/google/uuid"
)

// The golden wire fixture is shared with the ai-service route tests so both
// sides evolve the private proposal transport from one sample pair.
const episodicShadowWireFixturePath = "../../../../contracts/internal/agent-runtime/fixtures/episodic-shadow-wire.v1.json"

type episodicShadowWireMessage struct {
	ID   string `json:"id"`
	Seq  int    `json:"seq"`
	Role string `json:"role"`
	Text string `json:"text"`
}

type episodicShadowWireCommand struct {
	ConversationID  string                      `json:"conversation_id"`
	UserID          string                      `json:"user_id"`
	FromSeq         int                         `json:"from_seq"`
	ToSeq           int                         `json:"to_seq"`
	PolicyRevision  string                      `json:"policy_revision"`
	SourceMessages  []episodicShadowWireMessage `json:"source_messages"`
	ContextMessages []episodicShadowWireMessage `json:"context_messages"`
}

type episodicShadowWireProposal struct {
	PolicyRevision string `json:"policy_revision"`
	Quotes         []struct {
		SourceMessageID string `json:"source_message_id"`
		Quote           string `json:"quote"`
		Preference      string `json:"preference"`
	} `json:"quotes"`
}

type episodicShadowWireFixture struct {
	Command  json.RawMessage `json:"command"`
	Proposal json.RawMessage `json:"proposal"`
}

func loadEpisodicShadowWireFixture(t *testing.T) episodicShadowWireFixture {
	t.Helper()
	raw, err := os.ReadFile(episodicShadowWireFixturePath)
	if err != nil {
		t.Fatal(err)
	}
	var fixture episodicShadowWireFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func episodicShadowRequestFromWire(
	t *testing.T, command episodicShadowWireCommand,
) EpisodicShadowProposalRequest {
	t.Helper()
	request := EpisodicShadowProposalRequest{
		Owner:          uuid.MustParse(command.UserID),
		ConversationID: uuid.MustParse(command.ConversationID),
		FromSeq:        command.FromSeq,
		ToSeq:          command.ToSeq,
	}
	for _, message := range command.SourceMessages {
		request.Source = append(request.Source, EpisodicShadowSourceMessage{
			ID:   uuid.MustParse(message.ID),
			Seq:  message.Seq,
			Role: message.Role,
			Text: message.Text,
		})
	}
	for _, message := range command.ContextMessages {
		request.Context = append(request.Context, EpisodicShadowSourceMessage{
			ID:   uuid.MustParse(message.ID),
			Seq:  message.Seq,
			Role: message.Role,
			Text: message.Text,
		})
	}
	return request
}

func TestEpisodicShadowHTTPProposerUsesGoldenWireFormat(t *testing.T) {
	fixture := loadEpisodicShadowWireFixture(t)
	var command episodicShadowWireCommand
	if err := json.Unmarshal(fixture.Command, &command); err != nil {
		t.Fatal(err)
	}
	var (
		path, method, contentType string
		body                      []byte
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, method, contentType = r.URL.Path, r.Method, r.Header.Get("Content-Type")
		body, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture.Proposal)
	}))
	defer server.Close()

	proposer := NewEpisodicShadowHTTPProposer(&AIClient{
		httpClient: server.Client(),
		baseURL:    server.URL,
	})
	response, err := proposer.ProposeEpisodicShadowQuote(
		context.Background(), episodicShadowRequestFromWire(t, command),
	)
	if err != nil {
		t.Fatalf("proposal call failed: %v", err)
	}
	if path != episodicShadowProposalPath || method != http.MethodPost {
		t.Fatalf("unexpected transport: %s %s", method, path)
	}
	if contentType != "application/json" {
		t.Fatalf("unexpected content type %q", contentType)
	}
	// The request must be the same command, in the same field names, that the
	// ai-service adapter parses from the shared fixture.
	var got, want map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("sent body is not JSON: %v", err)
	}
	if err := json.Unmarshal(fixture.Command, &want); err != nil {
		t.Fatalf("golden command is not JSON: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("wire command diverged:\n got: %s\nwant: %s", body, fixture.Command)
	}

	var expected episodicShadowWireProposal
	if err := json.Unmarshal(fixture.Proposal, &expected); err != nil {
		t.Fatalf("golden proposal is not JSON: %v", err)
	}
	if len(response.Quotes) != len(expected.Quotes) {
		t.Fatalf("quote count %d, want %d", len(response.Quotes), len(expected.Quotes))
	}
	for i, quote := range response.Quotes {
		want := expected.Quotes[i]
		if quote.SourceMessageID.String() != want.SourceMessageID ||
			quote.Quote != want.Quote ||
			quote.Preference != want.Preference {
			t.Fatalf("quote %d diverged: %+v", i, quote)
		}
	}
}

func TestEpisodicShadowHTTPProposerFailsOnUpstreamStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "gateway unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	proposer := NewEpisodicShadowHTTPProposer(&AIClient{
		httpClient: server.Client(),
		baseURL:    server.URL,
	})
	_, err := proposer.ProposeEpisodicShadowQuote(
		context.Background(), episodicShadowRequestFromDiscovery(t),
	)
	if !errors.Is(err, ErrEpisodicShadowProposalFailed) {
		t.Fatalf("upstream error is not a proposal failure: %v", err)
	}
}

func TestEpisodicShadowHTTPProposerFailsOnBadShapeWithoutInventingQuotes(t *testing.T) {
	for name, payload := range map[string]string{
		"unknown field": `{"policy_revision":"episodic-shadow-quote-v1","quotes":[],"content_text":"伪造诊断"}`,
		"bad uuid":      `{"policy_revision":"episodic-shadow-quote-v1","quotes":[{"source_message_id":"not-a-uuid","quote":"请简短回答。","preference":"brief_answers"}]}`,
		"bad policy":    `{"policy_revision":"other","quotes":[]}`,
		"not json":      `not-json`,
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(payload))
			}))
			defer server.Close()
			proposer := NewEpisodicShadowHTTPProposer(&AIClient{
				httpClient: server.Client(),
				baseURL:    server.URL,
			})
			response, err := proposer.ProposeEpisodicShadowQuote(
				context.Background(), episodicShadowRequestFromDiscovery(t),
			)
			if !errors.Is(err, ErrEpisodicShadowProposalFailed) {
				t.Fatalf("bad shape is not a proposal failure: %v", err)
			}
			if len(response.Quotes) != 0 {
				t.Fatalf("bad shape produced quotes: %+v", response.Quotes)
			}
		})
	}
}

func TestEpisodicShadowHTTPProposerKeepsAbstentionEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"policy_revision":"episodic-shadow-quote-v1"}`))
	}))
	defer server.Close()
	proposer := NewEpisodicShadowHTTPProposer(&AIClient{
		httpClient: server.Client(),
		baseURL:    server.URL,
	})
	response, err := proposer.ProposeEpisodicShadowQuote(
		context.Background(), episodicShadowRequestFromDiscovery(t),
	)
	if err != nil {
		t.Fatalf("abstention is not failure: %v", err)
	}
	if len(response.Quotes) != 0 {
		t.Fatalf("abstention invented quotes: %+v", response.Quotes)
	}
}

func TestEpisodicShadowHTTPProposerWithoutClientFails(t *testing.T) {
	proposer := NewEpisodicShadowHTTPProposer(nil)
	_, err := proposer.ProposeEpisodicShadowQuote(
		context.Background(), episodicShadowRequestFromDiscovery(t),
	)
	if !errors.Is(err, ErrEpisodicShadowProposalFailed) {
		t.Fatalf("missing client is not a proposal failure: %v", err)
	}
}

// episodicShadowRequestFromDiscovery builds a synthetic, DB-free request from
// the shared fixture; the adapter itself validates the typed command.
func episodicShadowRequestFromDiscovery(t *testing.T) EpisodicShadowProposalRequest {
	t.Helper()
	fixture := loadEpisodicShadowWireFixture(t)
	var command episodicShadowWireCommand
	if err := json.Unmarshal(fixture.Command, &command); err != nil {
		t.Fatal(err)
	}
	return episodicShadowRequestFromWire(t, command)
}
