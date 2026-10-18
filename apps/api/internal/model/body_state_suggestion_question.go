package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// BodyStateSuggestionQuestionText is the fixed Ask User prompt Go writes for
// a bound suggestion. The model's wording is never shown as the question.
const BodyStateSuggestionQuestionText = "要把你说过的这句话记录为你自己的猜测吗？"

// BodyStateSuggestionUnverifiedQuestion replaces a runtime suggestion Go
// could not bind to the user's own words. Its answer records nothing.
const BodyStateSuggestionUnverifiedQuestion = `{"question":"这条记录建议没能核对到你的原话，本次不会保存任何内容。",` +
	`"answer_type":"single_choice","options":["知道了"],"allow_custom_input":false,"required":true}`

// BodyStateSuggestionBinding is the Go-written binding persisted inside an
// Ask User question. It is produced only by BuildBodyStateSuggestionQuestion
// after Go resolved the cited user message; the model cannot author it.
type BodyStateSuggestionBinding struct {
	Revision              string                      `json:"revision"`
	Kind                  string                      `json:"kind"`
	Source                BodyStateSuggestionSource   `json:"source"`
	BaseBodyStateRevision int64                       `json:"base_body_state_revision"`
	Payload               *BodyStateSuggestionPayload `json:"payload,omitempty"`
}

// BodyStateSuggestionQuestion is the complete canonical Ask User question
// for a bound suggestion. Quote is the excerpt Go copied from the stored user
// message; it is display only and is re-derived from the message on commit.
type BodyStateSuggestionQuestion struct {
	Purpose          string                     `json:"purpose"`
	Question         string                     `json:"question"`
	AnswerType       string                     `json:"answer_type"`
	Options          []string                   `json:"options"`
	AllowCustomInput bool                       `json:"allow_custom_input"`
	Required         bool                       `json:"required"`
	Quote            string                     `json:"quote"`
	StateBinding     BodyStateSuggestionBinding `json:"state_binding"`
}

// BuildBodyStateSuggestionQuestion binds a suggestion of kind to the rune
// span [start,end) of a resolved user message at the given BodyState
// revision. payload is already Go-resolved (an episode names the fact Go
// found for this exact span). It fails closed for any span that is not an
// exact excerpt of the message and for any payload that is not the user's
// own words.
func BuildBodyStateSuggestionQuestion(source SuggestionSourceMessage, kind string, start, end int, baseRevision int64, payload BodyStateSuggestionPayload) (json.RawMessage, error) {
	excerpt, ok := BodyStateSuggestionExcerpt(source.Text, start, end)
	if !ok {
		return nil, fmt.Errorf("%w: cited span is not an exact excerpt of the user message", ErrBodyStateSuggestionRejected)
	}
	if err := ValidateBodyStateSuggestionPayload(kind, payload, excerpt, true); err != nil {
		return nil, err
	}
	command := ConfirmBodyStateSuggestionCommand{
		Kind: kind,
		Source: BodyStateSuggestionSource{MessageID: source.ID, MessageSeq: source.Seq, SpanStart: start, SpanEnd: end,
			ExcerptSHA256: BodyStateSuggestionDigest(excerpt)},
		BaseBodyStateRevision: baseRevision,
		Payload:               payload,
	}
	if err := command.Validate(); err != nil {
		return nil, err
	}
	binding := BodyStateSuggestionBinding{Revision: BodyStateSuggestionBindingRevision, Kind: kind,
		Source: command.Source, BaseBodyStateRevision: baseRevision}
	if payload != (BodyStateSuggestionPayload{}) {
		binding.Payload = &payload
	}
	spec := bodyStateSuggestionKinds[kind]
	return json.Marshal(BodyStateSuggestionQuestion{
		Purpose: BodyStateSuggestionPurpose, Question: spec.Question, AnswerType: "single_choice",
		Options:  []string{spec.ConfirmOption, BodyStateSuggestionRejectOption},
		Required: true, Quote: excerpt, StateBinding: binding,
	})
}

