package consultation

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// The fixture's own user message is "A synthetic ambiguous source"; the
// suggestion cites runes [2,28) = "synthetic ambiguous source".
const (
	suggestionSpanStart = 2
	suggestionSpanEnd   = 28
	suggestionStatement = "synthetic ambiguous source"
)

func runtimeSuggestion(messageID uuid.UUID, start, end int, revision int64) json.RawMessage {
	raw, _ := json.Marshal(map[string]any{
		"purpose": model.BodyStateSuggestionPurpose, "question": "要我帮你记下这个猜测吗？", "answer_type": "single_choice",
		"options": []string{"好", "不要"},
		"suggestion": map[string]any{"kind": "hypothesis", "cited_message_id": messageID.String(),
			"span": map[string]any{"start": start, "end": end}, "base_body_state_revision": revision},
	})
	return raw
}

// bindSuggestion runs the real Go binder against PostgreSQL, as the runtime
// does when it persists the interrupt.
func bindSuggestion(t *testing.T, f resumeFixture, raw json.RawMessage) (json.RawMessage, bool) {
	t.Helper()
	bound, ok, err := service.BindBodyStateSuggestionQuestion(context.Background(), repository.NewBodyStateRepository(f.db), f.owner, f.interaction.ConversationID, raw)
	if err != nil {
		t.Fatal(err)
	}
	return bound, ok
}

func boundSuggestionResumeFixture(t *testing.T) resumeFixture {
	t.Helper()
	f := newResumeFixture(t)
	bound, ok := bindSuggestion(t, f, runtimeSuggestion(f.envelope.UserMessage.ID, suggestionSpanStart, suggestionSpanEnd, 0))
	if !ok {
		t.Fatalf("suggestion citing the owner's own message was not bound: %s", bound)
	}
	bindIntakeQuestion(t, f, "ask-suggestion-1", bound)
	return f
}

func suggestionCommand(f resumeFixture, option string) model.ClaimInteractionResume {
	answer, _ := json.Marshal(map[string]any{"selected": []string{option}})
	return model.ClaimInteractionResume{UserID: f.owner, ConversationID: f.interaction.ConversationID, InteractionID: f.interaction.ID, RequestID: uuid.NewString(), Answer: datatypes.JSON(answer)}
}

func suggestionBodyState(f resumeFixture) *service.BodyStateService {
	return service.NewBodyStateService(repository.NewBodyStateRepository(f.db))
}

func answeredReceipt(t *testing.T, result *service.InteractionResumeResult) (map[string]any, bool) {
	t.Helper()
	if result == nil || len(result.Events) == 0 || result.Events[0].Type != "state.interaction.answered" {
		t.Fatalf("missing answered milestone: %#v", result)
	}
	var payload map[string]any
	if err := json.Unmarshal(result.Events[0].Payload, &payload); err != nil {
		t.Fatal(err)
	}
	receipt, present := payload["body_state_receipt"]
	if !present {
		return nil, false
	}
	asMap, _ := receipt.(map[string]any)
	return asMap, true
}

func assertSuggestionClaimLeftNothing(t *testing.T, f resumeFixture, err error, priorEvidence ...int64) {
	t.Helper()
	if err == nil {
		t.Fatal("invalid suggestion confirmation must fail the claim")
	}
	var evidence int64
	if len(priorEvidence) == 1 {
		evidence = priorEvidence[0]
	}
	resumeCount(t, f, "body_hypotheses", 0)
	resumeCount(t, f, "statement_evidence", evidence)
	resumeCount(t, f, "interaction_resumes", 0)
	var interaction model.AgentInteraction
	if err := f.db.First(&interaction, "id = ?", f.interaction.ID).Error; err != nil || interaction.Status != model.AgentInteractionPending {
		t.Fatalf("failed confirmation closed the interaction: %v %v", interaction.Status, err)
	}
}

