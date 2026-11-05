package repository

// EpisodicPreference is the *only* S2-B content admitted from a
// source-quoted proposal. It is a fixed, non-clinical display preference;
// never arbitrary medical narrative or a model-authored summary.
type EpisodicPreference struct {
	Kind string
	Text string
}

// Exact equality intentionally fails closed for any appended health text,
// prompt injection, clinical claims or unapproved synonyms. Python has an
// independent checker and fixtures test their cross-language parity.
// Do not expand this list without product/privacy review.
var episodicPreferenceQuotes = map[string]EpisodicPreference{
	"请简短回答。":  {Kind: "brief_answers", Text: "对话偏好：回答简短。"},
	"请分点回答。":  {Kind: "bullet_answers", Text: "对话偏好：分点列出。"},
	"请用中文回答。": {Kind: "chinese_answers", Text: "对话偏好：使用中文。"},
	"请详细解释。":  {Kind: "detailed_answers", Text: "对话偏好：解释详细。"},
}

// CanonicalEpisodicPreference never returns model-authored words. This
// function alone cannot grant saving: DB ownership/provenance, erasure,
// source-role and cursor checks still apply under the Conversation lock.
func CanonicalEpisodicPreference(quote string) (EpisodicPreference, bool) {
	p, ok := episodicPreferenceQuotes[quote]
	return p, ok
}
