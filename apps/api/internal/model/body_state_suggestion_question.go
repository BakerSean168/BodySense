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

// BodyStateSuggestionBinding is the Go-written binding persisted inside an
// Ask User question. It is produced only by BuildBodyStateSuggestionQuestion
// after Go resolved the cited user message; the model cannot author it.
type BodyStateSuggestionBinding struct {
	Revision              string                    `json:"revision"`
	Kind                  string                    `json:"kind"`
	Source                BodyStateSuggestionSource `json:"source"`
	BaseBodyStateRevision int64                     `json:"base_body_state_revision"`
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

// BuildBodyStateSuggestionQuestion binds a hypothesis suggestion to the rune
// span [start,end) of a resolved user message at the given BodyState revision.
// It fails closed for any span that is not an exact excerpt of the message.
func BuildBodyStateSuggestionQuestion(source SuggestionSourceMessage, start, end int, baseRevision int64) (json.RawMessage, error) {
	excerpt, ok := BodyStateSuggestionExcerpt(source.Text, start, end)
	if !ok {
		return nil, fmt.Errorf("%w: cited span is not an exact excerpt of the user message", ErrBodyStateSuggestionRejected)
	}
	command := ConfirmBodyHypothesisSuggestionCommand{
		Source: BodyStateSuggestionSource{MessageID: source.ID, MessageSeq: source.Seq, SpanStart: start, SpanEnd: end,
			ExcerptSHA256: BodyStateSuggestionDigest(excerpt)},
		BaseBodyStateRevision: baseRevision,
	}
	if err := command.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(BodyStateSuggestionQuestion{
		Purpose: BodyStateSuggestionPurpose, Question: BodyStateSuggestionQuestionText, AnswerType: "single_choice",
		Options:  []string{BodyStateSuggestionConfirmOption, BodyStateSuggestionRejectOption},
		Required: true, Quote: excerpt,
		StateBinding: BodyStateSuggestionBinding{Revision: BodyStateSuggestionBindingRevision, Kind: BodyStateSuggestionKindHypothesis,
			Source: command.Source, BaseBodyStateRevision: baseRevision},
	})
}

// ParseBoundBodyStateSuggestion reads a persisted question. matched is false
// for any question that does not declare the suggestion purpose; those never
// write BodyState. A question that declares the purpose but is not exactly
// the canonical Go-written shape is matched and rejected, so a forged or
// edited binding can neither commit nor degrade into an ordinary answer.
func ParseBoundBodyStateSuggestion(question []byte) (ConfirmBodyHypothesisSuggestionCommand, bool, error) {
	var envelope struct {
		Purpose string `json:"purpose"`
	}
	if len(question) == 0 || json.Unmarshal(question, &envelope) != nil || envelope.Purpose != BodyStateSuggestionPurpose {
		return ConfirmBodyHypothesisSuggestionCommand{}, false, nil
	}
	reject := func(reason string) (ConfirmBodyHypothesisSuggestionCommand, bool, error) {
		return ConfirmBodyHypothesisSuggestionCommand{}, true, fmt.Errorf("%w: %s", ErrBodyStateSuggestionRejected, reason)
	}
	var q BodyStateSuggestionQuestion
	decoder := json.NewDecoder(bytes.NewReader(question))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&q) != nil || decoder.Decode(new(any)) != io.EOF {
		return reject("suggestion question is not canonical")
	}
	binding := q.StateBinding
	if q.Question != BodyStateSuggestionQuestionText || q.AnswerType != "single_choice" || q.AllowCustomInput || !q.Required ||
		len(q.Options) != 2 || q.Options[0] != BodyStateSuggestionConfirmOption || q.Options[1] != BodyStateSuggestionRejectOption ||
		binding.Revision != BodyStateSuggestionBindingRevision || binding.Kind != BodyStateSuggestionKindHypothesis {
		return reject("suggestion question is not canonical")
	}
	command := ConfirmBodyHypothesisSuggestionCommand{Source: binding.Source, BaseBodyStateRevision: binding.BaseBodyStateRevision}
	if err := command.Validate(); err != nil {
		return reject("suggestion binding is malformed")
	}
	if len([]rune(q.Quote)) != binding.Source.SpanEnd-binding.Source.SpanStart || BodyStateSuggestionDigest(q.Quote) != binding.Source.ExcerptSHA256 {
		return reject("suggestion quote does not match its binding")
	}
	return command, true, nil
}

// BodyStateSuggestionConfirmed reports the user's decision on a bound
// suggestion from the accepted (already validated) answer: exactly one of the
// two canonical options. Anything else is rejected rather than read as a no.
func BodyStateSuggestionConfirmed(answer []byte) (bool, error) {
	var payload struct {
		Selected []string `json:"selected"`
		IsCustom bool     `json:"is_custom"`
	}
	if len(answer) == 0 || json.Unmarshal(answer, &payload) != nil || payload.IsCustom || len(payload.Selected) != 1 {
		return false, fmt.Errorf("%w: suggestion answer is not a single canonical choice", ErrBodyStateSuggestionRejected)
	}
	switch payload.Selected[0] {
	case BodyStateSuggestionConfirmOption:
		return true, nil
	case BodyStateSuggestionRejectOption:
		return false, nil
	default:
		return false, fmt.Errorf("%w: suggestion answer is not a canonical choice", ErrBodyStateSuggestionRejected)
	}
}
