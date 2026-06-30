package service

import (
	"testing"
)

func TestValidateTransition(t *testing.T) {
	tests := []struct {
		from string
		to   string
		want bool
	}{
		{"pending", "running", true},
		{"pending", "cancelled", true},
		{"pending", "succeeded", false},
		{"running", "succeeded", true},
		{"running", "failed", true},
		{"running", "cancelled", true},
		{"running", "pending", false},
		{"succeeded", "running", false},
		{"failed", "running", false},
		{"cancelled", "running", false},
		{"unknown", "running", false},
	}

	for _, tt := range tests {
		got := ValidateTransition(tt.from, tt.to)
		if got != tt.want {
			t.Errorf("ValidateTransition(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestJobTransitions_AllTerminalStatesEmpty(t *testing.T) {
	terminal := []string{"succeeded", "failed", "cancelled"}
	for _, status := range terminal {
		allowed := jobTransitions[status]
		if len(allowed) != 0 {
			t.Errorf("terminal state %q should have no transitions, got %v", status, allowed)
		}
	}
}

func TestJobTransitions_PendingHasRunningAndCancelled(t *testing.T) {
	allowed := jobTransitions["pending"]
	has := map[string]bool{}
	for _, s := range allowed {
		has[s] = true
	}
	if !has["running"] {
		t.Error("pending should allow transition to running")
	}
	if !has["cancelled"] {
		t.Error("pending should allow transition to cancelled")
	}
}
