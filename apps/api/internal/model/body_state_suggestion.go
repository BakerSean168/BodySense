package model

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Source-bound BodyState suggestion contract (BS-AGENT-001).
//
// The Agent may only *suggest* a BodyState item by citing a span of the
// user's own chat message. Go resolves the cited message, copies the excerpt
// from the stored user text (never from the model), and persists the
// canonical binding into the Ask User question. Only the user's explicit
// confirmation of that bound question lets Go commit a user-authored record
// whose statement evidence points at the cited message. Rejection, expiry,
// a stale BodyState revision or a vanished source write nothing.
const (
	BodyStateSuggestionPurpose         = "body_state_suggestion"
	BodyStateSuggestionBindingRevision = "body-state-suggestion-v1"
	BodyStateSuggestionKindHypothesis  = "hypothesis"
	BodyStateSuggestionConfirmOption   = "记录为我的猜测"
	BodyStateSuggestionRejectOption    = "不记录"
	maxBodyStateSuggestionExcerptRunes = 2000
)

var (
	// ErrBodyStateSuggestionRejected means the suggestion cannot be bound to
	// an exact span of the owner's own live user message.
	ErrBodyStateSuggestionRejected = errors.New("body state suggestion rejected")
	// ErrBodyStateSuggestionStale means the BodyState revision the suggestion
	// was bound against is no longer current; nothing is written.
	ErrBodyStateSuggestionStale = errors.New("body state suggestion is stale")
)

// BodyStateSuggestionSource is the Go-written citation: exact user message
// identity and position plus a digest of the excerpt Go copied from it.
type BodyStateSuggestionSource struct {
	MessageID     uuid.UUID `json:"message_id"`
	MessageSeq    int       `json:"message_seq"`
	SpanStart     int       `json:"span_start"`
	SpanEnd       int       `json:"span_end"`
	ExcerptSHA256 string    `json:"excerpt_sha256"`
}

// ConfirmBodyHypothesisSuggestionCommand is what Go commits after the user
// confirmed a bound hypothesis suggestion. It carries no statement text: the
// statement is re-read from the cited message inside the commit transaction.
type ConfirmBodyHypothesisSuggestionCommand struct {
	Source                BodyStateSuggestionSource `json:"source"`
	BaseBodyStateRevision int64                     `json:"base_body_state_revision"`
}

func (c ConfirmBodyHypothesisSuggestionCommand) Validate() error {
	s := c.Source
	if s.MessageID == uuid.Nil || s.MessageSeq < 1 || s.SpanStart < 0 || s.SpanEnd <= s.SpanStart ||
		!validSuggestionDigest(s.ExcerptSHA256) || c.BaseBodyStateRevision < 0 {
		return ErrBodyStateSuggestionRejected
	}
	return nil
}

// SuggestionSourceMessage is one live, owner-scoped, user-authored chat
// message a suggestion may cite, together with the Run of its turn.
type SuggestionSourceMessage struct {
	ID             uuid.UUID
	ConversationID uuid.UUID
	RunID          uuid.UUID
	Seq            int
	Text           string
}

// BodyStateSuggestionExcerpt returns the exact excerpt [start,end) (rune
// offsets) of a user message. A span that is out of range, empty, padded
// with whitespace or unbounded is rejected rather than trimmed, so the
// committed statement is byte-identical to what the user wrote.
func BodyStateSuggestionExcerpt(text string, start, end int) (string, bool) {
	if start < 0 || end <= start || end-start > maxBodyStateSuggestionExcerptRunes || !utf8.ValidString(text) {
		return "", false
	}
	runes := []rune(text)
	if end > len(runes) {
		return "", false
	}
	excerpt := string(runes[start:end])
	if excerpt != strings.TrimSpace(excerpt) {
		return "", false
	}
	return excerpt, true
}

func BodyStateSuggestionDigest(excerpt string) string {
	sum := sha256.Sum256([]byte(excerpt))
	return hex.EncodeToString(sum[:])
}

func validSuggestionDigest(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
