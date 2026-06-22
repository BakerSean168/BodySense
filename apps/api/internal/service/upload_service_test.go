package service

import (
	"testing"
)

func TestAllowedMimeTypes(t *testing.T) {
	tests := []struct {
		mimeType string
		expected bool
	}{
		{"image/jpeg", true},
		{"image/png", true},
		{"image/webp", true},
		{"application/pdf", true},
		{"image/gif", false},
		{"text/plain", false},
		{"application/msword", false},
	}

	for _, tt := range tests {
		t.Run(tt.mimeType, func(t *testing.T) {
			result := allowedMimeTypes[tt.mimeType]
			if result != tt.expected {
				t.Errorf("allowedMimeTypes[%q] = %v, want %v", tt.mimeType, result, tt.expected)
			}
		})
	}
}

func TestAllowedFileTypes(t *testing.T) {
	tests := []struct {
		fileType string
		expected bool
	}{
		{"photo_front", true},
		{"photo_side", true},
		{"photo_back", true},
		{"report", true},
		{"invalid", false},
		{"", false},
		{"PHOTO", false},
	}

	for _, tt := range tests {
		t.Run(tt.fileType, func(t *testing.T) {
			result := allowedFileTypes[tt.fileType]
			if result != tt.expected {
				t.Errorf("allowedFileTypes[%q] = %v, want %v", tt.fileType, result, tt.expected)
			}
		})
	}
}

func TestMaxFileSize(t *testing.T) {
	expected := int64(10 << 20) // 10MB
	if MaxFileSize != expected {
		t.Errorf("MaxFileSize = %d, want %d", MaxFileSize, expected)
	}
}
