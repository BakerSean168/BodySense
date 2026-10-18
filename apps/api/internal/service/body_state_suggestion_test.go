package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type fakeSuggestionSourceReader struct {
	source   *model.SuggestionSourceMessage
	revision int64
	err      error
	calls    int
	factID   uuid.UUID
	excerpts []string
}

func (f *fakeSuggestionSourceReader) ReadSuggestionEpisodeFact(_ context.Context, _ uuid.UUID, source model.SuggestionSourceMessage, excerpt string) (uuid.UUID, error) {
	f.excerpts = append(f.excerpts, excerpt)
	if f.factID == uuid.Nil || source.ID != f.source.ID {
		return uuid.Nil, model.ErrBodyStateSuggestionRejected
	}
	return f.factID, nil
}

func (f *fakeSuggestionSourceReader) ReadSuggestionSource(_ context.Context, _, conversationID, messageID uuid.UUID) (*model.SuggestionSourceMessage, int64, error) {
	f.calls++
	if f.err != nil {
		return nil, 0, f.err
	}
	if f.source == nil || f.source.ID != messageID || f.source.ConversationID != conversationID {
		return nil, 0, model.ErrBodyStateSuggestionRejected
	}
	return f.source, f.revision, nil
}

func suggestionQuestion(t *testing.T, messageID uuid.UUID, start, end int, revision int64, extra map[string]any) json.RawMessage {
	t.Helper()
	q := map[string]any{
		"purpose": model.BodyStateSuggestionPurpose, "question": "我帮你把这个记下来？", "answer_type": "single_choice",
		"options": []string{"好", "不要"},
		"suggestion": map[string]any{"kind": "hypothesis", "cited_message_id": messageID.String(),
			"span": map[string]any{"start": start, "end": end}, "base_body_state_revision": revision},
	}
	for k, v := range extra {
		q[k] = v
	}
	raw, err := json.Marshal(q)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func suggestionFixture() (uuid.UUID, uuid.UUID, *fakeSuggestionSourceReader) {
	owner, conversation := uuid.New(), uuid.New()
	text := "我觉得是跑步太多导致膝盖疼。 "
	return owner, conversation, &fakeSuggestionSourceReader{
		source:   &model.SuggestionSourceMessage{ID: uuid.New(), ConversationID: conversation, RunID: uuid.New(), Seq: 3, Text: text},
		revision: 7,
	}
}

func TestBindBodyStateSuggestionWritesCanonicalGoBinding(t *testing.T) {
	owner, conversation, reader := suggestionFixture()
	// "跑步太多导致膝盖疼" = runes [4,13)
	raw := suggestionQuestion(t, reader.source.ID, 4, 13, 7, map[string]any{"state_binding": map[string]any{"revision": "forged"}, "quote": "模型改写"})
	bound, ok, err := BindBodyStateSuggestionQuestion(context.Background(), reader, owner, conversation, raw)
	if err != nil || !ok {
		t.Fatalf("bind = %v %v", ok, err)
	}
	var q model.BodyStateSuggestionQuestion
	if err := json.Unmarshal(bound, &q); err != nil {
		t.Fatal(err)
	}
	if q.Quote != "跑步太多导致膝盖疼" || q.Question != model.BodyStateSuggestionQuestionText || len(q.Options) != 2 ||
		q.StateBinding.Source.MessageID != reader.source.ID || q.StateBinding.Source.MessageSeq != 3 || q.StateBinding.BaseBodyStateRevision != 7 ||
		q.StateBinding.Source.ExcerptSHA256 != model.BodyStateSuggestionDigest("跑步太多导致膝盖疼") {
		t.Fatalf("binding is not canonical: %s", bound)
	}
	if strings.Contains(string(bound), "模型改写") || strings.Contains(string(bound), "forged") || strings.Contains(string(bound), "我帮你") {
		t.Fatalf("model-authored text survived binding: %s", bound)
	}
	command, matched, err := model.ParseBoundBodyStateSuggestion(bound)
	if err != nil || !matched || command.Source != q.StateBinding.Source || command.BaseBodyStateRevision != 7 {
		t.Fatalf("parse bound = %+v %v %v", command, matched, err)
	}
	if !IsBoundBodyStateSuggestion(bound) {
		t.Fatal("bound question not recognised")
	}
	if _, _, err := model.ValidateInteractionAnswer(bound, []byte(`{"selected":["记录为我的猜测"]}`)); err != nil {
		t.Fatalf("canonical confirmation answer rejected: %v", err)
	}
	if _, _, err := model.ValidateInteractionAnswer(bound, []byte(`{"selected":["随便"],"is_custom":true}`)); err == nil {
		t.Fatal("custom answer accepted for a bound suggestion")
	}
}

func TestBindBodyStateSuggestionStripsUnbindableSuggestions(t *testing.T) {
	owner, conversation, reader := suggestionFixture()
	id := reader.source.ID
	cases := map[string]struct {
		question     json.RawMessage
		owner        uuid.UUID
		conversation uuid.UUID
		reader       BodyStateSuggestionSourceReader
	}{
		"foreign or missing message": {suggestionQuestion(t, uuid.New(), 4, 13, 7, nil), owner, conversation, reader},
		"other conversation":         {suggestionQuestion(t, id, 4, 13, 7, nil), owner, uuid.New(), reader},
		"span out of range":          {suggestionQuestion(t, id, 4, 99, 7, nil), owner, conversation, reader},
		"empty span":                 {suggestionQuestion(t, id, 5, 5, 7, nil), owner, conversation, reader},
		"whitespace-padded span":     {suggestionQuestion(t, id, 13, 15, 7, nil), owner, conversation, reader},
		"stale revision":             {suggestionQuestion(t, id, 4, 13, 6, nil), owner, conversation, reader},
		"no reader":                  {suggestionQuestion(t, id, 4, 13, 7, nil), owner, conversation, nil},
		"anonymous owner":            {suggestionQuestion(t, id, 4, 13, 7, nil), uuid.Nil, conversation, reader},
		"unsupported kind":           {suggestionJSON(t, map[string]any{"purpose": model.BodyStateSuggestionPurpose, "question": "q", "suggestion": map[string]any{"kind": "diagnosis", "cited_message_id": id.String(), "span": map[string]any{"start": 4, "end": 13}, "base_body_state_revision": 7}}), owner, conversation, reader},
		"model excerpt smuggled":     {suggestionJSON(t, map[string]any{"purpose": model.BodyStateSuggestionPurpose, "question": "q", "suggestion": map[string]any{"kind": "hypothesis", "cited_message_id": id.String(), "span": map[string]any{"start": 4, "end": 13}, "base_body_state_revision": 7, "excerpt": "x"}}), owner, conversation, reader},
		"missing revision":           {suggestionJSON(t, map[string]any{"purpose": model.BodyStateSuggestionPurpose, "question": "q", "suggestion": map[string]any{"kind": "hypothesis", "cited_message_id": id.String(), "span": map[string]any{"start": 4, "end": 13}}}), owner, conversation, reader},
		"suggestion without purpose": {suggestionJSON(t, map[string]any{"question": "q", "suggestion": map[string]any{"kind": "hypothesis", "cited_message_id": id.String(), "span": map[string]any{"start": 4, "end": 13}, "base_body_state_revision": 7}}), owner, conversation, reader},
		"non-canonical uuid":         {suggestionJSON(t, map[string]any{"purpose": model.BodyStateSuggestionPurpose, "question": "q", "suggestion": map[string]any{"kind": "hypothesis", "cited_message_id": strings.ToUpper(id.String()), "span": map[string]any{"start": 4, "end": 13}, "base_body_state_revision": 7}}), owner, conversation, reader},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			out, ok, err := BindBodyStateSuggestionQuestion(context.Background(), tc.reader, tc.owner, tc.conversation, tc.question)
			if err != nil || ok {
				t.Fatalf("bind = %v %v", ok, err)
			}
			if IsBoundBodyStateSuggestion(out) || strings.Contains(string(out), `"suggestion"`) || strings.Contains(string(out), `"state_binding"`) {
				t.Fatalf("unbindable suggestion kept authority: %s", out)
			}
		})
	}
}

