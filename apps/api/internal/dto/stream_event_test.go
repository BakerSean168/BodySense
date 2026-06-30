package dto

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestStreamEventFixtureParity(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "packages", "contracts", "fixtures", "stream-events.v1.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var events []StreamEvent
	if err := json.Unmarshal(data, &events); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	if events[1].Channel != "job" {
		t.Fatalf("expected job event, got %q", events[1].Channel)
	}
	if events[1].IDs.JobID != "job-1" {
		t.Fatalf("expected job_id to round-trip, got %q", events[1].IDs.JobID)
	}
}

func TestNewStreamEventIncludesJobID(t *testing.T) {
	event, err := NewStreamEvent(1, "job", "job.completed", StreamEventIDs{JobID: "job-1"}, map[string]any{
		"result": "ok",
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}

	var decoded StreamEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("decode event: %v", err)
	}
	if decoded.IDs.JobID != "job-1" {
		t.Fatalf("expected job_id to survive JSON round-trip, got %q", decoded.IDs.JobID)
	}
}
