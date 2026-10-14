package consultation

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type captureFactService struct {
	calls          int
	owner, run     uuid.UUID
	command        model.RecordBodyFactsCommand
	fail           error
	missingReceipt bool
}

func (f *captureFactService) ListFacts(context.Context, uuid.UUID, int64) (*model.BodyFactPage, error) {
	return nil, nil
}
func (f *captureFactService) RecordRunFacts(_ context.Context, owner, run uuid.UUID, command model.RecordBodyFactsCommand) (*model.BodyFactCommit, error) {
	f.calls++
	f.owner = owner
	f.run = run
	f.command = command
	if f.fail != nil {
		return nil, f.fail
	}
	if f.missingReceipt {
		return nil, nil
	}
	ids := make([]uuid.UUID, len(command.Facts))
	for i := range ids {
		ids[i] = uuid.New()
	}
	encoded, _ := json.Marshal(ids)
	return &model.BodyFactCommit{ID: uuid.New(), UserID: owner, Revision: 3, RevisionID: uuid.New(), EventID: uuid.New(), FactIDs: datatypes.JSON(encoded)}, nil
}
func factStreamState() streamState {
	owner, conversation := uuid.New(), uuid.New()
	return streamState{UID: owner, ConversationID: conversation, Run: &model.Run{ID: uuid.New(), UserID: owner, ConversationID: conversation}, ExpectedConfigurationID: service.BodyFactConsultationConfigurationID}
}
func proposedFacts() json.RawMessage {
	return json.RawMessage(`{"status":"proposed","expected_revision":2,"facts":[{"kind":"symptom","statement":"I have no shoulder pain","body_region_ids":[],"temporal":{"precision":"unknown"},"details":{"presence":"absent"}}]}`)
}
func TestBodyFactToolUsesServerRunIdentityAndRealReceipt(t *testing.T) {
	ledger := &captureFactService{}
	r := &Runtime{}
	r.AttachBodyFactService(ledger)
	state := factStreamState()
	raw, err := r.commitBodyFactProposal(context.Background(), state, "intake-v1", proposedFacts())
	if err != nil {
		t.Fatal(err)
	}
	if ledger.owner != state.UID || ledger.run != state.Run.ID || ledger.calls != 1 {
		t.Fatal("source identity not server bound")
	}
	originalKey := ledger.command.IdempotencyKey
	if ledger.command.ExpectedRevision != 2 || len(originalKey) > 128 {
		t.Fatal("command revision/idempotency lost")
	}
	var reply struct {
		Status  string               `json:"status"`
		Receipt model.BodyFactCommit `json:"receipt"`
	}
	if err := json.Unmarshal(raw, &reply); err != nil || reply.Status != "persisted" || reply.Receipt.ID == uuid.Nil {
		t.Fatalf("reply=%s err=%v", raw, err)
	}
	if _, err := r.commitBodyFactProposal(context.Background(), state, "intake-v1", proposedFacts()); err != nil || ledger.command.IdempotencyKey != originalKey {
		t.Fatal("retry identity changed")
	}
}
func TestBodyFactToolRejectsForgedSuccessIdentityAndMissingRevision(t *testing.T) {
	cases := []string{
		`{"status":"persisted","receipt":{"success":true}}`,
		`{"status":"proposed","expected_revision":2,"facts":[],"user_id":"other"}`,
		`{"status":"proposed","facts":[]}`,
		string(proposedFacts()) + ` {"status":"persisted"}`,
	}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			ledger := &captureFactService{}
			r := &Runtime{bodyFacts: ledger}
			result, err := r.commitBodyFactProposal(context.Background(), factStreamState(), "intake-v1", json.RawMessage(raw))
			if err == nil || result != nil || ledger.calls != 0 {
				t.Fatal("untrusted proposal reached persistence")
			}
		})
	}
}
func TestBodyFactToolFailureNeverReturnsSuccessReceipt(t *testing.T) {
	for _, ledger := range []*captureFactService{{fail: errors.New("database unavailable")}, {missingReceipt: true}} {
		r := &Runtime{bodyFacts: ledger}
		result, err := r.commitBodyFactProposal(context.Background(), factStreamState(), "intake-v1", proposedFacts())
		if err == nil || result != nil {
			t.Fatal("failure leaked a success response")
		}
	}
}
func TestBodyFactToolRejectsHistoricalOrForeignRun(t *testing.T) {
	ledger := &captureFactService{}
	r := &Runtime{bodyFacts: ledger}
	state := factStreamState()
	state.ExpectedConfigurationID = "consult-config-7feb8ca2d5bfad5a"
	if _, err := r.commitBodyFactProposal(context.Background(), state, "intake-v1", proposedFacts()); err == nil {
		t.Fatal("historical config mutated new ledger")
	}
	state = factStreamState()
	state.Run.UserID = uuid.New()
	if _, err := r.commitBodyFactProposal(context.Background(), state, "intake-v1", proposedFacts()); err == nil || ledger.calls != 0 {
		t.Fatal("foreign run accepted")
	}
}

func TestBodyFactRuntimeV3RejectsLegacyFactMutationEvents(t *testing.T) {
	for _, event := range []service.ConsultationRuntimeEvent{
		{Payload: service.ConsultationRuntimeExtractedInfoPayload{}},
		{Payload: service.ConsultationRuntimeLifestyleContextPayload{}},
	} {
		if validateBodyFactRuntimeEvent(service.BodyFactConsultationConfigurationID, event) == nil {
			t.Fatal("v3 accepted legacy fact write")
		}
		if err := validateBodyFactRuntimeEvent("consult-config-7feb8ca2d5bfad5a", event); err != nil {
			t.Fatal("historical v2 behavior changed")
		}
	}
	if err := validateBodyFactRuntimeEvent(service.BodyFactConsultationConfigurationID, service.ConsultationRuntimeEvent{Payload: service.ConsultationRuntimeToolResultPayload{Tool: "record_body_state"}}); err != nil {
		t.Fatal(err)
	}
}

func TestBodyFactExpectedRevisionFollowsOnlyOwnContiguousSafetyReceipt(t *testing.T) {
	cases := []struct {
		name     string
		target   int64
		foreign  bool
		invalid  bool
		expected int64
	}{
		{name: "own contiguous", target: 3, expected: 3},
		{name: "external revision gap", target: 4, expected: 2},
		{name: "foreign receipt", target: 3, foreign: true, expected: 2},
		{name: "unpersisted receipt", target: 3, invalid: true, expected: 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ledger := &captureFactService{}
			r := &Runtime{bodyFacts: ledger}
			state := factStreamState()
			receipt := model.BodyStateRevision{ID: uuid.New(), UserID: state.UID, Revision: tc.target, ChangeType: "safety.updated", Source: "consultation"}
			if tc.foreign {
				receipt.UserID = uuid.New()
			}
			if tc.invalid {
				receipt.ID = uuid.Nil
			}
			if _, err := r.commitBodyFactProposal(context.Background(), state, "capture", proposedFacts(), receipt); err != nil {
				t.Fatal(err)
			}
			if ledger.command.ExpectedRevision != tc.expected {
				t.Fatalf("revision=%d wanted=%d", ledger.command.ExpectedRevision, tc.expected)
			}
		})
	}
}