func TestBindBodyStateSuggestionPassesOrdinaryQuestionsAndFailsClosedOnStorageError(t *testing.T) {
	owner, conversation, reader := suggestionFixture()
	plain := json.RawMessage(`{"question":"哪里疼？","answer_type":"text"}`)
	if out, ok, err := BindBodyStateSuggestionQuestion(context.Background(), reader, owner, conversation, plain); err != nil || ok || string(out) != string(plain) || reader.calls != 0 {
		t.Fatalf("ordinary question changed: %s %v %v", out, ok, err)
	}
	reader.err = errors.New("db down")
	if _, _, err := BindBodyStateSuggestionQuestion(context.Background(), reader, owner, conversation, suggestionQuestion(t, reader.source.ID, 4, 13, 7, nil)); err == nil {
		t.Fatal("storage failure did not fail closed")
	}
	if _, _, err := BindBodyStateSuggestionQuestion(context.Background(), reader, owner, conversation, json.RawMessage(`[]`)); err == nil {
		t.Fatal("non-object question accepted")
	}
}

func suggestionJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

type suggestionLedgerRepository struct {
	fakeBodyStateRepository
	commands []model.ConfirmBodyStateSuggestionCommand
	receipt  func(owner uuid.UUID) *model.BodyStateSuggestionReceipt
}

