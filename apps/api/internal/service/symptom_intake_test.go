package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

const testFactToolCall = "bodyfact-intake-v1:11111111-1111-4111-8111-111111111111"

// runtimeIntakeQuestion is the question shape the Python runtime emits: it may
// only reference a committed fact by producer call + index.
func runtimeIntakeQuestion(factToolCall string, index int) (string, json.RawMessage) {
	capture := SymptomIntakeCaptureID(factToolCall, index)
	raw, _ := json.Marshal(map[string]any{
		"question": "我已记录：“我昨天肩膀不舒服。”为了安全分层，请确认：", "answer_type": "text", "purpose": "symptom_intake",
		"fields": []any{map[string]any{"key": "safety_signals", "label": "x", "answer_type": "multi_choice", "options": []string{"以上均无"}}},
		"state_binding": map[string]any{
			"revision": SymptomIntakeBindingRevision, "capture_id": capture,
			"fact_ref":  map[string]any{"tool_call_id": factToolCall, "index": index},
			"field_map": map[string]any{"safety_signals": "safety_signals"},
		},
	})
	return "intake-" + capture, raw
}

func committedSymptom(presence string) []CommittedBodyFact {
	return []CommittedBodyFact{
		{ToolCallID: testFactToolCall, Index: 0, FactID: uuid.New(), Kind: "lifestyle"},
		{ToolCallID: testFactToolCall, Index: 1, FactID: uuid.New(), Kind: "symptom", Presence: presence},
	}
}

func boundTestQuestion(t *testing.T) (string, json.RawMessage, uuid.UUID) {
	t.Helper()
	toolCall, raw := runtimeIntakeQuestion(testFactToolCall, 1)
	facts := committedSymptom("present")
	bound, ok, err := BindSymptomIntakeQuestion(raw, toolCall, facts)
	if err != nil || !ok {
		t.Fatalf("expected a bound question: ok=%v err=%v", ok, err)
	}
	return toolCall, bound, facts[1].FactID
}

func TestBindSymptomIntakeQuestionWritesCanonicalGoBinding(t *testing.T) {
	toolCall, bound, factID := boundTestQuestion(t)
	var q struct {
		Purpose string `json:"purpose"`
		Fields  []struct {
			Key              string   `json:"key"`
			Options          []string `json:"options"`
			ExclusiveOptions []string `json:"exclusive_options"`
		} `json:"fields"`
		StateBinding map[string]any `json:"state_binding"`
	}
	if err := json.Unmarshal(bound, &q); err != nil {
		t.Fatal(err)
	}
	if q.Purpose != SymptomIntakePurpose || len(q.Fields) != 1 || q.Fields[0].Key != "safety_signals" ||
		strings.Join(q.Fields[0].Options, "|") != "外伤或创伤|放射痛|麻木|无力|头晕|以上均无" || strings.Join(q.Fields[0].ExclusiveOptions, "|") != "以上均无" {
		t.Fatalf("checklist was not replaced by the canonical Go field: %s", bound)
	}
	if q.StateBinding["body_fact_id"] != factID.String() || q.StateBinding["fact_ref"] != nil || "intake-"+q.StateBinding["capture_id"].(string) != toolCall {
		t.Fatalf("binding is not the Go-resolved fact: %#v", q.StateBinding)
	}
	if !IsBoundSymptomIntake(toolCall, bound) {
		t.Fatal("bound question must be recognized at resume")
	}
}

func TestBindSymptomIntakeQuestionStripsUnresolvableBindings(t *testing.T) {
	toolCall, raw := runtimeIntakeQuestion(testFactToolCall, 1)
	otherCall, otherRaw := runtimeIntakeQuestion(testFactToolCall, 7)
	_, boundQuestion, _ := boundTestQuestion(t)
	for name, tc := range map[string]struct {
		toolCall string
		question json.RawMessage
		facts    []CommittedBodyFact
	}{
		"no-commit-in-stream":   {toolCall, raw, nil},
		"absent-symptom":        {toolCall, raw, committedSymptom("absent")},
		"not-a-symptom":         {otherCall, otherRaw, append(committedSymptom("present"), CommittedBodyFact{ToolCallID: testFactToolCall, Index: 7, FactID: uuid.New(), Kind: "activity"})},
		"model-tool-call":       {"call_model_1", raw, committedSymptom("present")},
		"capture-not-derived":   {toolCall, json.RawMessage(strings.Replace(string(raw), `"index":1`, `"index":0`, 1)), committedSymptom("present")},
		"forged-go-binding":     {toolCall, boundQuestion, committedSymptom("present")},
		"binding-other-purpose": {toolCall, json.RawMessage(strings.Replace(string(raw), `"purpose":"symptom_intake"`, `"purpose":"other"`, 1)), committedSymptom("present")},
	} {
		t.Run(name, func(t *testing.T) {
			out, ok, err := BindSymptomIntakeQuestion(tc.question, tc.toolCall, tc.facts)
			if err != nil || ok {
				t.Fatalf("expected an unbound question: ok=%v err=%v", ok, err)
			}
			if strings.Contains(string(out), "state_binding") || strings.Contains(string(out), "symptom_intake") {
				t.Fatalf("unresolved binding must be stripped: %s", out)
			}
			if IsBoundSymptomIntake(tc.toolCall, out) {
				t.Fatal("stripped question must not record anything")
			}
		})
	}
	plain := json.RawMessage(`{"question":"左边还是右边？","answer_type":"single_choice","options":["左","右"]}`)
	if out, ok, err := BindSymptomIntakeQuestion(plain, "call_1", nil); err != nil || ok || string(out) != string(plain) {
		t.Fatalf("ordinary questions pass through untouched: %s %v %v", out, ok, err)
	}
}

