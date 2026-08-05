package dto

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func contractsPath(parts ...string) string {
	base := []string{"..", "..", "..", "..", "packages", "contracts"}
	return filepath.Join(append(base, parts...)...)
}

func loadFixtureEvents(t *testing.T) []StreamEvent {
	t.Helper()
	path := contractsPath("fixtures", "stream-events.v1.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var events []StreamEvent
	if err := json.Unmarshal(data, &events); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	return events
}

func TestStreamEventFixtureParity(t *testing.T) {
	events := loadFixtureEvents(t)
	if len(events) < 5 {
		t.Fatalf("expected at least 5 events, got %d", len(events))
	}

	byType := map[string]StreamEvent{}
	for _, e := range events {
		byType[e.Type] = e
	}

	if job, ok := byType["job.progress"]; !ok || job.Channel != "job" || job.IDs.JobID != "job-1" {
		t.Fatalf("expected job.progress fixture with job_id, got %#v", byType["job.progress"])
	}
	var foundInteraction bool
	for _, e := range events {
		if e.Type == "state.interaction.required" && e.IDs.InteractionID == "interaction-1" {
			foundInteraction = true
			break
		}
	}
	if !foundInteraction {
		t.Fatal("expected state.interaction.required fixture with interaction_id=interaction-1")
	}
	if _, ok := byType["state.interaction.expired"]; !ok {
		t.Fatal("expected state.interaction.expired fixture")
	}
	if run, ok := byType["run.started"]; !ok || run.Channel != "run" || run.IDs.RunID != "run-1" {
		t.Fatalf("expected run.started fixture, got %#v", byType["run.started"])
	}
	if title, ok := byType["title.generated"]; !ok || title.Channel != "title" {
		t.Fatalf("expected title.generated fixture, got %#v", byType["title.generated"])
	}
	if rev, ok := byType["safety.output_reviewed"]; !ok || rev.Channel != "safety" {
		t.Fatalf("expected safety.output_reviewed fixture, got %#v", byType)
	}
	if rej, ok := byType["safety.output_rejected"]; !ok || rej.Channel != "safety" {
		t.Fatalf("expected safety.output_rejected fixture, got %#v", byType)
	}
}

func TestFixtureMatchesSchema(t *testing.T) {
	path := contractsPath("fixtures", "stream-events.v1.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var raw []map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw fixture: %v", err)
	}

	channelEnum := map[string]bool{
		"conversation": true, "run": true, "message": true, "tool": true,
		"state": true, "source": true, "safety": true, "usage": true,
		"job": true, "stream": true, "title": true,
	}
	idKeys := map[string]bool{
		"conversation_id": true, "run_id": true, "turn_id": true, "message_id": true,
		"tool_call_id": true, "interaction_id": true, "job_id": true,
	}
	topKeys := map[string]bool{
		"version": true, "seq": true, "channel": true, "type": true, "ids": true, "payload": true,
	}

	for i, item := range raw {
		for k := range item {
			if !topKeys[k] {
				t.Fatalf("event[%d] has additional property %q", i, k)
			}
		}
		for _, req := range []string{"version", "seq", "channel", "type", "ids", "payload"} {
			if _, ok := item[req]; !ok {
				t.Fatalf("event[%d] missing required %q", i, req)
			}
		}
		if v, ok := item["version"].(float64); !ok || v != 1 {
			t.Fatalf("event[%d] version must be 1, got %#v", i, item["version"])
		}
		seq, ok := item["seq"].(float64)
		if !ok || seq < 1 || seq != float64(int(seq)) {
			t.Fatalf("event[%d] seq must be int >= 1, got %#v", i, item["seq"])
		}
		ch, _ := item["channel"].(string)
		if !channelEnum[ch] {
			t.Fatalf("event[%d] channel %q not in enum", i, ch)
		}
		typ, _ := item["type"].(string)
		if typ == "" {
			t.Fatalf("event[%d] type must be non-empty", i)
		}
		ids, ok := item["ids"].(map[string]any)
		if !ok {
			t.Fatalf("event[%d] ids must be object", i)
		}
		for k, v := range ids {
			if !idKeys[k] {
				t.Fatalf("event[%d] ids has unknown key %q", i, k)
			}
			if v != nil {
				if _, ok := v.(string); !ok {
					t.Fatalf("event[%d] ids.%s must be string or null", i, k)
				}
			}
		}
		if _, ok := item["payload"].(map[string]any); !ok {
			t.Fatalf("event[%d] payload must be object", i)
		}
	}
}

func TestFixtureCoversRequiredEventTypes(t *testing.T) {
	path := contractsPath("fixtures", "stream-event-types.v1.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read required types: %v", err)
	}
	var spec struct {
		RequiredEventTypes []string `json:"required_event_types"`
	}
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatalf("unmarshal required types: %v", err)
	}

	events := loadFixtureEvents(t)
	present := map[string]bool{}
	for _, e := range events {
		present[e.Type] = true
	}
	var missing []string
	for _, typ := range spec.RequiredEventTypes {
		if !present[typ] {
			missing = append(missing, typ)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("fixture missing required event types: %v", missing)
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

func TestConstructedEventMatchesSchemaShape(t *testing.T) {
	// T0-3 C1: events built by NewStreamEvent must satisfy the same closed
	// envelope rules as fixtures (version/seq/channel/type/ids/payload).
	event, err := NewStreamEvent(9, "state", "state.interaction.expired", StreamEventIDs{
		ConversationID: "conv-1",
		RunID:          "run-1",
		InteractionID:  "interaction-1",
		ToolCallID:     "tool-1",
	}, map[string]any{
		"interaction_id": "interaction-1",
		"expired_at":     "2026-07-26T12:00:00Z",
		"reason":         "ttl_elapsed",
	})
	if err != nil {
		t.Fatalf("NewStreamEvent: %v", err)
	}
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	topKeys := map[string]bool{"version": true, "seq": true, "channel": true, "type": true, "ids": true, "payload": true}
	for k := range raw {
		if !topKeys[k] {
			t.Fatalf("additional property %q", k)
		}
	}
	if raw["version"] != float64(1) {
		t.Fatalf("version: %#v", raw["version"])
	}
	if raw["channel"] != "state" || raw["type"] != "state.interaction.expired" {
		t.Fatalf("channel/type: %#v %#v", raw["channel"], raw["type"])
	}
	ids, _ := raw["ids"].(map[string]any)
	if ids["interaction_id"] != "interaction-1" {
		t.Fatalf("ids: %#v", ids)
	}
}
