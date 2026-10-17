package model

import (
	"testing"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

func TestDiagnosisProposalContentFingerprint(t *testing.T) {
	p := DiagnosisProposal{ChangeKind: DiagnosisCreate, Content: datatypes.JSON(`{"proposedEntry":{"medicalName":"Synthetic","causeAnalysis":{"summary":"Uncertain","rationale":"Example"}}}`), EvidenceRefs: []DiagnosisEvidenceRef{{EvidenceType: DiagnosisBodyFact, EvidenceID: uuid.New(), Version: "1"}}}
	original, err := DiagnosisProposalContentFingerprint(&p)
	if err != nil {
		t.Fatal(err)
	}
	equivalent := p
	equivalent.Content = datatypes.JSON(`{ "proposedEntry": {"causeAnalysis": {"rationale":"Example", "summary":"Uncertain"}, "medicalName":"Synthetic"} }`)
	if hash, err := DiagnosisProposalContentFingerprint(&equivalent); err != nil || hash != original {
		t.Fatalf("non-canonical fingerprint %s %v", hash, err)
	}
	for _, field := range []string{"content", "target", "operation", "version", "counter"} {
		t.Run(field, func(t *testing.T) {
			changed := p
			switch field {
			case "content":
				changed.Content = datatypes.JSON(`{"proposedEntry":{"medicalName":"Different"}}`)
			case "target":
				id := uuid.New()
				changed.TargetConditionID = &id
			case "operation":
				changed.ChangeKind = DiagnosisUpdate
			case "version":
				changed.EvidenceRefs = append([]DiagnosisEvidenceRef{}, p.EvidenceRefs...)
				changed.EvidenceRefs[0].Version = "2"
			case "counter":
				changed.CounterEvidenceRefs = append([]DiagnosisEvidenceRef{}, p.EvidenceRefs...)
			}
			if hash, err := DiagnosisProposalContentFingerprint(&changed); err != nil || hash == original {
				t.Fatalf("authorization not bound to %s: %s %v", field, hash, err)
			}
		})
	}
	for _, raw := range []string{"null", "[]", "{} {}", "not-json"} {
		bad := p
		bad.Content = datatypes.JSON(raw)
		if _, err := DiagnosisProposalContentFingerprint(&bad); err == nil {
			t.Fatalf("accepted invalid content %q", raw)
		}
	}
}
