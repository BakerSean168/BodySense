package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// Structured symptom intake (safety screen) contract.
//
// The consultation runtime asks the bounded safety checklist right after a
// turn commits a non-absent V2 symptom statement through record_body_state.
// The runtime may only *reference* that commit (producer tool call + fact
// index). Go resolves the reference against the receipts it committed in the
// same stream, writes the canonical binding (the committed body_fact_id and
// the canonical checklist field) into the persisted question, and strips any
// binding it cannot resolve. On resume, only a question carrying that
// Go-written binding records a safety screen; it never inserts a second fact
// for the same symptom.
const (
	SymptomIntakePurpose         = "symptom_intake"
	SymptomIntakeBindingRevision = "symptom-intake-binding-v2"
	symptomIntakeSafetyField     = "safety_signals"
	symptomIntakeNoneOption      = "以上均无"
	symptomIntakeSafetyLabel     = "请选出目前出现的所有安全信号（可多选）；若五项都没有，请只选“以上均无”。"
)

var bodyStateSafetySignalOptions = []struct{ option, concept string }{
	{"外伤或创伤", "trauma"}, {"放射痛", "radiating_pain"}, {"麻木", "numbness"},
	{"无力", "weakness"}, {"头晕", "dizziness"},
}

// CommittedBodyFact is one fact of a record_body_state batch that Go committed
// in the current runtime stream, addressed by its producer tool call and index.
type CommittedBodyFact struct {
	ToolCallID string
	Index      int
	FactID     uuid.UUID
	Kind       string
	Presence   string
}

// SymptomIntakeCaptureID derives the capture identity from the committed fact
// reference, so a capture cannot be re-pointed at another fact.
func SymptomIntakeCaptureID(factToolCallID string, index int) string {
	sum := sha256.Sum256([]byte("symptom-intake\x00" + factToolCallID + "\x00" + strconv.Itoa(index)))
	return hex.EncodeToString(sum[:12])
}

func symptomIntakeSafetyFieldSpec() map[string]any {
	options := make([]any, 0, len(bodyStateSafetySignalOptions)+1)
	for _, item := range bodyStateSafetySignalOptions {
		options = append(options, item.option)
	}
	options = append(options, symptomIntakeNoneOption)
	return map[string]any{
		"key": symptomIntakeSafetyField, "label": symptomIntakeSafetyLabel, "answer_type": "multi_choice",
		"options": options, "exclusive_options": []any{symptomIntakeNoneOption}, "required": true,
	}
}

// BindSymptomIntakeQuestion returns the question Go persists for a runtime
// interrupt. A symptom_intake question whose fact reference resolves to a
// non-absent symptom committed in this stream gets the canonical Go binding
// and checklist; any other binding (model-authored, stale, legacy, malformed)
// is removed so the answer can create nothing. bound reports which happened.
func BindSymptomIntakeQuestion(question json.RawMessage, toolCallID string, committed []CommittedBodyFact) (json.RawMessage, bool, error) {
	var q map[string]any
	if err := json.Unmarshal(question, &q); err != nil || q == nil {
		return nil, false, fmt.Errorf("interaction question must be an object")
	}
	_, hasBinding := q["state_binding"]
	if q["purpose"] != SymptomIntakePurpose && !hasBinding {
		return question, false, nil
	}
	factID, captureID, ok := resolveSymptomIntakeReference(q, toolCallID, committed)
	delete(q, "state_binding")
	if !ok {
		delete(q, "purpose")
		stripped, err := json.Marshal(q)
		return stripped, false, err
	}
	q["purpose"] = SymptomIntakePurpose
	q["answer_type"] = "text"
	q["required"] = true
	q["fields"] = []any{symptomIntakeSafetyFieldSpec()}
	q["state_binding"] = map[string]any{
		"revision": SymptomIntakeBindingRevision, "capture_id": captureID, "body_fact_id": factID.String(),
		"field_map": map[string]any{symptomIntakeSafetyField: symptomIntakeSafetyField},
	}
	bound, err := json.Marshal(q)
	return bound, err == nil, err
}

func resolveSymptomIntakeReference(q map[string]any, toolCallID string, committed []CommittedBodyFact) (uuid.UUID, string, bool) {
	if q["purpose"] != SymptomIntakePurpose {
		return uuid.Nil, "", false
	}
	binding, _ := q["state_binding"].(map[string]any)
	ref, _ := binding["fact_ref"].(map[string]any)
	if binding == nil || ref == nil || binding["revision"] != SymptomIntakeBindingRevision {
		return uuid.Nil, "", false
	}
	factToolCallID, _ := ref["tool_call_id"].(string)
	index, ok := ref["index"].(float64)
	if !ok || factToolCallID == "" || index < 0 || index > 19 || index != float64(int(index)) {
		return uuid.Nil, "", false
	}
	captureID := SymptomIntakeCaptureID(factToolCallID, int(index))
	if binding["capture_id"] != captureID || toolCallID != "intake-"+captureID {
		return uuid.Nil, "", false
	}
	for _, fact := range committed {
		if fact.ToolCallID == factToolCallID && fact.Index == int(index) {
			if fact.Kind != "symptom" || fact.Presence == "absent" || fact.FactID == uuid.Nil {
				return uuid.Nil, "", false
			}
			return fact.FactID, captureID, true
		}
	}
	return uuid.Nil, "", false
}

