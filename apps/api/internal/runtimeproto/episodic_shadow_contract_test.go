package runtimeproto_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	protovalidate "buf.build/go/protovalidate"
	runtimev1 "github.com/bodysense/api/internal/generated/runtimeproto/v1"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
)

type preferenceFixture struct {
	PolicyRevision string `json:"policy_revision"`
	Accepted       []struct {
		Quote      string `json:"quote"`
		Preference string `json:"preference"`
		Summary    string `json:"summary"`
	} `json:"accepted"`
	Rejected []string `json:"rejected"`
}

func TestEpisodicShadowProtoPolicyAndCorpus(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve source path")
	}
	name := filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "contracts", "internal", "agent-runtime", "fixtures", "episodic-shadow-s2b.v1.json")
	body, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	var corpus preferenceFixture
	if err := json.Unmarshal(body, &corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.PolicyRevision != "episodic-shadow-quote-v1" || len(corpus.Accepted) != 4 {
		t.Fatalf("policy authority mismatch %s (%d)", corpus.PolicyRevision, len(corpus.Accepted))
	}
	for _, accepted := range corpus.Accepted {
		p, ok := repository.CanonicalEpisodicPreference(accepted.Quote)
		if !ok || p.Kind != accepted.Preference || p.Text != accepted.Summary {
			t.Fatalf("Python/Go source preference parity mismatch: %q", accepted.Quote)
		}
	}
	for _, rejected := range corpus.Rejected {
		if _, ok := repository.CanonicalEpisodicPreference(rejected); ok {
			t.Fatalf("unsafe source quote authorized: %q", rejected)
		}
	}
	owner := uuid.New()
	conv := uuid.New()
	for i := 0; i < 120; i++ {
		source := uuid.New()
		reply := uuid.New()
		a := corpus.Accepted[i%len(corpus.Accepted)]
		cmd := &runtimev1.EpisodicShadowGenerationCommand{
			UserId: owner.String(), ConversationId: conv.String(),
			FromSeq: 1, ToSeq: 2, PolicyRevision: corpus.PolicyRevision,
			SourceMessages: []*runtimev1.EpisodicShadowSourceMessage{
				{Id: source.String(), Seq: 1, Role: "user", Text: a.Quote},
				{Id: reply.String(), Seq: 2, Role: "assistant", Text: "收到。"},
			},
		}
		proposal := &runtimev1.EpisodicShadowDraftProposal{
			PolicyRevision: corpus.PolicyRevision,
			Quotes: []*runtimev1.EpisodicShadowQuote{{
				SourceMessageId: source.String(), Quote: a.Quote, Preference: a.Preference,
			}},
		}
		if err := protovalidate.Validate(cmd); err != nil {
			t.Fatalf("command %d: %v", i, err)
		}
		if err := protovalidate.Validate(proposal); err != nil {
			t.Fatalf("response %d: %v", i, err)
		}
	}
	invalids := []*runtimev1.EpisodicShadowDraftProposal{
		{},
		{PolicyRevision: "other"},
		{PolicyRevision: corpus.PolicyRevision, Quotes: []*runtimev1.EpisodicShadowQuote{{SourceMessageId: uuid.New().String(), Quote: "invented clinical conclusion", Preference: "unknown"}}},
		{PolicyRevision: corpus.PolicyRevision, Quotes: []*runtimev1.EpisodicShadowQuote{{SourceMessageId: uuid.New().String(), Quote: "请简短回答。", Preference: "brief_answers"}, {SourceMessageId: uuid.New().String(), Quote: "请分点回答。", Preference: "bullet_answers"}}},
	}
	for i, bad := range invalids {
		if err := protovalidate.Validate(bad); err == nil {
			t.Fatalf("malformed proposal %d accepted", i)
		}
	}
}