func TestSuggestionAcceptCommitsExactlyOneUserHypothesisWithReceiptPostgres(t *testing.T) {
	f := boundSuggestionResumeFixture(t)
	resumes := f.resumes.WithBodyStateSuggestions(suggestionBodyState(f))
	command := suggestionCommand(f, model.BodyStateSuggestionConfirmOption)
	claimed, err := resumes.Claim(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	resumeCount(t, f, "body_hypotheses", 1)
	resumeCount(t, f, "body_hypothesis_commits", 1)
	resumeCount(t, f, "body_state_revisions", 1)
	var hypothesis model.BodyHypothesis
	if err := f.db.Where("user_id = ?", f.owner).Take(&hypothesis).Error; err != nil {
		t.Fatal(err)
	}
	if hypothesis.Statement != suggestionStatement || hypothesis.AssertionStatus != "asserted" {
		t.Fatalf("hypothesis is not the user's verbatim words: %+v", hypothesis)
	}
	var evidence model.StatementEvidence
	if err := f.db.Where("user_id = ? AND id = ?", f.owner, hypothesis.SourceEvidenceID).Take(&evidence).Error; err != nil {
		t.Fatal(err)
	}
	if evidence.SourceKind != "user_message" || evidence.Excerpt != suggestionStatement || evidence.SourceMessageID == nil ||
		*evidence.SourceMessageID != f.envelope.UserMessage.ID || evidence.SourceRunID == nil || *evidence.SourceRunID != f.envelope.Run.ID {
		t.Fatalf("evidence is not bound to the cited user message: %+v", evidence)
	}
	receipt, present := answeredReceipt(t, claimed)
	if !present || receipt == nil || receipt["hypothesis_id"] != hypothesis.ID.String() || receipt["kind"] != "hypothesis" || receipt["revision"] != float64(1) {
		t.Fatalf("answered milestone lacks the commit receipt: %#v", receipt)
	}

	// Replay (same request, then same answer from another device) returns the
	// original result and writes nothing more.
	for i := 0; i < 2; i++ {
		retry, err := resumes.Claim(context.Background(), command)
		if err != nil || !retry.Envelope.Existed {
			t.Fatalf("replay was not idempotent: %v", err)
		}
		command.RequestID = uuid.NewString()
	}
	resumeCount(t, f, "body_hypotheses", 1)
	resumeCount(t, f, "body_state_revisions", 1)

	// A direct second confirmation of the same interaction returns the same receipt.
	source := model.AnswerFactSource{RunID: f.envelope.Run.ID, InteractionID: f.interaction.ID, MessageID: claimed.Envelope.UserMessage.ID}
	parsed, _, _ := model.ParseBoundBodyStateSuggestion(f.interaction.Question)
	again, err := repository.NewBodyStateRepository(f.db).ConfirmBodyHypothesisSuggestion(context.Background(), f.owner, source, parsed)
	if err != nil || again.HypothesisID != hypothesis.ID {
		t.Fatalf("receipt replay changed: %+v %v", again, err)
	}
	resumeCount(t, f, "body_hypotheses", 1)
}

func TestSuggestionRejectWritesNothingPostgres(t *testing.T) {
	f := boundSuggestionResumeFixture(t)
	claimed, err := f.resumes.WithBodyStateSuggestions(suggestionBodyState(f)).Claim(context.Background(), suggestionCommand(f, model.BodyStateSuggestionRejectOption))
	if err != nil {
		t.Fatal(err)
	}
	if receipt, present := answeredReceipt(t, claimed); !present || receipt != nil {
		t.Fatalf("a decline must carry a null receipt: %#v %v", receipt, present)
	}
	resumeCount(t, f, "body_hypotheses", 0)
	resumeCount(t, f, "statement_evidence", 0)
	resumeCount(t, f, "body_state_revisions", 0)
	resumeCount(t, f, "interaction_resumes", 1)
}

func TestSuggestionBinderRejectsUnboundSourcesPostgres(t *testing.T) {
	f := newResumeFixture(t)
	foreign := newResumeFixture(t)
	cases := map[string]json.RawMessage{
		"foreign message":            runtimeSuggestion(foreign.envelope.UserMessage.ID, suggestionSpanStart, suggestionSpanEnd, 0),
		"assistant message":          runtimeSuggestion(f.envelope.AssistantMessage.ID, 0, 3, 0),
		"span text mismatch":         runtimeSuggestion(f.envelope.UserMessage.ID, suggestionSpanStart, suggestionSpanEnd+5, 0),
		"stale revision":             runtimeSuggestion(f.envelope.UserMessage.ID, suggestionSpanStart, suggestionSpanEnd, 3),
		"whitespace boundary span":   runtimeSuggestion(f.envelope.UserMessage.ID, 1, suggestionSpanEnd, 0),
		"missing message identifier": runtimeSuggestion(uuid.New(), suggestionSpanStart, suggestionSpanEnd, 0),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if out, ok := bindSuggestion(t, f, raw); ok || service.IsBoundBodyStateSuggestion(out) {
				t.Fatalf("unbindable suggestion was bound: %s", out)
			}
		})
	}
	// The foreign owner's own suggestion still binds for that owner only.
	if _, ok := bindSuggestion(t, foreign, runtimeSuggestion(foreign.envelope.UserMessage.ID, suggestionSpanStart, suggestionSpanEnd, 0)); !ok {
		t.Fatal("owner's own citation was not bound")
	}
}

