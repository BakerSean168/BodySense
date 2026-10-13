package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type fakeFactLedger struct {
	calls int
	owner uuid.UUID
}

func (f *fakeFactLedger) RecordDirectFacts(_ context.Context, owner uuid.UUID, _ model.RecordBodyFactsCommand) (*model.BodyFactCommit, error) {
	f.calls++
	f.owner = owner
	return &model.BodyFactCommit{ID: uuid.New(), UserID: owner, Revision: 1}, nil
}
func (f *fakeFactLedger) RecordMessageFacts(_ context.Context, owner uuid.UUID, _ model.MessageFactSource, _ model.RecordBodyFactsCommand) (*model.BodyFactCommit, error) {
	f.calls++
	f.owner = owner
	return &model.BodyFactCommit{ID: uuid.New(), UserID: owner, Revision: 1}, nil
}
func (f *fakeFactLedger) RecordRunFacts(ctx context.Context, owner, runID uuid.UUID, command model.RecordBodyFactsCommand) (*model.BodyFactCommit, error) {
	return f.RecordMessageFacts(ctx, owner, model.MessageFactSource{RunID: runID}, command)
}
func (f *fakeFactLedger) ListFacts(_ context.Context, owner uuid.UUID, _ int64) (*model.BodyFactPage, error) {
	f.calls++
	f.owner = owner
	return &model.BodyFactPage{UserID: owner, Facts: []model.BodyFact{}}, nil
}
func serviceFactCommand() model.RecordBodyFactsCommand {
	return model.RecordBodyFactsCommand{IdempotencyKey: "service-test", Facts: []model.BodyFactInput{{Kind: "symptom", Statement: "I report shoulder discomfort", BodyRegionIDs: []string{}, Temporal: model.FactTemporal{Precision: "unknown"}, Details: json.RawMessage(`{}`)}}}
}
func TestBodyFactServiceValidatesBeforePersistence(t *testing.T) {
	cases := []struct {
		name      string
		region    string
		validator BodyRegionIDValidator
		want      error
	}{
		{"unavailable", "shoulder.left", nil, ErrBodyRegionIDValidationUnavailable},
		{"unknown", "not-an-anatomical-region", NewCanonicalBodyRegionIDValidator(), ErrUnknownBodyRegionID},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			ledger := &fakeFactLedger{}
			command := serviceFactCommand()
			command.Facts[0].BodyRegionIDs = []string{test.region}
			_, err := NewBodyFactService(ledger, test.validator).RecordDirectFacts(context.Background(), uuid.New(), command)
			if !errors.Is(err, test.want) || ledger.calls != 0 {
				t.Fatalf("err=%v calls=%d", err, ledger.calls)
			}
		})
	}
}
func TestBodyFactServiceAcceptsExplicitStatementWithoutExtraApproval(t *testing.T) {
	ledger := &fakeFactLedger{}
	owner := uuid.New()
	command := serviceFactCommand()
	command.Facts[0].BodyRegionIDs = []string{"shoulder.left"}
	receipt, err := NewBodyFactService(ledger, NewCanonicalBodyRegionIDValidator()).RecordDirectFacts(context.Background(), owner, command)
	if err != nil || receipt == nil || ledger.calls != 1 || ledger.owner != owner {
		t.Fatalf("receipt=%#v err=%v", receipt, err)
	}
}
func TestBodyFactServiceRejectsMissingOwnerAndInvalidCommand(t *testing.T) {
	ledger := &fakeFactLedger{}
	application := NewBodyFactService(ledger, nil)
	if _, err := application.RecordDirectFacts(context.Background(), uuid.Nil, serviceFactCommand()); !errors.Is(err, model.ErrInvalidBodyFact) {
		t.Fatal(err)
	}
	invalid := serviceFactCommand()
	invalid.Facts[0].Kind = "diagnosis"
	if _, err := application.RecordDirectFacts(context.Background(), uuid.New(), invalid); !errors.Is(err, model.ErrInvalidBodyFact) {
		t.Fatal(err)
	}
	if _, err := application.ListFacts(context.Background(), uuid.Nil, 0); !errors.Is(err, model.ErrInvalidBodyFact) {
		t.Fatal(err)
	}
	if ledger.calls != 0 {
		t.Fatal("invalid command reached persistence")
	}
}

func TestBodyFactMessageSourceRequiresOwnerRunAndValidatedRegions(t *testing.T) {
	ledger := &fakeFactLedger{}
	application := NewBodyFactService(ledger, NewCanonicalBodyRegionIDValidator())
	owner := uuid.New()
	source := model.MessageFactSource{RunID: uuid.New(), MessageID: uuid.New()}
	command := serviceFactCommand()
	for _, tc := range []struct {
		name   string
		owner  uuid.UUID
		source model.MessageFactSource
		want   error
	}{
		{"missing owner", uuid.Nil, source, model.ErrInvalidBodyFact},
		{"missing run", owner, model.MessageFactSource{MessageID: source.MessageID}, model.ErrInvalidBodyFact},
		{"missing message", owner, model.MessageFactSource{RunID: source.RunID}, model.ErrInvalidBodyFact},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if receipt, err := application.RecordMessageFacts(context.Background(), tc.owner, tc.source, command); !errors.Is(err, tc.want) || receipt != nil {
				t.Fatalf("receipt=%#v err=%v", receipt, err)
			}
		})
	}
	command.Facts[0].BodyRegionIDs = []string{"unverified.claimed.anatomy"}
	if _, err := application.RecordMessageFacts(context.Background(), owner, source, command); !errors.Is(err, ErrUnknownBodyRegionID) {
		t.Fatalf("unknown canonical region accepted: %v", err)
	}
	if ledger.calls != 0 {
		t.Fatal("invalid message binding reached persistence")
	}
	command.Facts[0].BodyRegionIDs = []string{"shoulder.left"}
	if receipt, err := application.RecordMessageFacts(context.Background(), owner, source, command); err != nil || receipt == nil || ledger.calls != 1 || ledger.owner != owner {
		t.Fatalf("verified command not forwarded: %#v %v", receipt, err)
	}
}