// ParseBoundBodyStateSuggestion reads a persisted question. matched is false
// for any question that does not declare the suggestion purpose; those never
// write BodyState. A question that declares the purpose but is not exactly
// the canonical Go-written shape is matched and rejected, so a forged or
// edited binding can neither commit nor degrade into an ordinary answer.
func ParseBoundBodyStateSuggestion(question []byte) (ConfirmBodyStateSuggestionCommand, bool, error) {
	var envelope struct {
		Purpose string `json:"purpose"`
	}
	if len(question) == 0 || json.Unmarshal(question, &envelope) != nil || envelope.Purpose != BodyStateSuggestionPurpose {
		return ConfirmBodyStateSuggestionCommand{}, false, nil
	}
	reject := func(reason string) (ConfirmBodyStateSuggestionCommand, bool, error) {
		return ConfirmBodyStateSuggestionCommand{}, true, fmt.Errorf("%w: %s", ErrBodyStateSuggestionRejected, reason)
	}
	var q BodyStateSuggestionQuestion
	decoder := json.NewDecoder(bytes.NewReader(question))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&q) != nil || decoder.Decode(new(any)) != io.EOF {
		return reject("suggestion question is not canonical")
	}
	binding := q.StateBinding
	spec, supported := bodyStateSuggestionKinds[binding.Kind]
	if !supported || q.Question != spec.Question || q.AnswerType != "single_choice" || q.AllowCustomInput || !q.Required ||
		len(q.Options) != 2 || q.Options[0] != spec.ConfirmOption || q.Options[1] != BodyStateSuggestionRejectOption ||
		binding.Revision != BodyStateSuggestionBindingRevision {
		return reject("suggestion question is not canonical")
	}
	var payload BodyStateSuggestionPayload
	if binding.Payload != nil {
		// An empty payload object is not the canonical (omitted) shape.
		if *binding.Payload == (BodyStateSuggestionPayload{}) {
			return reject("suggestion payload is not canonical")
		}
		payload = *binding.Payload
	}
	command := ConfirmBodyStateSuggestionCommand{Kind: binding.Kind, Source: binding.Source,
		BaseBodyStateRevision: binding.BaseBodyStateRevision, Payload: payload}
	if err := command.Validate(); err != nil {
		return reject("suggestion binding is malformed")
	}
	if len([]rune(q.Quote)) != binding.Source.SpanEnd-binding.Source.SpanStart || BodyStateSuggestionDigest(q.Quote) != binding.Source.ExcerptSHA256 {
		return reject("suggestion quote does not match its binding")
	}
	if err := ValidateBodyStateSuggestionPayload(command.Kind, payload, q.Quote, true); err != nil {
		return reject("suggestion payload does not match its quote")
	}
	return command, true, nil
}

// BodyStateSuggestionConfirmed reports the user's decision on a bound
// suggestion of kind from the accepted (already validated) answer: exactly
// one of the two canonical options. Anything else is rejected, not read as no.
func BodyStateSuggestionConfirmed(kind string, answer []byte) (bool, error) {
	var payload struct {
		Selected []string `json:"selected"`
		IsCustom bool     `json:"is_custom"`
	}
	if len(answer) == 0 || json.Unmarshal(answer, &payload) != nil || payload.IsCustom || len(payload.Selected) != 1 {
		return false, fmt.Errorf("%w: suggestion answer is not a single canonical choice", ErrBodyStateSuggestionRejected)
	}
	spec, supported := bodyStateSuggestionKinds[kind]
	switch {
	case supported && payload.Selected[0] == spec.ConfirmOption:
		return true, nil
	case supported && payload.Selected[0] == BodyStateSuggestionRejectOption:
		return false, nil
	default:
		return false, fmt.Errorf("%w: suggestion answer is not a canonical choice", ErrBodyStateSuggestionRejected)
	}
}
