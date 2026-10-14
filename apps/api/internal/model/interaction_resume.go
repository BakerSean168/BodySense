package model

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

var (
	ErrInvalidInteractionAnswer     = errors.New("invalid interaction answer")
	ErrInteractionResumeConflict    = errors.New("interaction resume conflict")
	ErrInteractionResumeUnavailable = errors.New("interaction resume unavailable")
	ErrInteractionResumeNotFound    = errors.New("interaction resume not found")
	ErrInteractionResumeExpired     = errors.New("interaction resume expired")
)

// InteractionResume is a metadata-only receipt of one accepted Ask User answer.
// Its identities are assigned by Go; it is not a second execution or a health
// commit receipt. Answer content remains owned by AgentInteraction/Message.
type InteractionResume struct {
	InteractionID       uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID              uuid.UUID `gorm:"type:uuid;not null"`
	ConversationID      uuid.UUID `gorm:"type:uuid;not null"`
	RunID               uuid.UUID `gorm:"type:uuid;not null"`
	RequestID           string    `gorm:"not null"`
	AnswerFingerprint   string    `gorm:"not null"`
	QuestionFingerprint string    `gorm:"not null"`
	UserMessageID       uuid.UUID `gorm:"type:uuid;not null"`
	AssistantMessageID  uuid.UUID `gorm:"type:uuid;not null"`
	ClaimedAt           time.Time `gorm:"not null"`
}

func (InteractionResume) TableName() string { return "interaction_resumes" }

type ClaimInteractionResume struct {
	UserID         uuid.UUID
	ConversationID uuid.UUID
	InteractionID  uuid.UUID
	RequestID      string
	Answer         datatypes.JSON
}

type InteractionResumeEnvelope struct {
	Receipt          *InteractionResume
	Interaction      *AgentInteraction
	Run              *Run
	UserMessage      *Message
	AssistantMessage *Message
	Existed          bool
}

type AnswerFactSource struct {
	RunID         uuid.UUID
	InteractionID uuid.UUID
	MessageID     uuid.UUID
}

type answerField struct {
	Key              string   `json:"key"`
	Label            string   `json:"label"`
	AnswerType       string   `json:"answer_type"`
	Options          []string `json:"options"`
	ExclusiveOptions []string `json:"exclusive_options"`
	Required         *bool    `json:"required"`
}

type answerQuestion struct {
	Question    string        `json:"question"`
	AnswerType  string        `json:"answer_type"`
	Options     []string      `json:"options"`
	AllowCustom bool          `json:"allow_custom_input"`
	Fields      []answerField `json:"fields"`
}

type acceptedAnswer struct {
	Text     string                     `json:"text"`
	Selected []string                   `json:"selected,omitempty"`
	IsCustom bool                       `json:"is_custom,omitempty"`
	Value    *float64                   `json:"value,omitempty"`
	Fields   map[string]json.RawMessage `json:"fields,omitempty"`
}

