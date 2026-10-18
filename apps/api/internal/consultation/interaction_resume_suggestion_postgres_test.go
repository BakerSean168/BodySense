package consultation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
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
	return runtimeKindSuggestion(messageID, "hypothesis", start, end, revision, nil)
}

// runtimeKindSuggestion is the question the runtime handler hands the binder
// after merging the typed InteractionRequired.suggestion field.
func runtimeKindSuggestion(messageID uuid.UUID, kind string, start, end int, revision int64, payload map[string]any) json.RawMessage {
	typed := map[string]any{"kind": kind, "cited_message_id": messageID.String(),
		"span": map[string]any{"start": start, "end": end}, "base_body_state_revision": revision}
	if payload != nil {
		typed["payload"] = payload
	}
	encoded, _ := json.Marshal(typed)
	raw, err := service.AttachRuntimeBodyStateSuggestion(json.RawMessage(`{"question":"要我帮你记下来吗？","answer_type":"single_choice","options":["好","不要"]}`), encoded)
	if err != nil {
		panic(err)
	}
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
	// The answered interaction keeps the same receipt for chat history.
	var stored model.AgentInteraction
	if err := f.db.First(&stored, "id = ?", f.interaction.ID).Error; err != nil {
		t.Fatal(err)
	}
	var metadata map[string]any
	if err := json.Unmarshal(stored.Metadata, &metadata); err != nil {
		t.Fatal(err)
	}
	if history, _ := metadata["body_state_receipt"].(map[string]any); history["commit_id"] != receipt["commit_id"] || metadata["runtime_interrupt_id"] == nil {
		t.Fatalf("interaction history lacks the receipt: %#v", metadata)
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
	again, err := repository.NewBodyStateRepository(f.db).ConfirmBodyStateSuggestion(context.Background(), f.owner, source, parsed)
	if err != nil || again.RecordID != hypothesis.ID || again.Kind != model.BodyStateSuggestionKindHypothesis {
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
	var stored model.AgentInteraction
	if err := f.db.First(&stored, "id = ?", f.interaction.ID).Error; err != nil || strings.Contains(string(stored.Metadata), "body_state_receipt") {
		t.Fatalf("a decline left a receipt: %s %v", stored.Metadata, err)
	}
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

func boundKindSuggestionResumeFixture(t *testing.T, f resumeFixture, kind string, revision int64, payload map[string]any) model.ConfirmBodyStateSuggestionCommand {
	t.Helper()
	bound, ok := bindSuggestion(t, f, runtimeKindSuggestion(f.envelope.UserMessage.ID, kind, suggestionSpanStart, suggestionSpanEnd, revision, payload))
	if !ok {
		t.Fatalf("%s suggestion citing the owner's own message was not bound: %s", kind, bound)
	}
	bindIntakeQuestion(t, f, "ask-suggestion-"+kind, bound)
	command, matched, err := model.ParseBoundBodyStateSuggestion(bound)
	if err != nil || !matched {
		t.Fatalf("bound %s question did not parse: %v", kind, err)
	}
	return command
}

func TestSuggestionObservationAcceptCommitsUserReportedObservationPostgres(t *testing.T) {
	f := newResumeFixture(t)
	boundKindSuggestionResumeFixture(t, f, "observation", 0, map[string]any{"observation_kind": "synthetic_note", "raw_value": "ambiguous"})
	resumes := f.resumes.WithBodyStateSuggestions(suggestionBodyState(f))
	claimed, err := resumes.Claim(context.Background(), suggestionCommand(f, model.BodyStateSuggestionConfirmOptionFor("observation")))
	if err != nil {
		t.Fatal(err)
	}
	resumeCount(t, f, "body_observations", 1)
	resumeCount(t, f, "body_observation_commits", 1)
	resumeCount(t, f, "body_state_revisions", 1)
	page, err := repository.NewBodyStateRepository(f.db).ListDirectObservations(context.Background(), f.owner, 0)
	if err != nil || len(page.Observations) != 1 {
		t.Fatalf("confirmed observation is not readable: %+v %v", page, err)
	}
	observation := page.Observations[0]
	if observation.Statement != suggestionStatement || observation.Kind != "synthetic_note" || observation.RawValue != "ambiguous" ||
		observation.RawUnit != "" || observation.ObservedAt.Precision != "unknown" {
		t.Fatalf("observation is not the user's verbatim words: %+v", observation)
	}
	var evidence model.StatementEvidence
	if err := f.db.Where("user_id = ? AND id = ?", f.owner, observation.Source.ID).Take(&evidence).Error; err != nil {
		t.Fatal(err)
	}
	if evidence.SourceKind != "user_message" || evidence.SourceMessageID == nil || *evidence.SourceMessageID != f.envelope.UserMessage.ID {
		t.Fatalf("observation evidence is not bound to the cited message: %+v", evidence)
	}
	receipt, present := answeredReceipt(t, claimed)
	if !present || receipt["kind"] != "observation" || receipt["observation_id"] != observation.ID.String() || receipt["source_evidence_id"] != evidence.ID.String() {
		t.Fatalf("answered milestone lacks the observation receipt: %#v", receipt)
	}
	// The user can withdraw a confirmed chat observation like a direct one.
	if _, err := repository.NewBodyStateRepository(f.db).WithdrawDirectObservation(context.Background(), f.owner, observation.ID,
		model.AmendBodyObservationCommand{IdempotencyKey: uuid.NewString(), ExpectedRevision: 1}); err != nil {
		t.Fatalf("confirmed chat observation cannot be withdrawn: %v", err)
	}
}

func TestSuggestionObservationRejectWritesNothingPostgres(t *testing.T) {
	f := newResumeFixture(t)
	boundKindSuggestionResumeFixture(t, f, "observation", 0, map[string]any{"observation_kind": "synthetic_note"})
	resumes := f.resumes.WithBodyStateSuggestions(suggestionBodyState(f))
	// Another kind's confirming option is not a confirmation of this one.
	if _, err := resumes.Claim(context.Background(), suggestionCommand(f, model.BodyStateSuggestionConfirmOption)); err == nil {
		t.Fatal("a hypothesis option confirmed an observation")
	}
	if _, err := resumes.Claim(context.Background(), suggestionCommand(f, model.BodyStateSuggestionRejectOption)); err != nil {
		t.Fatal(err)
	}
	resumeCount(t, f, "body_observations", 0)
	resumeCount(t, f, "statement_evidence", 0)
	resumeCount(t, f, "body_state_revisions", 0)
}

func recordSuggestionSymptomFact(t *testing.T, f resumeFixture) uuid.UUID {
	t.Helper()
	// The runtime commits a turn's facts while its Run streams, before the
	// suggestion interrupt pauses it.
	setRunStatus := func(status model.RunStatus) {
		if err := f.db.Model(f.envelope.Run).Update("status", status).Error; err != nil {
			t.Fatal(err)
		}
	}
	setRunStatus(model.RunStatusRunning)
	defer setRunStatus(model.RunStatusWaitingUser)
	commit, err := repository.NewBodyStateRepository(f.db).RecordRunFacts(context.Background(), f.owner, f.envelope.Run.ID, model.RecordBodyFactsCommand{
		IdempotencyKey: uuid.NewString(), ExpectedRevision: 0,
		Facts: []model.BodyFactInput{{Kind: "symptom", Statement: suggestionStatement, BodyRegionIDs: []string{},
			Temporal: model.FactTemporal{Precision: "unknown"}, Details: []byte(`{"presence":"present"}`)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var ids []uuid.UUID
	if err := json.Unmarshal(commit.FactIDs, &ids); err != nil || len(ids) != 1 {
		t.Fatalf("fact receipt: %v %v", ids, err)
	}
	return ids[0]
}

func TestSuggestionEpisodeAcceptOrganizesTheCitedSymptomFactPostgres(t *testing.T) {
	f := newResumeFixture(t)
	factID := recordSuggestionSymptomFact(t, f)
	command := boundKindSuggestionResumeFixture(t, f, "episode", 1, map[string]any{"symptom_type": "ambiguous"})
	if command.Payload.FactID != factID.String() {
		t.Fatalf("episode bound to %s, want the Go-resolved fact %s", command.Payload.FactID, factID)
	}
	claimed, err := f.resumes.WithBodyStateSuggestions(suggestionBodyState(f)).Claim(context.Background(), suggestionCommand(f, model.BodyStateSuggestionConfirmOptionFor("episode")))
	if err != nil {
		t.Fatal(err)
	}
	resumeCount(t, f, "symptom_episodes", 1)
	resumeCount(t, f, "episode_fact_links", 1)
	resumeCount(t, f, "body_state_revisions", 2)
	var episode model.SymptomEpisode
	if err := f.db.Where("user_id = ?", f.owner).Take(&episode).Error; err != nil {
		t.Fatal(err)
	}
	if episode.SymptomType != "ambiguous" || episode.Laterality != "unknown" {
		t.Fatalf("episode is not organized from the user's own words: %+v", episode)
	}
	receipt, present := answeredReceipt(t, claimed)
	if !present || receipt["kind"] != "episode" || receipt["episode_id"] != episode.ID.String() || receipt["fact_id"] != factID.String() {
		t.Fatalf("answered milestone lacks the episode receipt: %#v", receipt)
	}
	// The organized fact cannot be suggested as a new episode again.
	if out, ok := bindSuggestion(t, f, runtimeKindSuggestion(f.envelope.UserMessage.ID, "episode", suggestionSpanStart, suggestionSpanEnd, 2, map[string]any{"symptom_type": "ambiguous"})); ok {
		t.Fatalf("an organized fact was suggested again: %s", out)
	}
}

func TestSuggestionEpisodeBinderAndCommitRejectUnboundFactsPostgres(t *testing.T) {
	t.Run("no fact from the cited words", func(t *testing.T) {
		f := newResumeFixture(t)
		if out, ok := bindSuggestion(t, f, runtimeKindSuggestion(f.envelope.UserMessage.ID, "episode", suggestionSpanStart, suggestionSpanEnd, 0, map[string]any{"symptom_type": "ambiguous"})); ok ||
			string(out) != model.BodyStateSuggestionUnverifiedQuestion {
			t.Fatalf("episode without a committed fact was bound: %s", out)
		}
	})
	t.Run("runtime-named fact", func(t *testing.T) {
		f := newResumeFixture(t)
		factID := recordSuggestionSymptomFact(t, f)
		if _, ok := bindSuggestion(t, f, runtimeKindSuggestion(f.envelope.UserMessage.ID, "episode", suggestionSpanStart, suggestionSpanEnd, 1, map[string]any{"symptom_type": "ambiguous", "fact_id": factID.String()})); ok {
			t.Fatal("a runtime-named fact was bound")
		}
	})
	t.Run("fact withdrawn before confirmation", func(t *testing.T) {
		f := newResumeFixture(t)
		factID := recordSuggestionSymptomFact(t, f)
		boundKindSuggestionResumeFixture(t, f, "episode", 1, map[string]any{"symptom_type": "ambiguous"})
		if err := f.db.Exec("UPDATE body_facts SET assertion_status = 'withdrawn' WHERE user_id = ? AND id = ?", f.owner, factID).Error; err != nil {
			t.Fatal(err)
		}
		_, err := f.resumes.WithBodyStateSuggestions(suggestionBodyState(f)).Claim(context.Background(), suggestionCommand(f, model.BodyStateSuggestionConfirmOptionFor("episode")))
		if !errors.Is(err, model.ErrBodyStateSuggestionRejected) {
			t.Fatalf("withdrawn fact not refused: %v", err)
		}
		resumeCount(t, f, "symptom_episodes", 0)
		resumeCount(t, f, "interaction_resumes", 0)
	})
}