func testBoundInteraction(t *testing.T, answer string) (*model.AgentInteraction, uuid.UUID) {
	toolCall, bound, factID := boundTestQuestion(t)
	return &model.AgentInteraction{ID: uuid.New(), ToolName: "ask_user", ToolCallID: toolCall, Question: datatypes.JSON(bound), Answer: datatypes.JSON(answer)}, factID
}

func TestStructuredIntakeAnswerRecordsScreenForBoundFactOnly(t *testing.T) {
	for _, tc := range []struct {
		name    string
		answer  string
		screen  *model.SafetyScreenAnswers
		wantErr bool
	}{
		{"none", `{"fields":{"safety_signals":["以上均无"]}}`, &model.SafetyScreenAnswers{}, false},
		{"selected", `{"fields":{"safety_signals":["放射痛","麻木"]}}`, &model.SafetyScreenAnswers{RadiatingPain: true, Numbness: true}, false},
		{"contradictory", `{"fields":{"safety_signals":["以上均无","麻木"]}}`, nil, true},
		{"unknown", `{"fields":{"safety_signals":["未知"]}}`, nil, true},
		{"missing", `{"fields":{}}`, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeBodyStateRepository{}
			interaction, factID := testBoundInteraction(t, tc.answer)
			err := NewBodyStateService(repo).RecordStructuredIntakeAnswer(context.Background(), uuid.New(), model.AnswerFactSource{RunID: uuid.New(), InteractionID: interaction.ID, MessageID: uuid.New()}, interaction)
			if tc.wantErr {
				if err == nil || len(repo.screens) != 0 {
					t.Fatalf("invalid answer must record nothing: err=%v screens=%#v", err, repo.screens)
				}
				return
			}
			if err != nil || len(repo.screens) != 1 {
				t.Fatalf("err=%v screens=%#v", err, repo.screens)
			}
			got := repo.screens[0]
			if got.Answers != *tc.screen || got.BodyFactID != factID || "intake-"+got.CaptureID != interaction.ToolCallID {
				t.Fatalf("screen = %#v, want %#v for fact %s", got, tc.screen, factID)
			}
		})
	}
}

func TestUnboundAskUserCannotRecordSafetyScreen(t *testing.T) {
	answer := datatypes.JSON(`{"fields":{"safety_signals":["以上均无"]}}`)
	toolCall, bound, _ := boundTestQuestion(t)
	_, runtimeRaw := runtimeIntakeQuestion(testFactToolCall, 1)
	for name, interaction := range map[string]*model.AgentInteraction{
		"model-tool-call":      {ToolCallID: "model-tool-call", Question: datatypes.JSON(bound), Answer: answer},
		"wrong-capture":        {ToolCallID: "intake-ffffffffffffffffffffffff", Question: datatypes.JSON(bound), Answer: answer},
		"runtime-unresolved":   {ToolCallID: toolCall, Question: datatypes.JSON(runtimeRaw), Answer: answer},
		"other-purpose":        {ToolCallID: toolCall, Question: datatypes.JSON(strings.Replace(string(bound), `"purpose":"symptom_intake"`, `"purpose":"other"`, 1)), Answer: answer},
		"wrong-revision":       {ToolCallID: toolCall, Question: datatypes.JSON(strings.Replace(string(bound), SymptomIntakeBindingRevision, "symptom-intake-binding-v1", 1)), Answer: answer},
		"ordinary-interaction": {ToolCallID: "call_1", Question: datatypes.JSON(`{"question":"q"}`), Answer: datatypes.JSON(`{"text":"我昨天肩膀不舒服。"}`)},
	} {
		repo := &fakeBodyStateRepository{}
		if err := NewBodyStateService(repo).RecordStructuredIntakeAnswer(context.Background(), uuid.New(), model.AnswerFactSource{RunID: uuid.New(), InteractionID: uuid.New(), MessageID: uuid.New()}, interaction); err != nil {
			t.Fatalf("%s: unbound answer must be ignored, got %v", name, err)
		}
		if len(repo.screens) != 0 {
			t.Fatalf("%s: unbound ask_user recorded a screen: %#v", name, repo.screens)
		}
	}
}

func TestBoundIntakeWithTamperedChecklistIsRejected(t *testing.T) {
	interaction, _ := testBoundInteraction(t, `{"fields":{"safety_signals":["以上均无"]}}`)
	interaction.Question = datatypes.JSON(strings.Replace(string(interaction.Question), `"exclusive_options":["以上均无"]`, `"exclusive_options":["麻木"]`, 1))
	repo := &fakeBodyStateRepository{}
	if err := NewBodyStateService(repo).RecordStructuredIntakeAnswer(context.Background(), uuid.New(), model.AnswerFactSource{RunID: uuid.New(), InteractionID: interaction.ID, MessageID: uuid.New()}, interaction); err == nil || len(repo.screens) != 0 {
		t.Fatalf("tampered checklist must fail closed: err=%v screens=%#v", err, repo.screens)
	}
}
