package model

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"

	"github.com/google/uuid"
)

type ConditionRelationKind string

const (
	ConditionDifferentialWith ConditionRelationKind = "differential_with"
	ConditionSupersedes       ConditionRelationKind = "supersedes"
	ConditionCoexisting       ConditionRelationKind = "coexisting"
)

// DiagnosisProposalContentFingerprint binds the exact semantic operation,
// target, proposed content and source versions. Lifecycle/proposal IDs and the
// base revision are checked separately; none can change an approved payload.
// Objects are canonicalized, while array ordering and JSON numbers are retained.
func DiagnosisProposalContentFingerprint(p *DiagnosisProposal) (string, error) {
	if p == nil || len(p.Content) == 0 || len(p.Content) > 1<<20 {
		return "", fmt.Errorf("invalid diagnosis proposal content")
	}
	var content map[string]any
	d := json.NewDecoder(bytes.NewReader(p.Content))
	d.UseNumber()
	if err := d.Decode(&content); err != nil || content == nil {
		return "", fmt.Errorf("proposal content must be a JSON object")
	}
	var trailing any
	if err := d.Decode(&trailing); err != io.EOF {
		return "", fmt.Errorf("trailing proposal content")
	}
	refs := append([]DiagnosisEvidenceRef{}, p.EvidenceRefs...)
	counter := append([]DiagnosisEvidenceRef{}, p.CounterEvidenceRefs...)
	encoded, err := json.Marshal(struct {
		ChangeKind        DiagnosisChangeKind    `json:"changeKind"`
		TargetConditionID *uuid.UUID             `json:"targetConditionId"`
		Content           map[string]any         `json:"content"`
		EvidenceRefs      []DiagnosisEvidenceRef `json:"evidenceRefs"`
		CounterRefs       []DiagnosisEvidenceRef `json:"counterEvidenceRefs"`
	}{p.ChangeKind, p.TargetConditionID, content, refs, counter})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}
