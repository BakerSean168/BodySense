package model

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestInteractionAnswerIsBoundToExactQuestion(t *testing.T) {
	cases := []struct {
		name, question, answer, text string
		valid                        bool
	}{
		{"text", `{"question":"How do you feel?","answer_type":"text"}`, `{"text":"I have no shoulder pain"}`, "I have no shoulder pain", true},
		{"choice", `{"question":"Which side?","answer_type":"single_choice","options":["left","right"]}`, `{"text":"left","selected":["left"]}`, "left", true},
		{"choice summary forgery", `{"question":"Which side?","answer_type":"single_choice","options":["left","right"]}`, `{"text":"right","selected":["left"]}`, "", false},
		{"foreign choice", `{"question":"Which side?","answer_type":"single_choice","options":["left","right"]}`, `{"selected":["both"]}`, "", false},
		{"multiple single", `{"question":"Which side?","answer_type":"single_choice","options":["left","right"]}`, `{"selected":["left","right"]}`, "", false},
		{"custom forbidden", `{"question":"Which side?","answer_type":"single_choice","options":["left","right"],"allow_custom_input":false}`, `{"selected":["both"],"is_custom":true}`, "", false},
		{"custom permitted", `{"question":"Which side?","answer_type":"single_choice","options":["left","right"],"allow_custom_input":true}`, `{"selected":["I am not sure"],"is_custom":true}`, "I am not sure", true},
		{"duplicates", `{"question":"Where?","answer_type":"multi_choice","options":["left","right"]}`, `{"selected":["left","left"]}`, "", false},
		{"number", `{"question":"How long?","answer_type":"number"}`, `{"text":"7","value":7}`, "7", true},
		{"numeric forgery", `{"question":"How long?","answer_type":"number"}`, `{"text":"7","value":70}`, "", false},
		{"not finite", `{"question":"How long?","answer_type":"number"}`, `{"text":"NaN"}`, "", false},
		{"valid date", `{"question":"When?","answer_type":"date"}`, `{"text":"2026-10-13"}`, "2026-10-13", true},
		{"invalid date", `{"question":"When?","answer_type":"date"}`, `{"text":"2026-02-30"}`, "", false},
		{"extra source authority", `{"question":"How do you feel?","answer_type":"text"}`, `{"text":"I feel well","source_id":"forged"}`, "", false},
		{"missing question", `{}`, `{"text":"I feel well"}`, "", false},
		{"unknown type", `{"question":"How?","answer_type":"object"}`, `{"text":"anything"}`, "", false},
		{"empty", `{"question":"How?","answer_type":"text"}`, `{"text":" "}`, "", false},
		{"trailing JSON", `{"question":"How?","answer_type":"text"}`, `{"text":"I feel well"}{}`, "", false},
		{"field bound", `{"question":"Details?","fields":[{"key":"side","label":"Side","answer_type":"single_choice","options":["left","right"]}]}`, `{"text":"Side: left","fields":{"side":"left"}}`, "Side: left", true},
		{"field summary forged", `{"question":"Details?","fields":[{"key":"side","label":"Side","answer_type":"single_choice","options":["left","right"]}]}`, `{"text":"I have right shoulder pain","fields":{"side":"left"}}`, "", false},
		{"unknown field", `{"question":"Details?","fields":[{"key":"side","label":"Side","answer_type":"text"}]}`, `{"fields":{"side":"left","clinical":"diagnosed"}}`, "", false},
		{"exclusive field", `{"question":"Details?","fields":[{"key":"symptom","label":"Symptoms","answer_type":"multi_choice","options":["none","pain"],"exclusive_options":["none"]}]}`, `{"fields":{"symptom":["none","pain"]}}`, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, text, err := ValidateInteractionAnswer([]byte(tc.question), []byte(tc.answer))
			if !tc.valid {
				if !errors.Is(err, ErrInvalidInteractionAnswer) || raw != nil || text != "" {
					t.Fatalf("invalid answer accepted: %s %q %v", raw, text, err)
				}
				return
			}
			if err != nil || text != tc.text || !json.Valid(raw) {
				t.Fatalf("answer=%s text=%q err=%v", raw, text, err)
			}
			again, againText, err := ValidateInteractionAnswer([]byte(tc.question), raw)
			if err != nil || againText != text || string(again) != string(raw) {
				t.Fatal("normalized answer is not stable")
			}
		})
	}
}

func TestInteractionFingerprintPreservesLargeNumbersAndIgnoresKeyOrder(t *testing.T) {
	a, _ := InteractionJSONFingerprint([]byte(`{"n":9007199254740993,"ok":true}`))
	b, _ := InteractionJSONFingerprint([]byte(`{"ok":true,"n":9007199254740993}`))
	c, _ := InteractionJSONFingerprint([]byte(`{"ok":true,"n":9007199254740992}`))
	if a != b || a == c || len(a) != 64 {
		t.Fatal("fingerprint lost content identity")
	}
	if _, err := InteractionJSONFingerprint([]byte(`{} {}`)); err == nil {
		t.Fatal("trailing JSON accepted")
	}
}
