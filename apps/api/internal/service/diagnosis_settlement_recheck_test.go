package service

import (
	"errors"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

func TestDiagnosisSettlementSupersessionLineage(t *testing.T) {
	owner, run, a, b, c := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	proposal := func(id uuid.UUID, parent *uuid.UUID) model.DiagnosisProposal {
		return model.DiagnosisProposal{UserID: owner, RunID: run, ID: id, SupersedesProposalID: parent}
	}
	t.Run("only the last live proposal awaits choice", func(t *testing.T) {
		choices, err := newDiagnosisRunChoices([]model.DiagnosisProposal{proposal(a, nil), proposal(b, &a), proposal(c, &b)}, nil, nil)
		if err != nil || len(choices.pending) != 1 || choices.pending[0] != c {
			t.Fatalf("lineage %+v %v", choices, err)
		}
	})
	for name, proposals := range map[string][]model.DiagnosisProposal{
		"cycle":               {proposal(a, &b), proposal(b, &a)},
		"fork":                {proposal(a, nil), proposal(b, &a), proposal(c, &a)},
		"missing predecessor": {proposal(b, &a)},
		"duplicate identity":  {proposal(a, nil), proposal(a, nil)},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := newDiagnosisRunChoices(proposals, nil, nil); !errors.Is(err, ErrDiagnosisSettlementInvalid) {
				t.Fatalf("invalid supersession accepted: %v", err)
			}
		})
	}
}
