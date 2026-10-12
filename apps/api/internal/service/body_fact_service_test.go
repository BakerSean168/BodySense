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
