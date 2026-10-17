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