func (r *suggestionLedgerRepository) ConfirmBodyStateSuggestion(_ context.Context, owner uuid.UUID, _ model.AnswerFactSource, command model.ConfirmBodyStateSuggestionCommand) (*model.BodyStateSuggestionReceipt, error) {
	r.commands = append(r.commands, command)
	if r.receipt != nil {
		return r.receipt(owner), nil
	}
	receipt := &model.BodyStateSuggestionReceipt{Kind: command.Kind, CommitID: uuid.New(), UserID: owner, RecordID: uuid.New(),
		EventID: uuid.New(), RevisionID: uuid.New(), Revision: 8}
	if command.Kind == model.BodyStateSuggestionKindEpisode {
		receipt.FactID = uuid.MustParse(command.Payload.FactID)
	} else {
		receipt.SourceEvidenceID = uuid.New()
	}
	return receipt, nil
}

func boundSuggestionInteraction(t *testing.T, answer string) *model.AgentInteraction {
	t.Helper()
	owner, conversation, reader := suggestionFixture()
	bound, ok, err := BindBodyStateSuggestionQuestion(context.Background(), reader, owner, conversation, suggestionQuestion(t, reader.source.ID, 4, 13, 7, nil))
	if err != nil || !ok {
		t.Fatalf("bind = %v %v", ok, err)
	}
	return &model.AgentInteraction{ID: uuid.New(), ToolName: "ask_user", ToolCallID: "call_s", Question: []byte(bound), Answer: []byte(answer)}
}

