package service

import (
	"encoding/json"
	"testing"
)

// PC-RET-001: the context cursor rides the validated runtime command.
func TestStartTurnCommandCarriesTheContextWindow(t *testing.T) {
	req := validStartConsultationTurnRequest()
	req.BusinessContext.ContextWindow = &ConsultationContextWindow{
		ContextStartSeq: 7,
		RunIDs:          []string{testRuntimeRunID, testRuntimeInterruptID},
	}
	body, err := marshalStartTurnCommand(testRuntimeThreadID, req)
	if err != nil {
		t.Fatal(err)
	}
	var command struct {
		BusinessContext struct {
			ContextWindow struct {
				ContextStartSeq int      `json:"context_start_seq"`
				RunIDs          []string `json:"run_ids"`
			} `json:"context_window"`
		} `json:"business_context"`
	}
	if err := json.Unmarshal(body, &command); err != nil {
		t.Fatal(err)
	}
	window := command.BusinessContext.ContextWindow
	if window.ContextStartSeq != 7 || len(window.RunIDs) != 2 || window.RunIDs[0] != testRuntimeRunID {
		t.Fatalf("context_window = %+v", window)
	}
}

func TestStartTurnCommandWithoutContextWindowOmitsIt(t *testing.T) {
	body, err := marshalStartTurnCommand(testRuntimeThreadID, validStartConsultationTurnRequest())
	if err != nil {
		t.Fatal(err)
	}
	var command struct {
		BusinessContext map[string]any `json:"business_context"`
	}
	if err := json.Unmarshal(body, &command); err != nil {
		t.Fatal(err)
	}
	if _, ok := command.BusinessContext["context_window"]; ok {
		t.Fatal("absent window was sent")
	}
}

func TestStartTurnCommandRejectsAnInvalidContextWindow(t *testing.T) {
	for name, window := range map[string]*ConsultationContextWindow{
		"zero start":     {ContextStartSeq: 0},
		"non-uuid run":   {ContextStartSeq: 1, RunIDs: []string{"run-1"}},
		"duplicate runs": {ContextStartSeq: 1, RunIDs: []string{testRuntimeRunID, testRuntimeRunID}},
	} {
		req := validStartConsultationTurnRequest()
		req.BusinessContext.ContextWindow = window
		if _, err := marshalStartTurnCommand(testRuntimeThreadID, req); err == nil {
			t.Fatalf("%s: invalid window accepted", name)
		}
	}
}
