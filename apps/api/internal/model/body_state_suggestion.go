package model

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
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
	BodyStateSuggestionKindObservation = "observation"
	BodyStateSuggestionKindEpisode     = "episode"
	BodyStateSuggestionConfirmOption   = "记录为我的猜测"
	BodyStateSuggestionRejectOption    = "不记录"
	maxBodyStateSuggestionExcerptRunes = 2000
)

// bodyStateSuggestionKind is the fixed Go wording of one suggestion kind:
// the Ask User prompt and the single confirming option. The model's own
// wording is never shown as the question.
type bodyStateSuggestionKind struct {
	Question      string
	ConfirmOption string
}

var bodyStateSuggestionKinds = map[string]bodyStateSuggestionKind{
	BodyStateSuggestionKindHypothesis:  {BodyStateSuggestionQuestionText, BodyStateSuggestionConfirmOption},
	BodyStateSuggestionKindObservation: {"要把你说过的这句话记录为一条身体观察吗？", "记录为身体观察"},
	BodyStateSuggestionKindEpisode:     {"要把你描述的这次症状记录为一次新的发作吗？", "记录为一次发作"},
}

// BodyStateSuggestionKindSupported reports whether Go can bind and commit kind.
func BodyStateSuggestionKindSupported(kind string) bool {
	_, ok := bodyStateSuggestionKinds[kind]
	return ok
}

// BodyStateSuggestionConfirmOptionFor is the confirming option of kind.
func BodyStateSuggestionConfirmOptionFor(kind string) string {
	return bodyStateSuggestionKinds[kind].ConfirmOption
}

// BodyStateSuggestionPayload carries the kind-specific labels of a
// suggestion. Every user-facing label is a verbatim substring of the cited
// excerpt (Go checks it), so no model wording is ever committed:
//   - hypothesis: empty;
//   - observation: an organizational observation_kind label (never a
//     clinical interpretation) plus an optional raw_value/raw_unit copied
//     from the user's words;
//   - episode: the symptom_type words, plus the fact_id of the symptom fact
//     Go itself resolved from the same cited span (never runtime supplied).
type BodyStateSuggestionPayload struct {
	ObservationKind string `json:"observation_kind,omitempty"`
	RawValue        string `json:"raw_value,omitempty"`
	RawUnit         string `json:"raw_unit,omitempty"`
	SymptomType     string `json:"symptom_type,omitempty"`
	FactID          string `json:"fact_id,omitempty"`
}

var suggestionObservationKind = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)

// ValidateBodyStateSuggestionPayload checks payload against kind and the
// exact excerpt it was bound to. resolved requires the Go-resolved episode
// fact; a runtime envelope (resolved=false) must not carry one.
func ValidateBodyStateSuggestionPayload(kind string, payload BodyStateSuggestionPayload, excerpt string, resolved bool) error {
	reject := func(reason string) error { return fmt.Errorf("%w: %s", ErrBodyStateSuggestionRejected, reason) }
	verbatim := func(value string, maxRunes int) bool {
		return value != "" && value == strings.TrimSpace(value) && utf8.RuneCountInString(value) <= maxRunes && strings.Contains(excerpt, value)
	}
	switch kind {
	case BodyStateSuggestionKindHypothesis:
		if payload != (BodyStateSuggestionPayload{}) {
			return reject("a hypothesis carries no payload")
		}
	case BodyStateSuggestionKindObservation:
		if !suggestionObservationKind.MatchString(payload.ObservationKind) || payload.SymptomType != "" || payload.FactID != "" {
			return reject("an observation needs only a bounded observation label")
		}
		if (payload.RawValue != "" && !verbatim(payload.RawValue, 120)) || (payload.RawUnit != "" && !verbatim(payload.RawUnit, 48)) ||
			(payload.RawUnit != "" && payload.RawValue == "") {
			return reject("observation raw value and unit must be the user's own words")
		}
	case BodyStateSuggestionKindEpisode:
		if payload.ObservationKind != "" || payload.RawValue != "" || payload.RawUnit != "" || !verbatim(payload.SymptomType, 120) {
			return reject("an episode needs the user's own symptom words")
		}
		if resolved {
			id, err := uuid.Parse(payload.FactID)
			if err != nil || id == uuid.Nil || id.String() != payload.FactID {
				return reject("an episode must reference the Go-resolved symptom fact")
			}
		} else if payload.FactID != "" {
			return reject("the runtime cannot name the episode fact")
		}
	default:
		return reject("unsupported suggestion kind")
	}
	return nil
}

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

// ConfirmBodyStateSuggestionCommand is what Go commits after the user
// confirmed a bound suggestion. It carries no statement text: the statement
// is re-read from the cited message inside the commit transaction.
type ConfirmBodyStateSuggestionCommand struct {
	Kind                  string                     `json:"kind"`
	Source                BodyStateSuggestionSource  `json:"source"`
	BaseBodyStateRevision int64                      `json:"base_body_state_revision"`
	Payload               BodyStateSuggestionPayload `json:"payload"`
}

func (c ConfirmBodyStateSuggestionCommand) Validate() error {
	s := c.Source
	if !BodyStateSuggestionKindSupported(c.Kind) || s.MessageID == uuid.Nil || s.MessageSeq < 1 || s.SpanStart < 0 ||
		s.SpanEnd <= s.SpanStart || !validSuggestionDigest(s.ExcerptSHA256) || c.BaseBodyStateRevision < 0 {
		return ErrBodyStateSuggestionRejected
	}
	return nil
}

// BodyStateSuggestionReceipt is the id-only commit receipt of a confirmed
// suggestion. RecordID is the hypothesis, observation or episode id;
// SourceEvidenceID is the new statement evidence (hypothesis, observation)
// and FactID the organized symptom fact (episode).
type BodyStateSuggestionReceipt struct {
	Kind             string
	UserID           uuid.UUID
	CommitID         uuid.UUID
	RecordID         uuid.UUID
	SourceEvidenceID uuid.UUID
	FactID           uuid.UUID
	Revision         int64
	RevisionID       uuid.UUID
	EventID          uuid.UUID
	CommittedAt      time.Time
}

// Public is the id-only public receipt (stream milestone and interaction
// history); nil when the user declined and nothing was written.
func (r *BodyStateSuggestionReceipt) Public() map[string]any {
	if r == nil {
		return nil
	}
	receipt := map[string]any{"kind": r.Kind, "commit_id": r.CommitID.String(),
		"revision": r.Revision, "event_id": r.EventID.String(), "committed_at": r.CommittedAt.UTC().Format(time.RFC3339Nano)}
	switch r.Kind {
	case BodyStateSuggestionKindHypothesis:
		receipt["hypothesis_id"] = r.RecordID.String()
		receipt["source_evidence_id"] = r.SourceEvidenceID.String()
	case BodyStateSuggestionKindObservation:
		receipt["observation_id"] = r.RecordID.String()
		receipt["source_evidence_id"] = r.SourceEvidenceID.String()
	case BodyStateSuggestionKindEpisode:
		receipt["episode_id"] = r.RecordID.String()
		receipt["fact_id"] = r.FactID.String()
	}
	return receipt
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
