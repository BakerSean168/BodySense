package chat

import (
	"encoding/json"
	"testing"
)

func TestAssistantPartsBuilderMergesToolResultsIntoExistingToolCall(t *testing.T) {
	builder := newAssistantPartsBuilder()

	builder.AddToolCall("tc-1", "search_knowledge", json.RawMessage(`{"query":"头前伸自测"}`))
	builder.AddToolResult("tc-1", "search_knowledge", json.RawMessage(`{"found":2}`))

	parts := builder.Parts()
	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	if got := parts[0]["type"]; got != "tool-call" {
		t.Fatalf("part type = %v, want tool-call", got)
	}
	if got := parts[0]["result"]; got == nil {
		t.Fatal("expected merged tool result on the tool-call part")
	}
}

func TestAssistantPartsBuilderStoresCitationProviderMetadata(t *testing.T) {
	builder := newAssistantPartsBuilder()

	builder.AddCitation(json.RawMessage(`{
		"citation": {
			"title": "头前伸自测方法",
			"url": "https://example.com/guide",
			"summary": "判断耳垂与肩峰的关系",
			"source_title": "姿势评估",
			"source_author": "BodySense"
		}
	}`))

	parts := builder.Parts()
	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	if got := parts[0]["type"]; got != "source" {
		t.Fatalf("part type = %v, want source", got)
	}
	metadata, ok := parts[0]["providerMetadata"].(map[string]any)
	if !ok {
		t.Fatal("expected providerMetadata to be present")
	}
	bodysense, ok := metadata["bodysense"].(map[string]any)
	if !ok {
		t.Fatal("expected bodysense provider metadata")
	}
	if got := bodysense["summary"]; got != "判断耳垂与肩峰的关系" {
		t.Fatalf("summary = %v", got)
	}
}

func TestAssistantPartsBuilderStoresKnowledgeGapAsDataPart(t *testing.T) {
	builder := newAssistantPartsBuilder()

	builder.AddKnowledgeGap(json.RawMessage(`{"query":"斜方肌紧张","message":"暂无专项资料"}`))

	parts := builder.Parts()
	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	if got := parts[0]["type"]; got != "data" {
		t.Fatalf("part type = %v, want data", got)
	}
	if got := parts[0]["name"]; got != "knowledge_gap" {
		t.Fatalf("data part name = %v, want knowledge_gap", got)
	}
}
