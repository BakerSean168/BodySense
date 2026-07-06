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
		{"pending", "completed", false},
		{"running", "completed", true},
		{"running", "failed", true},
		{"running", "cancelled", true},
		{"running", "timed_out", true},
		{"running", "pending", false},
		{"completed", "running", false},
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
	terminal := []string{"completed", "succeeded", "failed", "cancelled", "timed_out"}
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

func TestJobTransitions_WaitingUserCanResume(t *testing.T) {
	allowed := jobTransitions["waiting_user"]
	has := map[string]bool{}
	for _, s := range allowed {
		has[s] = true
	}
	if !has["running"] {
		t.Error("waiting_user should allow transition to running")
	}
	if !has["cancelled"] {
		t.Error("waiting_user should allow transition to cancelled")
	}
}

func TestJobTransitions_RunningCanWaitUser(t *testing.T) {
	allowed := jobTransitions["running"]
	has := map[string]bool{}
	for _, s := range allowed {
		has[s] = true
	}
	if !has["waiting_user"] {
		t.Error("running should allow transition to waiting_user")
	}
}

func TestJobEventTypeForStatus(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"completed", "job.completed"},
		{"succeeded", "job.completed"},
		{"failed", "job.failed"},
		{"running", "job.running"},
	}
	for _, tt := range tests {
		if got := jobEventTypeForStatus(tt.status); got != tt.want {
			t.Errorf("jobEventTypeForStatus(%q) = %q, want %q", tt.status, got, tt.want)
		}
	}
}
