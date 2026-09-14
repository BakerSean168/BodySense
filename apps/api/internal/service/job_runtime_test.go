package service

import (
	"testing"

	"github.com/bodysense/api/internal/model"
)

func TestValidateTransition(t *testing.T) {
	tests := []struct {
		from model.JobStatus
		to   model.JobStatus
		want bool
	}{
		{model.JobStatusPending, model.JobStatusRunning, true},
		{model.JobStatusPending, model.JobStatusCancelled, true},
		{model.JobStatusPending, model.JobStatusCompleted, false},
		{model.JobStatusRunning, model.JobStatusCompleted, true},
		{model.JobStatusRunning, model.JobStatusFailed, true},
		{model.JobStatusRunning, model.JobStatusCancelled, true},
		{model.JobStatusRunning, model.JobStatusTimedOut, true},
		{model.JobStatusRunning, model.JobStatusPending, true},
		{model.JobStatusCompleted, model.JobStatusRunning, false},
		{model.JobStatusFailed, model.JobStatusRunning, false},
		{model.JobStatusCancelled, model.JobStatusRunning, false},
		{model.JobStatus("unknown"), model.JobStatusRunning, false},
	}

	for _, tt := range tests {
		got := ValidateTransition(tt.from, tt.to)
		if got != tt.want {
			t.Errorf("ValidateTransition(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestJobTransitions_AllTerminalStatesEmpty(t *testing.T) {
	terminal := []model.JobStatus{
		model.JobStatusCompleted,
		model.JobStatusFailed,
		model.JobStatusCancelled,
		model.JobStatusTimedOut,
	}
	for _, status := range terminal {
		if allowed := jobTransitions[status]; len(allowed) != 0 {
			t.Errorf("terminal state %q should have no transitions, got %v", status, allowed)
		}
	}
}

func TestJobTransitions_PendingHasRunningAndCancelled(t *testing.T) {
	allowed := jobTransitions[model.JobStatusPending]
	if _, ok := allowed[model.JobStatusRunning]; !ok {
		t.Error("pending should allow transition to running")
	}
	if _, ok := allowed[model.JobStatusCancelled]; !ok {
		t.Error("pending should allow transition to cancelled")
	}
}

func TestJobTransitions_WaitingUserCanResume(t *testing.T) {
	allowed := jobTransitions[model.JobStatusWaitingUser]
	if _, ok := allowed[model.JobStatusRunning]; !ok {
		t.Error("waiting_user should allow transition to running")
	}
	if _, ok := allowed[model.JobStatusCancelled]; !ok {
		t.Error("waiting_user should allow transition to cancelled")
	}
}

func TestJobTransitions_RunningCanWaitUser(t *testing.T) {
	if _, ok := jobTransitions[model.JobStatusRunning][model.JobStatusWaitingUser]; !ok {
		t.Error("running should allow transition to waiting_user")
	}
}

func TestJobEventTypeForStatus(t *testing.T) {
	tests := []struct {
		status model.JobStatus
		want   string
	}{
		{model.JobStatusCompleted, "job.completed"},
		{model.JobStatusFailed, "job.failed"},
		{model.JobStatusRunning, "job.running"},
	}
	for _, tt := range tests {
		if got := jobEventTypeForStatus(tt.status); got != tt.want {
			t.Errorf("jobEventTypeForStatus(%q) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestJobStatusHasNoSucceededAlias(t *testing.T) {
	if _, ok := jobTransitions[model.JobStatus("succeeded")]; ok {
		t.Fatal("job lifecycle must not retain succeeded as an alias for completed")
	}
}