// boundSafetyScreen parses a persisted, Go-bound intake question and its
// accepted answer. matched is false for every question without the Go
// binding; those answers never record health state.
func boundSafetyScreen(toolCallID string, question, answer json.RawMessage) (model.SafetyScreenCommand, bool, error) {
	var envelope struct {
		Purpose string `json:"purpose"`
		Fields  []struct {
			Key              string   `json:"key"`
			AnswerType       string   `json:"answer_type"`
			Options          []string `json:"options"`
			ExclusiveOptions []string `json:"exclusive_options"`
			Required         bool     `json:"required"`
		} `json:"fields"`
		StateBinding struct {
			Revision   string            `json:"revision"`
			CaptureID  string            `json:"capture_id"`
			BodyFactID string            `json:"body_fact_id"`
			FieldMap   map[string]string `json:"field_map"`
		} `json:"state_binding"`
	}
	if len(question) == 0 || json.Unmarshal(question, &envelope) != nil || envelope.Purpose != SymptomIntakePurpose {
		return model.SafetyScreenCommand{}, false, nil
	}
	binding := envelope.StateBinding
	factID, err := uuid.Parse(binding.BodyFactID)
	if binding.Revision != SymptomIntakeBindingRevision || !bodyStateValidCaptureID(binding.CaptureID) ||
		toolCallID != "intake-"+binding.CaptureID || err != nil || factID == uuid.Nil {
		return model.SafetyScreenCommand{}, false, nil
	}
	invalid := func(reason string) (model.SafetyScreenCommand, bool, error) {
		return model.SafetyScreenCommand{}, true, fmt.Errorf("%w: %s", model.ErrInvalidBodyFact, reason)
	}
	if len(binding.FieldMap) != 1 || binding.FieldMap[symptomIntakeSafetyField] != symptomIntakeSafetyField || len(envelope.Fields) != 1 {
		return invalid("symptom intake binding is not canonical")
	}
	field := envelope.Fields[0]
	if field.Key != symptomIntakeSafetyField || field.AnswerType != "multi_choice" || !field.Required ||
		len(field.Options) != len(bodyStateSafetySignalOptions)+1 || len(field.ExclusiveOptions) != 1 ||
		field.ExclusiveOptions[0] != symptomIntakeNoneOption || field.Options[len(bodyStateSafetySignalOptions)] != symptomIntakeNoneOption {
		return invalid("symptom intake checklist is not canonical")
	}
	for i, option := range bodyStateSafetySignalOptions {
		if field.Options[i] != option.option {
			return invalid("symptom intake checklist is not canonical")
		}
	}
	var payload struct {
		Fields map[string]any `json:"fields"`
	}
	if len(answer) == 0 || json.Unmarshal(answer, &payload) != nil {
		return invalid("symptom intake answer is malformed")
	}
	answers, ok := bodyStateSafetySelection(payload.Fields[symptomIntakeSafetyField])
	if !ok {
		return invalid("symptom intake checklist answer is incomplete")
	}
	return model.SafetyScreenCommand{CaptureID: binding.CaptureID, BodyFactID: factID, Answers: answers}, true, nil
}

func bodyStateSafetySelection(value any) (model.SafetyScreenAnswers, bool) {
	items, ok := value.([]any)
	if !ok || len(items) == 0 {
		return model.SafetyScreenAnswers{}, false
	}
	selected := map[string]bool{}
	for _, item := range items {
		option, ok := item.(string)
		if !ok || selected[option] {
			return model.SafetyScreenAnswers{}, false
		}
		selected[option] = true
	}
	if selected[symptomIntakeNoneOption] && len(selected) != 1 {
		return model.SafetyScreenAnswers{}, false
	}
	answers := model.SafetyScreenAnswers{}
	targets := map[string]*bool{
		"trauma": &answers.Trauma, "radiating_pain": &answers.RadiatingPain, "numbness": &answers.Numbness,
		"weakness": &answers.Weakness, "dizziness": &answers.Dizziness,
	}
	for _, item := range bodyStateSafetySignalOptions {
		*targets[item.concept] = selected[item.option]
		delete(selected, item.option)
	}
	delete(selected, symptomIntakeNoneOption)
	return answers, len(selected) == 0
}

func bodyStateValidCaptureID(value string) bool {
	if len(value) != 24 || strings.ToLower(value) != value {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 12
}

type safetyScreenLedger interface {
	RecordSafetyScreen(ctx context.Context, userID uuid.UUID, source model.AnswerFactSource, command model.SafetyScreenCommand) (*model.SafetyScreen, error)
}

// RecordStructuredIntakeAnswer records the safety screen of a Go-bound symptom
// intake answer inside the interaction-resume claim transaction. The screen is
// tied to the exact V2 symptom fact Go bound when it persisted the question;
// no second symptom fact is created. Any other interaction answer, including a
// model-authored ask_user without the Go binding, records nothing.
func (s *BodyStateService) RecordStructuredIntakeAnswer(
	ctx context.Context,
	userID uuid.UUID,
	source model.AnswerFactSource,
	interaction *model.AgentInteraction,
) error {
	if interaction == nil {
		return nil
	}
	command, matched, err := boundSafetyScreen(interaction.ToolCallID, json.RawMessage(interaction.Question), json.RawMessage(interaction.Answer))
	if !matched {
		return nil
	}
	if err != nil {
		return err
	}
	ledger, ok := s.repo.(safetyScreenLedger)
	if !ok {
		return errors.New("safety screen ledger is not configured")
	}
	_, err = ledger.RecordSafetyScreen(ctx, userID, source, command)
	return err
}

// IsBoundSymptomIntake reports whether a persisted question carries the Go
// symptom intake binding, whose answer must not be accepted without a recorder.
func IsBoundSymptomIntake(toolCallID string, question json.RawMessage) bool {
	_, matched, _ := boundSafetyScreen(toolCallID, question, nil)
	return matched
}