func TestRecordBodyStateSuggestionAnswerCommitsOnlyConfirmation(t *testing.T) {
	owner := uuid.New()
	source := model.AnswerFactSource{RunID: uuid.New(), InteractionID: uuid.New(), MessageID: uuid.New()}
	repo := &suggestionLedgerRepository{}
	svc := NewBodyStateService(repo)

	confirm := boundSuggestionInteraction(t, `{"text":"记录为我的猜测","selected":["记录为我的猜测"]}`)
	receipt, err := svc.RecordBodyStateSuggestionAnswer(context.Background(), owner, source, confirm)
	if err != nil || receipt == nil || len(repo.commands) != 1 || repo.commands[0].BaseBodyStateRevision != 7 || repo.commands[0].Source.SpanEnd != 13 {
		t.Fatalf("confirm = %+v %v %+v", receipt, err, repo.commands)
	}
	decline := boundSuggestionInteraction(t, `{"text":"不记录","selected":["不记录"]}`)
	if receipt, err := svc.RecordBodyStateSuggestionAnswer(context.Background(), owner, source, decline); err != nil || receipt != nil || len(repo.commands) != 1 {
		t.Fatalf("decline wrote: %+v %v", receipt, err)
	}
	plain := &model.AgentInteraction{ID: uuid.New(), Question: []byte(`{"question":"哪里疼？","answer_type":"text"}`), Answer: []byte(`{"text":"膝盖"}`)}
	if receipt, err := svc.RecordBodyStateSuggestionAnswer(context.Background(), owner, source, plain); err != nil || receipt != nil || len(repo.commands) != 1 {
		t.Fatalf("ordinary answer wrote: %+v %v", receipt, err)
	}
	forged := &model.AgentInteraction{ID: uuid.New(), Question: []byte(`{"purpose":"body_state_suggestion","question":"q","answer_type":"single_choice","options":["记录为我的猜测","不记录"]}`), Answer: []byte(`{"selected":["记录为我的猜测"]}`)}
	if _, err := svc.RecordBodyStateSuggestionAnswer(context.Background(), owner, source, forged); !errors.Is(err, model.ErrBodyStateSuggestionRejected) || len(repo.commands) != 1 {
		t.Fatalf("forged binding not refused: %v", err)
	}
	repo.receipt = func(uuid.UUID) *model.BodyStateSuggestionReceipt {
		return &model.BodyStateSuggestionReceipt{Kind: model.BodyStateSuggestionKindHypothesis, CommitID: uuid.New(), UserID: uuid.New(), RecordID: uuid.New(),
			SourceEvidenceID: uuid.New(), EventID: uuid.New(), RevisionID: uuid.New(), Revision: 9}
	}
	if _, err := svc.RecordBodyStateSuggestionAnswer(context.Background(), owner, source, confirm); err == nil {
		t.Fatal("foreign receipt accepted")
	}
	if _, err := NewBodyStateService(&fakeBodyStateRepository{}).RecordBodyStateSuggestionAnswer(context.Background(), owner, source, confirm); err == nil {
		t.Fatal("missing ledger silently dropped a confirmation")
	}
}

func typedSuggestion(t *testing.T, kind string, messageID uuid.UUID, start, end int, revision int64, payload map[string]any) json.RawMessage {
	t.Helper()
	value := map[string]any{"kind": kind, "cited_message_id": messageID.String(),
		"span": map[string]any{"start": start, "end": end}, "base_body_state_revision": revision}
	if payload != nil {
		value["payload"] = payload
	}
	return suggestionJSON(t, value)
}

func TestAttachRuntimeBodyStateSuggestionOnlyTrustsTheTypedField(t *testing.T) {
	_, _, reader := suggestionFixture()
	placeholder := json.RawMessage(`{"question":"要记下来吗？","answer_type":"single_choice","options":["好","不要"]}`)
	if out, err := AttachRuntimeBodyStateSuggestion(placeholder, nil); err != nil || string(out) != string(placeholder) {
		t.Fatalf("plain question changed: %s %v", out, err)
	}
	// A question that carries its own suggestion/purpose/binding loses them.
	smuggled := suggestionQuestion(t, reader.source.ID, 4, 13, 7, map[string]any{"state_binding": map[string]any{"revision": "x"}, "quote": "x"})
	out, err := AttachRuntimeBodyStateSuggestion(smuggled, json.RawMessage("null"))
	if err != nil || strings.Contains(string(out), "suggestion") || strings.Contains(string(out), "state_binding") || strings.Contains(string(out), `"quote"`) {
		t.Fatalf("question-carried suggestion survived: %s %v", out, err)
	}
	typed := typedSuggestion(t, "hypothesis", reader.source.ID, 4, 13, 7, nil)
	out, err = AttachRuntimeBodyStateSuggestion(placeholder, typed)
	if err != nil {
		t.Fatal(err)
	}
	owner, conversation := uuid.New(), reader.source.ConversationID
	if _, ok, err := BindBodyStateSuggestionQuestion(context.Background(), reader, owner, conversation, out); err != nil || !ok {
		t.Fatalf("typed suggestion did not bind: %v %v (%s)", ok, err, out)
	}
	if _, err := AttachRuntimeBodyStateSuggestion(json.RawMessage(`"x"`), typed); err == nil {
		t.Fatal("non-object question accepted")
	}
}

