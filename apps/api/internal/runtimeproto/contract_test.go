package runtimeproto_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	protovalidate "buf.build/go/protovalidate"
	runtimev1 "github.com/bodysense/api/internal/generated/runtimeproto/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type invalidCase struct {
	Name    string          `json:"name"`
	Message string          `json:"message"`
	Value   json.RawMessage `json:"value"`
}

type runtimeCorpus struct {
	StartTurn       json.RawMessage   `json:"start_turn"`
	ResumeInterrupt json.RawMessage   `json:"resume_interrupt"`
	Events          []json.RawMessage `json:"events"`
	Invalid         []invalidCase     `json:"invalid"`
}

func loadRuntimeCorpus(t *testing.T) runtimeCorpus {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve contract test path")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "contracts", "internal", "agent-runtime", "fixtures", "runtime.v1.json"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read runtime corpus: %v", err)
	}
	var corpus runtimeCorpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatalf("decode runtime corpus: %v", err)
	}
	return corpus
}

func parseAndValidate(t *testing.T, raw json.RawMessage, message proto.Message) error {
	t.Helper()
	if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(raw, message); err != nil {
		return err
	}
	return protovalidate.Validate(message)
}

func TestRuntimeProtoValidCorpus(t *testing.T) {
	corpus := loadRuntimeCorpus(t)
	if err := parseAndValidate(t, corpus.StartTurn, &runtimev1.StartTurnCommand{}); err != nil {
		t.Fatalf("valid start-turn rejected: %v", err)
	}
	if err := parseAndValidate(t, corpus.ResumeInterrupt, &runtimev1.ResumeInterruptCommand{}); err != nil {
		t.Fatalf("valid resume rejected: %v", err)
	}
	if len(corpus.Events) != 17 {
		t.Fatalf("event corpus = %d, want 17 typed variants", len(corpus.Events))
	}
	for i, raw := range corpus.Events {
		var event runtimev1.RuntimeEvent
		if err := parseAndValidate(t, raw, &event); err != nil {
			t.Fatalf("valid event %d rejected: %v", i+1, err)
		}
		if event.GetEvent() == nil {
			t.Fatalf("valid event %d has no oneof variant", i+1)
		}
	}
}

func TestRuntimeProtoInvalidCorpus(t *testing.T) {
	corpus := loadRuntimeCorpus(t)
	for _, tc := range corpus.Invalid {
		t.Run(tc.Name, func(t *testing.T) {
			var message proto.Message
			switch tc.Message {
			case "start_turn":
				message = &runtimev1.StartTurnCommand{}
			case "resume_interrupt":
				message = &runtimev1.ResumeInterruptCommand{}
			case "runtime_event":
				message = &runtimev1.RuntimeEvent{}
			default:
				t.Fatalf("unknown fixture message type %q", tc.Message)
			}
			if err := parseAndValidate(t, tc.Value, message); err == nil {
				t.Fatalf("invalid fixture %q was accepted", tc.Name)
			}
		})
	}
}
