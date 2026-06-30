package context

import (
	"testing"

	"github.com/bodysense/api/internal/dto"
)

func TestExtractTextFromParts(t *testing.T) {
	tests := []struct {
		name  string
		parts []dto.PartDTO
		want  string
	}{
		{
			name:  "empty parts",
			parts: []dto.PartDTO{},
			want:  "",
		},
		{
			name: "single text part",
			parts: []dto.PartDTO{
				{Type: "text", Text: "hello world"},
			},
			want: "hello world",
		},
		{
			name: "multiple parts, returns first text",
			parts: []dto.PartDTO{
				{Type: "image", Text: ""},
				{Type: "text", Text: "first text"},
				{Type: "text", Text: "second text"},
			},
			want: "first text",
		},
		{
			name: "no text parts",
			parts: []dto.PartDTO{
				{Type: "image", Text: ""},
				{Type: "file", Text: ""},
			},
			want: "",
		},
		{
			name: "text part with empty text is skipped",
			parts: []dto.PartDTO{
				{Type: "text", Text: ""},
				{Type: "text", Text: "actual text"},
			},
			want: "actual text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractTextFromParts(tt.parts)
			if got != tt.want {
				t.Errorf("ExtractTextFromParts() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildChatContextInput_Fields(t *testing.T) {
	// Verify the input struct has the expected fields
	input := BuildChatContextInput{
		IsDraft: true,
	}
	if !input.IsDraft {
		t.Error("expected IsDraft to be true")
	}
}

func TestContextTrace_Fields(t *testing.T) {
	trace := &ContextTrace{
		ExcludedCurrentTurn:  true,
		ProfileIncluded:      true,
		ConsultationIncluded: false,
		UseCase:              "consultation.reply",
	}
	if !trace.ExcludedCurrentTurn {
		t.Error("expected ExcludedCurrentTurn to be true")
	}
	if !trace.ProfileIncluded {
		t.Error("expected ProfileIncluded to be true")
	}
	if trace.ConsultationIncluded {
		t.Error("expected ConsultationIncluded to be false")
	}
	if trace.UseCase != "consultation.reply" {
		t.Errorf("expected UseCase to be 'consultation.reply', got %q", trace.UseCase)
	}
}