// ValidateInteractionAnswer binds values to the exact durable question. A
// client-supplied text summary cannot smuggle a different value into extraction.
func ValidateInteractionAnswer(question, raw []byte) (datatypes.JSON, string, error) {
	invalid := func() (datatypes.JSON, string, error) { return nil, "", ErrInvalidInteractionAnswer }
	if len(raw) == 0 || len(raw) > 32768 || len(question) == 0 || len(question) > 65536 {
		return invalid()
	}
	var q answerQuestion
	if json.Unmarshal(question, &q) != nil || strings.TrimSpace(q.Question) == "" {
		return invalid()
	}
	var a acceptedAnswer
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&a) != nil || d.Decode(new(any)) != io.EOF {
		return invalid()
	}
	a.Text = strings.TrimSpace(a.Text)
	if len(a.Text) > 12000 {
		return invalid()
	}
	if len(q.Fields) > 0 {
		if len(q.Fields) > 20 || len(a.Fields) == 0 || a.Selected != nil || a.Value != nil || a.IsCustom {
			return invalid()
		}
		known := make(map[string]bool)
		parts := make([]string, 0, len(q.Fields))
		for _, field := range q.Fields {
			if field.Key == "" || field.Label == "" || known[field.Key] {
				return invalid()
			}
			known[field.Key] = true
			value, exists := a.Fields[field.Key]
			if !exists {
				if field.Required == nil || *field.Required {
					return invalid()
				}
				continue
			}
			var text string
			if field.AnswerType == "multi_choice" {
				var selected []string
				if json.Unmarshal(value, &selected) != nil || !validAnswerChoices(selected, field.Options, false) {
					return invalid()
				}
				if len(selected) > 1 {
					for _, choice := range selected {
						if containsAnswerChoice(field.ExclusiveOptions, choice) {
							return invalid()
						}
					}
				}
				text = strings.Join(selected, "，")
			} else {
				if json.Unmarshal(value, &text) != nil || text != strings.TrimSpace(text) || !validAnswerScalar(text, field.AnswerType, field.Options) {
					return invalid()
				}
			}
			parts = append(parts, field.Label+": "+text)
		}
		for key := range a.Fields {
			if !known[key] {
				return invalid()
			}
		}
		canonicalText := strings.Join(parts, "；")
		if canonicalText == "" || (a.Text != "" && a.Text != canonicalText) {
			return invalid()
		}
		a.Text = canonicalText
	} else {
		if a.Fields != nil {
			return invalid()
		}
		switch q.AnswerType {
		case "single_choice", "multi_choice":
			if a.Value != nil || (a.IsCustom && !q.AllowCustom) || !validAnswerChoices(a.Selected, q.Options, a.IsCustom && q.AllowCustom) {
				return invalid()
			}
			if q.AnswerType == "single_choice" && len(a.Selected) != 1 {
				return invalid()
			}
			canonicalText := strings.Join(a.Selected, "，")
			if a.Text != "" && a.Text != canonicalText {
				return invalid()
			}
			a.Text = canonicalText
		case "text", "number", "date":
			if a.Selected != nil || a.IsCustom || !validAnswerScalar(a.Text, q.AnswerType, nil) {
				return invalid()
			}
			if a.Value != nil {
				v, err := strconv.ParseFloat(a.Text, 64)
				if q.AnswerType != "number" || err != nil || *a.Value != v {
					return invalid()
				}
			}
		default:
			return invalid()
		}
	}
	encoded, err := json.Marshal(a)
	if err != nil {
		return invalid()
	}
	return datatypes.JSON(encoded), a.Text, nil
}

func validAnswerScalar(text, kind string, choices []string) bool {
	if strings.TrimSpace(text) == "" || len(text) > 12000 {
		return false
	}
	switch kind {
	case "text":
		return true
	case "single_choice":
		return containsAnswerChoice(choices, text)
	case "number", "scale":
		n, err := strconv.ParseFloat(text, 64)
		return err == nil && !math.IsInf(n, 0) && !math.IsNaN(n)
	case "date":
		_, err := time.Parse("2006-01-02", text)
		return err == nil
	default:
		return false
	}
}

func validAnswerChoices(values, choices []string, custom bool) bool {
	if len(values) == 0 || len(values) > 20 {
		return false
	}
	seen := make(map[string]bool)
	for _, value := range values {
		if value == "" || value != strings.TrimSpace(value) || len(value) > 12000 || seen[value] || (!custom && !containsAnswerChoice(choices, value)) {
			return false
		}
		seen[value] = true
	}
	return true
}

func containsAnswerChoice(choices []string, value string) bool {
	for _, choice := range choices {
		if choice == value {
			return true
		}
	}
	return false
}

// JSONFingerprint ignores object-key order, not payload content or number precision.
func InteractionJSONFingerprint(raw []byte) (string, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value any
	if err := d.Decode(&value); err != nil {
		return "", err
	}
	if d.Decode(new(any)) != io.EOF {
		return "", fmt.Errorf("trailing JSON")
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}
