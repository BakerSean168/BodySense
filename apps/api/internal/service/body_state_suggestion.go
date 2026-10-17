package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// Source-bound BodyState suggestion binder (BS-AGENT-001).
//
// The consultation runtime may ask the user to confirm a BodyState item only
// by citing a span of the user's own chat message:
//
//	{"purpose":"body_state_suggestion","question":"…",
//	 "suggestion":{"kind":"hypothesis","cited_message_id":"<uuid>",
//	               "span":{"start":0,"end":12},"base_body_state_revision":3}}
//
// Go resolves the citation against the owner's stored, live, user-authored
// message, checks the BodyState revision, copies the excerpt itself and
// replaces the whole question with the canonical Go-written confirmation.
// Anything it cannot bind (foreign or assistant message, span mismatch,
// stale revision, unsupported kind, malformed or model-authored binding) is
// stripped to an ordinary question whose answer can write nothing.

// BodyStateSuggestionSourceReader resolves the citable message and the
// owner's current BodyState revision at bind time.
type BodyStateSuggestionSourceReader interface {
	ReadSuggestionSource(ctx context.Context, owner, conversationID, messageID uuid.UUID) (*model.SuggestionSourceMessage, int64, error)
}

type runtimeBodyStateSuggestion struct {
	Kind           string `json:"kind"`
	CitedMessageID string `json:"cited_message_id"`
	Span           struct {
		Start *int `json:"start"`
		End   *int `json:"end"`
	} `json:"span"`
	BaseBodyStateRevision *int64 `json:"base_body_state_revision"`
}

// BindBodyStateSuggestionQuestion returns the question Go persists for a
// runtime interrupt. bound reports whether it is the canonical Go binding.
// A non-nil error is an infrastructure failure (the runtime must fail
// closed); an unbindable suggestion is never an error, only stripped.
func BindBodyStateSuggestionQuestion(ctx context.Context, reader BodyStateSuggestionSourceReader, owner, conversationID uuid.UUID, question json.RawMessage) (json.RawMessage, bool, error) {
	var q map[string]any
	if err := json.Unmarshal(question, &q); err != nil || q == nil {
		return nil, false, fmt.Errorf("interaction question must be an object")
	}
	rawSuggestion, hasSuggestion := q["suggestion"]
	if q["purpose"] != model.BodyStateSuggestionPurpose && !hasSuggestion {
		return question, false, nil
	}
	strip := func() (json.RawMessage, bool, error) {
		delete(q, "suggestion")
		delete(q, "quote")
		if q["purpose"] == model.BodyStateSuggestionPurpose {
			delete(q, "purpose")
			delete(q, "state_binding")
		}
		stripped, err := json.Marshal(q)
		return stripped, false, err
	}
	if q["purpose"] != model.BodyStateSuggestionPurpose || reader == nil || owner == uuid.Nil || conversationID == uuid.Nil {
		return strip()
	}
	suggestion, ok := decodeRuntimeBodyStateSuggestion(rawSuggestion)
	if !ok {
		return strip()
	}
	messageID, err := uuid.Parse(suggestion.CitedMessageID)
	if err != nil || messageID == uuid.Nil || messageID.String() != suggestion.CitedMessageID {
		return strip()
	}
	source, current, err := reader.ReadSuggestionSource(ctx, owner, conversationID, messageID)
	if errors.Is(err, model.ErrBodyStateSuggestionRejected) {
		return strip()
	}
	if err != nil {
		return nil, false, fmt.Errorf("resolve suggestion source: %w", err)
	}
	if source == nil || source.ID != messageID || source.ConversationID != conversationID || *suggestion.BaseBodyStateRevision != current {
		return strip()
	}
	bound, err := model.BuildBodyStateSuggestionQuestion(*source, *suggestion.Span.Start, *suggestion.Span.End, current)
	if errors.Is(err, model.ErrBodyStateSuggestionRejected) {
		return strip()
	}
	if err != nil {
		return nil, false, err
	}
	return bound, true, nil
}

// decodeRuntimeBodyStateSuggestion accepts only the finite runtime envelope;
// unknown keys (an attempted receipt, excerpt or owner) reject it.
func decodeRuntimeBodyStateSuggestion(raw any) (runtimeBodyStateSuggestion, bool) {
	var suggestion runtimeBodyStateSuggestion
	encoded, err := json.Marshal(raw)
	if err != nil {
		return suggestion, false
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&suggestion) != nil || decoder.Decode(new(any)) != io.EOF {
		return suggestion, false
	}
	if suggestion.Kind != model.BodyStateSuggestionKindHypothesis || suggestion.Span.Start == nil || suggestion.Span.End == nil ||
		suggestion.BaseBodyStateRevision == nil || *suggestion.BaseBodyStateRevision < 0 {
		return suggestion, false
	}
	return suggestion, true
}

// IsBoundBodyStateSuggestion reports whether a persisted question declares
// the suggestion purpose, whose answer must not be accepted without a
// recorder (a forged declaration is refused by the recorder itself).
func IsBoundBodyStateSuggestion(question json.RawMessage) bool {
	_, matched, _ := model.ParseBoundBodyStateSuggestion(question)
	return matched
}

type bodyStateSuggestionLedger interface {
	ConfirmBodyHypothesisSuggestion(ctx context.Context, owner uuid.UUID, answer model.AnswerFactSource, command model.ConfirmBodyHypothesisSuggestionCommand) (*model.BodyHypothesisCommit, error)
}

// RecordBodyStateSuggestionAnswer commits the user's decision on a Go-bound
// suggestion inside the interaction-resume claim transaction. Confirmation
// commits exactly one user-authored BodyHypothesis whose statement Go
// re-reads from the cited message; decline writes nothing. Questions without
// the suggestion purpose record nothing; a forged binding is refused.
func (s *BodyStateService) RecordBodyStateSuggestionAnswer(ctx context.Context, userID uuid.UUID, source model.AnswerFactSource, interaction *model.AgentInteraction) (*model.BodyHypothesisCommit, error) {
	if interaction == nil {
		return nil, nil
	}
	command, matched, err := model.ParseBoundBodyStateSuggestion(interaction.Question)
	if !matched {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	confirmed, err := model.BodyStateSuggestionConfirmed(interaction.Answer)
	if err != nil || !confirmed {
		return nil, err
	}
	ledger, ok := s.repo.(bodyStateSuggestionLedger)
	if !ok {
		return nil, errors.New("body state suggestion ledger is not configured")
	}
	receipt, err := ledger.ConfirmBodyHypothesisSuggestion(ctx, userID, source, command)
	if err != nil {
		return nil, err
	}
	if receipt == nil || receipt.UserID != userID || receipt.ID == uuid.Nil || receipt.HypothesisID == uuid.Nil ||
		receipt.EventID == uuid.Nil || receipt.RevisionID == uuid.Nil || receipt.Revision <= 0 {
		return nil, errors.New("body state suggestion receipt is missing or does not match its owner")
	}
	return receipt, nil
}
