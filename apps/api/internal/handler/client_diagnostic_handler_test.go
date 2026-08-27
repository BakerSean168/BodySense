package handler

import (
	"testing"
)

func TestSanitizeDiagnosticAttributesAcceptsFlatPrimitiveAttributes(t *testing.T) {
	attrs, ok := sanitizeDiagnosticAttributes(map[string]any{
		"viewer.progress_pct": float64(75),
		"resource.cache_hit":  true,
		"network.protocol":    "h2",
	})
	if !ok || len(attrs) != 3 {
		t.Fatalf("unexpected attributes: ok=%v attrs=%#v", ok, attrs)
	}
}

func TestSanitizeDiagnosticAttributesRejectsNestedValues(t *testing.T) {
	if _, ok := sanitizeDiagnosticAttributes(map[string]any{
		"nested": map[string]any{"secret": "value"},
	}); ok {
		t.Fatal("nested diagnostic attributes must be rejected")
	}
}

func TestSanitizeDiagnosticResourceDropsQueryAndFragment(t *testing.T) {
	got := sanitizeDiagnosticResource("https://example.test/model.glb?token=secret#mesh")
	if got != "/model.glb" {
		t.Fatalf("resource=%q", got)
	}
}
