package repository

import (
	"encoding/json"
	"strings"
)

// episodicSourceText is the single canonical source-text rule shared by the
// S2 distillation planner and the shadow-note writer. Runtime messages are
// persisted in more than one shape: a user message usually keeps its text
// only in parts ([{"text":"…","type":"text"}]) with an empty content_text,
// while an assistant reply keeps its body in content_text and may carry
// quote/image parts beside it. Neither repository may re-derive its own
// interpretation of that row.
//
// Canonical text is:
//   - content_text when it is non-empty, otherwise
//   - every part with type "text", concatenated in array order.
//
// Pure text means parts is [] or every part has type "text". A message that
// contains any other part type is not pure text, but its canonical text is
// still usable when content_text carries the body.
//
// ok is false for a malformed parts value (bad JSON, null, a non-object
// element, or a missing/empty type). Such a row is never guessed at and never
// panics; callers treat it as an unusable source with a stable reason.
func episodicSourceText(parts, contentText string) (text string, pureText bool, ok bool) {
	var decoded []json.RawMessage
	if err := json.Unmarshal([]byte(parts), &decoded); err != nil || decoded == nil {
		return "", false, false
	}
	var textParts strings.Builder
	pureText = true
	for _, raw := range decoded {
		var part struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(raw, &part); err != nil || part.Type == "" {
			return "", false, false
		}
		if part.Type != "text" {
			pureText = false
			continue
		}
		textParts.WriteString(part.Text)
	}
	if contentText != "" {
		return contentText, pureText, true
	}
	return textParts.String(), pureText, true
}
