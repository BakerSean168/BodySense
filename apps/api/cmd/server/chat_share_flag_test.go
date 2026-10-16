package main

import (
	"strings"
	"testing"
)

func TestPCD8RetireChatShareFlagIsExplicitAndStrict(t *testing.T) {
	tests := []struct {
		raw     string
		enabled bool
		invalid bool
	}{
		{"", false, false},
		{"false", false, false},
		{" FALSE ", false, false},
		{"true", true, false},
		{" TRUE ", true, false},
		{"True", true, false},
		{"1", false, true},
		{"yes", false, true},
		{"tru", false, true},
	}
	for _, tc := range tests {
		t.Run("input-"+strings.TrimSpace(tc.raw), func(t *testing.T) {
			enabled, err := parseRetireChatShareFlag(tc.raw)
			if tc.invalid {
				if err == nil || enabled {
					t.Fatalf("flag=%q silently allowed invalid operator setting", tc.raw)
				}
				return
			}
			if err != nil || enabled != tc.enabled {
				t.Fatalf("flag=%q enabled=%v err=%v, want=%v", tc.raw, enabled, err, tc.enabled)
			}
		})
	}
}