func TestSuggestionConfirmationRejectedAtCommitTimePostgres(t *testing.T) {
	t.Run("stale revision", func(t *testing.T) {
		f := boundSuggestionResumeFixture(t)
		if _, err := repository.NewBodyStateRepository(f.db).RecordDirectFacts(context.Background(), f.owner, model.RecordBodyFactsCommand{
			IdempotencyKey: uuid.NewString(), ExpectedRevision: 0,
			Facts: []model.BodyFactInput{{Kind: "symptom", Statement: "Another statement", BodyRegionIDs: []string{}, Temporal: model.FactTemporal{Precision: "unknown"}, Details: []byte(`{"presence":"present"}`)}},
		}); err != nil {
			t.Fatal(err)
		}
		resumes := f.resumes.WithBodyStateSuggestions(suggestionBodyState(f))
		_, err := resumes.Claim(context.Background(), suggestionCommand(f, model.BodyStateSuggestionConfirmOption))
		if !errors.Is(err, model.ErrBodyStateSuggestionStale) {
			t.Fatalf("stale confirmation not refused: %v", err)
		}
		assertSuggestionClaimLeftNothing(t, f, err, 1) // only the direct fact's own evidence
		// The user can still decline; that resumes and writes nothing.
		if _, err := resumes.Claim(context.Background(), suggestionCommand(f, model.BodyStateSuggestionRejectOption)); err != nil {
			t.Fatal(err)
		}
		resumeCount(t, f, "body_hypotheses", 0)
	})
	t.Run("withdrawn source", func(t *testing.T) {
		f := boundSuggestionResumeFixture(t)
		// Retention pruning moves the AI-visible cursor past the cited message.
		if err := f.db.Model(&model.Conversation{}).Where("id = ?", f.interaction.ConversationID).Updates(map[string]any{
			"processed_through_seq": f.envelope.UserMessage.Seq, "pruned_through_seq": f.envelope.UserMessage.Seq, "context_start_seq": f.envelope.UserMessage.Seq + 1}).Error; err != nil {
			t.Fatal(err)
		}
		_, err := f.resumes.WithBodyStateSuggestions(suggestionBodyState(f)).Claim(context.Background(), suggestionCommand(f, model.BodyStateSuggestionConfirmOption))
		if !errors.Is(err, model.ErrBodyStateSuggestionRejected) {
			t.Fatalf("withdrawn source not refused: %v", err)
		}
		assertSuggestionClaimLeftNothing(t, f, err)
	})
	t.Run("tampered quote", func(t *testing.T) {
		f := boundSuggestionResumeFixture(t)
		var q map[string]any
		_ = json.Unmarshal(f.interaction.Question, &q)
		q["quote"] = "synthetic certain diagnosis"
		forged, _ := json.Marshal(q)
		bindIntakeQuestion(t, f, f.interaction.ToolCallID, forged)
		_, err := f.resumes.WithBodyStateSuggestions(suggestionBodyState(f)).Claim(context.Background(), suggestionCommand(f, model.BodyStateSuggestionConfirmOption))
		assertSuggestionClaimLeftNothing(t, f, err)
	})
	t.Run("foreign claimant", func(t *testing.T) {
		f := boundSuggestionResumeFixture(t)
		command := suggestionCommand(f, model.BodyStateSuggestionConfirmOption)
		command.UserID = uuid.New()
		_, err := f.resumes.WithBodyStateSuggestions(suggestionBodyState(f)).Claim(context.Background(), command)
		assertSuggestionClaimLeftNothing(t, f, err)
	})
	t.Run("accept after cancel", func(t *testing.T) {
		f := boundSuggestionResumeFixture(t)
		if err := f.db.Model(f.envelope.Run).Update("status", model.RunStatusCancelled).Error; err != nil {
			t.Fatal(err)
		}
		_, err := f.resumes.WithBodyStateSuggestions(suggestionBodyState(f)).Claim(context.Background(), suggestionCommand(f, model.BodyStateSuggestionConfirmOption))
		assertSuggestionClaimLeftNothing(t, f, err)
	})
	t.Run("missing recorder", func(t *testing.T) {
		f := boundSuggestionResumeFixture(t)
		_, err := f.resumes.Claim(context.Background(), suggestionCommand(f, model.BodyStateSuggestionConfirmOption))
		if !errors.Is(err, model.ErrInteractionResumeUnavailable) {
			t.Fatalf("bound suggestion without a recorder must be refused, got %v", err)
		}
		assertSuggestionClaimLeftNothing(t, f, err)
	})
	t.Run("free text cannot confirm", func(t *testing.T) {
		f := boundSuggestionResumeFixture(t)
		command := suggestionCommand(f, model.BodyStateSuggestionConfirmOption)
		command.Answer = datatypes.JSON(`{"text":"记录为我的猜测"}`)
		_, err := f.resumes.WithBodyStateSuggestions(suggestionBodyState(f)).Claim(context.Background(), command)
		assertSuggestionClaimLeftNothing(t, f, err)
	})
}
