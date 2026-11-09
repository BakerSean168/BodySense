package repository

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestEpisodicSourceTextNormalizesRuntimeShapes(t *testing.T) {
	cases := []struct {
		name    string
		parts   string
		content string
		text    string
		pure    bool
		ok      bool
	}{
		{name: "ideal empty parts and content", parts: `[]`, content: "hello", text: "hello", pure: true, ok: true},
		{name: "user text part only", parts: `[{"text":"hello","type":"text"}]`, text: "hello", pure: true, ok: true},
		{name: "content wins over text parts", parts: `[{"text":"hello","type":"text"}]`, content: "body", text: "body", pure: true, ok: true},
		{name: "content wins beside a quote part", parts: `[{"type":"quote","message_id":"x"}]`, content: "reply", text: "reply", pure: false, ok: true},
		{name: "text parts concatenate in order", parts: `[{"type":"text","text":"a"},{"type":"text","text":"b"}]`, text: "ab", pure: true, ok: true},
		{name: "assistant quote with trailing text part", parts: `[{"type":"quote","message_id":"x"},{"type":"text","text":"ignored"}]`, content: "reply", text: "reply", pure: false, ok: true},
		{name: "user image part is not pure", parts: `[{"type":"image","upload_id":"x"}]`, content: "caption", text: "caption", pure: false, ok: true},
		{name: "empty parts and empty content", parts: `[]`, text: "", pure: true, ok: true},
		{name: "malformed json", parts: `{`, content: "hello", ok: false},
		{name: "json null", parts: `null`, content: "hello", ok: false},
		{name: "non object element", parts: `["x"]`, content: "hello", ok: false},
		{name: "missing type", parts: `[{"text":"x"}]`, ok: false},
		{name: "empty type", parts: `[{"type":"","text":"x"}]`, ok: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text, pure, ok := episodicSourceText(tc.parts, tc.content)
			if text != tc.text || pure != tc.pure || ok != tc.ok {
				t.Fatalf("text=%q pure=%v ok=%v, want %q %v %v", text, pure, ok, tc.text, tc.pure, tc.ok)
			}
		})
	}
}

func TestEvaluateDistillationGroupAcceptsRuntimeSourceShape(t *testing.T) {
	owner := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	conversation := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	completed := "completed"
	row := func(role string, seq int) episodicDistillationRow {
		return episodicDistillationRow{
			ID: uuid.New(), TurnID: uuid.New(), Seq: seq, Role: role, Status: "completed",
			RunStatus: &completed, RunUserID: &owner, RunConversationID: &conversation,
			TurnSize: 2,
		}
	}
	runtimeTurn := func() []episodicDistillationRow {
		user := row("user", 3)
		// Runtime user shape: body only in a text part, content_text empty.
		user.Parts = `[{"text":"请简短回答。","type":"text"}]`
		assistant := row("assistant", 4)
		// Runtime assistant shape: body in content_text, quotes in parts.
		assistant.ContentText = "好的，我会简短回答。"
		assistant.Parts = `[{"type":"quote","message_id":"x"},{"type":"text","text":"请简短回答。"}]`
		return []episodicDistillationRow{user, assistant}
	}
	if eligible, reason := evaluateDistillationGroup(runtimeTurn(), owner, conversation, nil); !eligible || reason != "" {
		t.Fatalf("runtime shape rejected: eligible=%v reason=%q", eligible, reason)
	}
	ideal := row("user", 3)
	ideal.Parts = `[]`
	ideal.ContentText = "请简短回答。"
	idealReply := row("assistant", 4)
	idealReply.Parts = `[]`
	idealReply.ContentText = "好的。"
	if eligible, reason := evaluateDistillationGroup([]episodicDistillationRow{ideal, idealReply}, owner, conversation, nil); !eligible || reason != "" {
		t.Fatalf("ideal shape regressed: eligible=%v reason=%q", eligible, reason)
	}

	cases := []struct {
		name   string
		mutate func([]episodicDistillationRow) []episodicDistillationRow
		reason string
	}{
		{
			name: "user non text part",
			mutate: func(rows []episodicDistillationRow) []episodicDistillationRow {
				rows[0].Parts = `[{"type":"image","upload_id":"x"}]`
				rows[0].ContentText = ""
				return rows
			},
			reason: "multimodal_source",
		},
		{
			name: "assistant only non text part and no body",
			mutate: func(rows []episodicDistillationRow) []episodicDistillationRow {
				rows[1].Parts = `[{"type":"quote","message_id":"x"}]`
				rows[1].ContentText = ""
				return rows
			},
			reason: "empty_source",
		},
		{
			name: "malformed parts",
			mutate: func(rows []episodicDistillationRow) []episodicDistillationRow {
				rows[0].Parts = `{"text":"broken"}`
				return rows
			},
			reason: "invalid_source",
		},
		{
			name: "padded canonical text",
			mutate: func(rows []episodicDistillationRow) []episodicDistillationRow {
				rows[0].Parts = `[{"type":"text","text":" padded "}]`
				return rows
			},
			reason: "padded_source",
		},
		{
			name: "empty canonical text",
			mutate: func(rows []episodicDistillationRow) []episodicDistillationRow {
				rows[0].Parts = `[]`
				return rows
			},
			reason: "empty_source",
		},
		{
			name: "oversized canonical text",
			mutate: func(rows []episodicDistillationRow) []episodicDistillationRow {
				rows[0].Parts = `[{"type":"text","text":"` + strings.Repeat("健", EpisodicShadowSourceMaxRunes+1) + `"}]`
				return rows
			},
			reason: "oversized_source",
		},
		{
			name: "clinical reference",
			mutate: func(rows []episodicDistillationRow) []episodicDistillationRow {
				rows[0].ClinicalRefs = 1
				return rows
			},
			reason: "clinical_source",
		},
		{
			name: "unsettled source",
			mutate: func(rows []episodicDistillationRow) []episodicDistillationRow {
				rows[1].Status = "streaming"
				return rows
			},
			reason: "unsettled_source",
		},
		{
			name: "foreign run",
			mutate: func(rows []episodicDistillationRow) []episodicDistillationRow {
				other := uuid.New()
				rows[0].RunUserID = &other
				return rows
			},
			reason: "foreign_source",
		},
		{
			name: "unsupported role pair",
			mutate: func(rows []episodicDistillationRow) []episodicDistillationRow {
				rows[1].Role = "user"
				rows[1].Parts = `[]`
				rows[1].ContentText = "reply"
				return rows
			},
			reason: "unsupported_turn_shape",
		},
		{
			name: "three messages",
			mutate: func(rows []episodicDistillationRow) []episodicDistillationRow {
				return append(rows, row("assistant", 5))
			},
			reason: "unsupported_turn_shape",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			eligible, reason := evaluateDistillationGroup(tc.mutate(runtimeTurn()), owner, conversation, nil)
			if eligible || reason != tc.reason {
				t.Fatalf("eligible=%v reason=%q, want false %q", eligible, reason, tc.reason)
			}
		})
	}

	tombstones := []seqRange{{from: 3, to: 4}}
	if eligible, reason := evaluateDistillationGroup(runtimeTurn(), owner, conversation, tombstones); eligible || reason != "user_deleted" {
		t.Fatalf("tombstone: eligible=%v reason=%q", eligible, reason)
	}
}
