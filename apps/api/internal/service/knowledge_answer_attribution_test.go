package service

import (
	"encoding/json"
	"testing"
)

func validAnswerAttributionPayload() json.RawMessage {
	return json.RawMessage(`{
		"attribution":{
			"attribution_id":"tool-1:0",
			"policy_revision":"consultation-answer-attribution-v1",
			"claim_text":"疼痛与伤害感受不是同一现象",
			"evidence_refs":["published:11111111-1111-1111-1111-111111111111:v3:tfu-example"],
			"grounding_status":"supported",
			"reason_codes":["lexical_support_sufficient"],
			"bindings":[{
				"evidence_ref":"published:11111111-1111-1111-1111-111111111111:v3:tfu-example",
				"publication_id":"11111111-1111-1111-1111-111111111111",
				"publication_key":"pain-definition-v3",
				"publication_batch_key":"thought-forest-reviewed-health-pilot",
				"published_version":3,
				"unit_key":"tfu-example",
				"claim_id":"tfc-example",
				"claim_review_id":"claim-review-example",
				"claim_kind":"definition",
				"grounding_status":"supported",
				"reason_codes":["lexical_support_sufficient"],
				"source_locator":{
					"locator_type":"markdown_lines",
					"repository":"thought-forest",
					"git_commit":"abc123",
					"path":"z/pain-and-nociception.md",
					"line_start":20,
					"line_end":23
				}
			}]
		}
	}`)
}

func TestParseConsultationAnswerAttributionPayloadAcceptsExactPublishedBinding(t *testing.T) {
	payload, err := ParseConsultationAnswerAttributionPayload(validAnswerAttributionPayload())
	if err != nil {
		t.Fatalf("ParseConsultationAnswerAttributionPayload: %v", err)
	}
	if payload.Attribution.PolicyRevision != ConsultationAnswerAttributionPolicyV1 {
		t.Fatalf("unexpected policy: %s", payload.Attribution.PolicyRevision)
	}
	if got := payload.Attribution.Bindings[0].PublishedVersion; got != 3 {
		t.Fatalf("published version = %d, want 3", got)
	}
}

func TestParseConsultationAnswerAttributionPayloadRejectsUnknownOrDriftingBinding(t *testing.T) {
	var payload map[string]any
	if err := json.Unmarshal(validAnswerAttributionPayload(), &payload); err != nil {
		t.Fatal(err)
	}
	attribution := payload["attribution"].(map[string]any)
	bindings := attribution["bindings"].([]any)
	binding := bindings[0].(map[string]any)
	binding["evidence_ref"] = "published:different"
	raw, _ := json.Marshal(payload)
	if _, err := ParseConsultationAnswerAttributionPayload(raw); err == nil {
		t.Fatal("expected undeclared binding evidence_ref to be rejected")
	}

	binding["evidence_ref"] = attribution["evidence_refs"].([]any)[0]
	binding["source_locator"].(map[string]any)["git_commit"] = ""
	raw, _ = json.Marshal(payload)
	if _, err := ParseConsultationAnswerAttributionPayload(raw); err == nil {
		t.Fatal("expected incomplete source locator to be rejected")
	}
}