func TestBindBodyStateSuggestionObservationAndEpisodeKinds(t *testing.T) {
	owner, conversation := uuid.New(), uuid.New()
	text := "今早我量了血压是135/88 mmHg。我右肩又开始疼了。"
	reader := &fakeSuggestionSourceReader{source: &model.SuggestionSourceMessage{ID: uuid.New(), ConversationID: conversation, RunID: uuid.New(), Seq: 1, Text: text}, revision: 2, factID: uuid.New()}
	bind := func(typed json.RawMessage) (model.BodyStateSuggestionQuestion, bool, json.RawMessage) {
		question, err := AttachRuntimeBodyStateSuggestion(json.RawMessage(`{"question":"记下来？"}`), typed)
		if err != nil {
			t.Fatal(err)
		}
		out, ok, err := BindBodyStateSuggestionQuestion(context.Background(), reader, owner, conversation, question)
		if err != nil {
			t.Fatal(err)
		}
		var q model.BodyStateSuggestionQuestion
		_ = json.Unmarshal(out, &q)
		return q, ok, out
	}
	q, ok, _ := bind(typedSuggestion(t, "observation", reader.source.ID, 2, 19, 2, map[string]any{"observation_kind": "blood_pressure", "raw_value": "135/88", "raw_unit": "mmHg"}))
	if !ok || q.StateBinding.Kind != "observation" || q.StateBinding.Payload == nil || q.StateBinding.Payload.RawValue != "135/88" || q.Options[0] != "记录为身体观察" {
		t.Fatalf("observation = %+v %v", q, ok)
	}
	q, ok, _ = bind(typedSuggestion(t, "episode", reader.source.ID, 20, 28, 2, map[string]any{"symptom_type": "疼"}))
	if !ok || q.StateBinding.Payload == nil || q.StateBinding.Payload.FactID != reader.factID.String() || reader.excerpts[0] != "我右肩又开始疼了" {
		t.Fatalf("episode = %+v %v %v", q, ok, reader.excerpts)
	}
	unbindable := map[string]json.RawMessage{
		"observation value not the user's":  typedSuggestion(t, "observation", reader.source.ID, 2, 19, 2, map[string]any{"observation_kind": "blood_pressure", "raw_value": "140/90"}),
		"observation clinical label":        typedSuggestion(t, "observation", reader.source.ID, 2, 19, 2, map[string]any{"observation_kind": "高血压"}),
		"episode names its own fact":        typedSuggestion(t, "episode", reader.source.ID, 20, 28, 2, map[string]any{"symptom_type": "疼", "fact_id": uuid.NewString()}),
		"episode without a committed fact":  typedSuggestion(t, "episode", reader.source.ID, 2, 19, 2, map[string]any{"symptom_type": "血压"}),
		"episode symptom words invented":    typedSuggestion(t, "episode", reader.source.ID, 20, 28, 2, map[string]any{"symptom_type": "肩周炎"}),
		"hypothesis with an invented label": typedSuggestion(t, "hypothesis", reader.source.ID, 20, 28, 2, map[string]any{"symptom_type": "疼"}),
		"unknown payload key":               typedSuggestion(t, "observation", reader.source.ID, 2, 19, 2, map[string]any{"observation_kind": "blood_pressure", "diagnosis": "x"}),
	}
	reader.factID = uuid.Nil // no fact for the observation span below
	for name, typed := range unbindable {
		if name == "episode symptom words invented" || name == "episode names its own fact" {
			reader.factID = uuid.New()
		}
		_, ok, out := bind(typed)
		if ok || string(out) != model.BodyStateSuggestionUnverifiedQuestion {
			t.Errorf("%s: bound=%v out=%s", name, ok, out)
		}
		reader.factID = uuid.Nil
	}
}
