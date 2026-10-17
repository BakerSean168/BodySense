package model

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func boundSuggestionFixture(t *testing.T) (json.RawMessage, SuggestionSourceMessage) {
	t.Helper()
	source := SuggestionSourceMessage{ID: uuid.New(), ConversationID: uuid.New(), RunID: uuid.New(), Seq: 2, Text: "我猜是久坐导致腰酸"}
	raw, err := BuildBodyStateSuggestionQuestion(source, 2, 9, 4)
	if err != nil {
		t.Fatal(err)
	}
	return raw, source
}

func TestBodyStateSuggestionExcerptIsExactRuneSpan(t *testing.T) {
	text := "我猜是久坐导致腰酸 "
	if got, ok := BodyStateSuggestionExcerpt(text, 2, 9); !ok || got != "是久坐导致腰酸" {
		t.Fatalf("excerpt = %q %v", got, ok)
	}
	for name, span := range map[string][2]int{"empty": {3, 3}, "reversed": {5, 2}, "negative": {-1, 2}, "past end": {2, 11}, "trailing space": {2, 10}} {
		if _, ok := BodyStateSuggestionExcerpt(text, span[0], span[1]); ok {
			t.Errorf("%s span accepted", name)
		}
	}
	if _, ok := BodyStateSuggestionExcerpt(strings.Repeat("痛", 2001), 0, 2001); ok {
		t.Error("unbounded excerpt accepted")
	}
	if _, ok := BodyStateSuggestionExcerpt("\xff\xfe", 0, 1); ok {
		t.Error("invalid UTF-8 accepted")
	}
}

func TestParseBoundBodyStateSuggestionRoundTrip(t *testing.T) {
	raw, source := boundSuggestionFixture(t)
	command, matched, err := ParseBoundBodyStateSuggestion(raw)
	if err != nil || !matched {
		t.Fatalf("parse = %v %v", matched, err)
	}
	if command.Source.MessageID != source.ID || command.Source.MessageSeq != 2 || command.Source.SpanStart != 2 || command.Source.SpanEnd != 9 ||
		command.BaseBodyStateRevision != 4 || command.Source.ExcerptSHA256 != BodyStateSuggestionDigest("是久坐导致腰酸") {
		t.Fatalf("command = %+v", command)
	}
	if _, matched, err := ParseBoundBodyStateSuggestion([]byte(`{"question":"q","answer_type":"text"}`)); matched || err != nil {
		t.Fatal("ordinary question matched")
	}
}

func TestParseBoundBodyStateSuggestionRejectsTampering(t *testing.T) {
	raw, _ := boundSuggestionFixture(t)
	mutate := func(edit func(q map[string]any)) []byte {
		var q map[string]any
		if err := json.Unmarshal(raw, &q); err != nil {
			t.Fatal(err)
		}
		edit(q)
		out, _ := json.Marshal(q)
		return out
	}
	binding := func(q map[string]any) map[string]any { return q["state_binding"].(map[string]any) }
	source := func(q map[string]any) map[string]any { return binding(q)["source"].(map[string]any) }
	cases := map[string][]byte{
		"edited quote":    mutate(func(q map[string]any) { q["quote"] = "是跑步导致腰酸" }),
		"edited question": mutate(func(q map[string]any) { q["question"] = "直接保存？" }),
		"custom allowed":  mutate(func(q map[string]any) { q["allow_custom_input"] = true }),
		"options swapped": mutate(func(q map[string]any) {
			q["options"] = []any{BodyStateSuggestionRejectOption, BodyStateSuggestionConfirmOption}
		}),
		"extra key":        mutate(func(q map[string]any) { q["receipt"] = map[string]any{"id": "x"} }),
		"binding revision": mutate(func(q map[string]any) { binding(q)["revision"] = "body-state-suggestion-v0" }),
		"binding kind":     mutate(func(q map[string]any) { binding(q)["kind"] = "observation" }),
		"span widened":     mutate(func(q map[string]any) { source(q)["span_end"] = 10 }),
		"digest uppercase": mutate(func(q map[string]any) {
			source(q)["excerpt_sha256"] = strings.ToUpper(source(q)["excerpt_sha256"].(string))
		}),
		"nil message":        mutate(func(q map[string]any) { source(q)["message_id"] = uuid.Nil.String() }),
		"negative revision":  mutate(func(q map[string]any) { binding(q)["base_body_state_revision"] = -1 }),
		"binding key extra":  mutate(func(q map[string]any) { binding(q)["statement"] = "x" }),
		"answer type change": mutate(func(q map[string]any) { q["answer_type"] = "text" }),
	}
	for name, tampered := range cases {
		if _, matched, err := ParseBoundBodyStateSuggestion(tampered); !matched || !errors.Is(err, ErrBodyStateSuggestionRejected) {
			t.Errorf("%s: matched=%v err=%v", name, matched, err)
		}
	}
}

func TestBodyStateSuggestionConfirmed(t *testing.T) {
	if ok, err := BodyStateSuggestionConfirmed([]byte(`{"text":"记录为我的猜测","selected":["记录为我的猜测"]}`)); err != nil || !ok {
		t.Fatalf("confirm = %v %v", ok, err)
	}
	if ok, err := BodyStateSuggestionConfirmed([]byte(`{"text":"不记录","selected":["不记录"]}`)); err != nil || ok {
		t.Fatalf("reject = %v %v", ok, err)
	}
	for _, bad := range []string{``, `{}`, `{"selected":[]}`, `{"selected":["记录为我的猜测","不记录"]}`, `{"selected":["好"]}`, `{"selected":["记录为我的猜测"],"is_custom":true}`} {
		if _, err := BodyStateSuggestionConfirmed([]byte(bad)); !errors.Is(err, ErrBodyStateSuggestionRejected) {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestBuildBodyStateSuggestionQuestionRejectsInexactSpan(t *testing.T) {
	source := SuggestionSourceMessage{ID: uuid.New(), Seq: 1, Text: "膝盖疼"}
	if _, err := BuildBodyStateSuggestionQuestion(source, 0, 9, 0); !errors.Is(err, ErrBodyStateSuggestionRejected) {
		t.Fatalf("out-of-range span bound: %v", err)
	}
	source.Seq = 0
	if _, err := BuildBodyStateSuggestionQuestion(source, 0, 3, 0); !errors.Is(err, ErrBodyStateSuggestionRejected) {
		t.Fatalf("invalid source identity bound: %v", err)
	}
}
