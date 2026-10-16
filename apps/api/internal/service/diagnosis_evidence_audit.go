package service

import (
	"encoding/json"
	"fmt"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// These are references to the exact reasoning input, not another BodyState
// ledger. Citation text remains Agent audit and never becomes statement_evidence.
type diagnosisEvidenceAudit struct {
	BodyStateAuthority string                     `json:"body_state_authority"`
	BodyStateRefs      []diagnosisBodyEvidenceRef `json:"body_state_refs"`
	Citations          []map[string]any           `json:"citations"`
}

type diagnosisBodyEvidenceRef struct {
	EvidenceType        string    `json:"evidence_type"`
	EvidenceID          uuid.UUID `json:"evidence_id"`
	Version             int64     `json:"version"`
	StatementEvidenceID uuid.UUID `json:"statement_evidence_id"`
	SourceVersion       int64     `json:"source_version,omitempty"`
	AssertionStatus     string    `json:"assertion_status"`
}

func buildDiagnosisEvidenceAudit(owner uuid.UUID, revision int64, replayInput, citations json.RawMessage) (diagnosisEvidenceAudit, error) {
	audit := diagnosisEvidenceAudit{BodyStateAuthority: "none", BodyStateRefs: []diagnosisBodyEvidenceRef{}, Citations: []map[string]any{}}
	if len(citations) > 0 && string(citations) != "null" {
		if err := json.Unmarshal(citations, &audit.Citations); err != nil {
			return audit, fmt.Errorf("decode Diagnosis audit citations: %w", err)
		}
	}
	// Non-Agent callers can persist a result without a reasoning input. Such a
	// result has no verified native references; no AI-provided IDs fill the gap.
	if len(replayInput) == 0 {
		return audit, nil
	}
	var frozen DiagnosisReplayInput
	if err := json.Unmarshal(replayInput, &frozen); err != nil {
		return audit, fmt.Errorf("decode Diagnosis audit input: %w", err)
	}
	var body BodyStateReadModel
	if err := json.Unmarshal(frozen.BodyState, &body); err != nil {
		return audit, fmt.Errorf("decode native Diagnosis audit sources: %w", err)
	}
	if owner == uuid.Nil || body.UserID != owner {
		return audit, fmt.Errorf("Diagnosis audit source owner mismatch")
	}
	if frozen.BodyStateRevision != revision || body.CurrentRevision != revision {
		return audit, fmt.Errorf("Diagnosis audit source revision mismatch")
	}
	for _, fact := range body.Facts {
		if fact.UserID != owner {
			return audit, fmt.Errorf("Diagnosis audit fact owner mismatch")
		}
		if fact.ID == uuid.Nil || fact.SourceEvidenceID == uuid.Nil || fact.CreatedRevision <= 0 || fact.CreatedRevision > revision {
			return audit, fmt.Errorf("invalid native Diagnosis fact reference")
		}
		audit.BodyStateRefs = append(audit.BodyStateRefs, diagnosisBodyEvidenceRef{
			EvidenceType: "body_fact", EvidenceID: fact.ID, Version: fact.CreatedRevision,
			StatementEvidenceID: fact.SourceEvidenceID, AssertionStatus: fact.AssertionStatus,
		})
	}
	for _, observation := range body.Observations {
		if observation.UserID != owner {
			return audit, fmt.Errorf("Diagnosis audit observation owner mismatch")
		}
		if observation.ID == uuid.Nil || observation.Source.ID == uuid.Nil || observation.Source.Version <= 0 || observation.CreatedRevision <= 0 || observation.CreatedRevision > revision {
			return audit, fmt.Errorf("invalid native Diagnosis observation reference")
		}
		audit.BodyStateRefs = append(audit.BodyStateRefs, diagnosisBodyEvidenceRef{
			EvidenceType: "body_observation", EvidenceID: observation.ID, Version: observation.CreatedRevision,
			StatementEvidenceID: observation.Source.ID, SourceVersion: observation.Source.Version,
			AssertionStatus: observation.AssertionStatus,
		})
	}
	return audit, nil
}

// Treatment receives only the selected analysis's audit, never an owner-wide
// evidence pool. Current BodyState and Safety remain separate authoritative inputs.
func treatmentAnalysisEvidence(owner uuid.UUID, analysis *model.DiagnosisAnalysisRecord) (json.RawMessage, error) {
	if analysis.UserID != owner {
		return nil, fmt.Errorf("Treatment analysis owner mismatch")
	}
	var trace struct {
		Evidence diagnosisEvidenceAudit `json:"evidence"`
	}
	if len(analysis.DecisionTrace) > 0 {
		if err := json.Unmarshal(analysis.DecisionTrace, &trace); err != nil {
			return nil, fmt.Errorf("decode selected Diagnosis evidence audit: %w", err)
		}
	}
	if trace.Evidence.BodyStateAuthority != "none" {
		return nil, fmt.Errorf("selected Diagnosis evidence audit is unavailable")
	}
	items := make([]any, 0, len(trace.Evidence.BodyStateRefs)+len(trace.Evidence.Citations))
	for _, ref := range trace.Evidence.BodyStateRefs {
		items = append(items, ref)
	}
	for _, citation := range trace.Evidence.Citations {
		// The Agent's citation fields cannot impersonate a verified native ref.
		item := make(map[string]any, len(citation)+2)
		for key, value := range citation {
			item[key] = value
		}
		item["evidence_type"] = "external_citation"
		item["body_state_authority"] = "none"
		items = append(items, item)
	}
	return json.Marshal(items)
}
