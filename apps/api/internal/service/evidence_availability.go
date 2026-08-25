package service

import (
	"encoding/json"
	"errors"
	"fmt"
)

const (
	evidenceAvailabilityTraceV2 = "evidence-acquisition-trace-v2"

	externalEvidenceNotRequired        = "not_required"
	externalEvidenceAvailable          = "available"
	externalEvidencePartiallyAvailable = "partially_available"
	externalEvidenceUnresolved         = "unresolved"
)

var ErrEvidenceAvailabilityTraceInvalid = errors.New("evidence availability trace is invalid")

type evidenceAvailabilityAttempt struct {
	Gap struct {
		Kind string `json:"kind"`
	} `json:"gap"`
	Status          string  `json:"status"`
	StopReason      string  `json:"stop_reason"`
	SearchPerformed bool    `json:"search_performed"`
	RetrievalStatus *string `json:"retrieval_status"`
}

type evidenceAvailabilityTrace struct {
	TraceRevision          string                        `json:"trace_revision"`
	PolicyRevision         string                        `json:"policy_revision"`
	ExternalEvidenceStatus string                        `json:"external_evidence_status"`
	Attempts               []evidenceAvailabilityAttempt `json:"attempts"`
}

func validateEvidenceAvailabilityForConfiguration(configurationID string, raw json.RawMessage) (string, error) {
	expectedPolicy := ""
	switch configurationID {
	case diagnosisEvidenceGapConfigurationID:
		expectedPolicy = "diagnosis-evidence-gap-v2"
	case treatmentEvidenceGapConfigurationID:
		expectedPolicy = "treatment-evidence-gap-v2"
	default:
		return externalEvidenceNotRequired, nil
	}
	if len(raw) == 0 || string(raw) == "null" || string(raw) == "{}" {
		return "", fmt.Errorf("%w: evidence-gap configuration requires trace", ErrEvidenceAvailabilityTraceInvalid)
	}
	var trace evidenceAvailabilityTrace
	if err := json.Unmarshal(raw, &trace); err != nil {
		return "", fmt.Errorf("%w: decode trace: %v", ErrEvidenceAvailabilityTraceInvalid, err)
	}
	if trace.TraceRevision != evidenceAvailabilityTraceV2 || trace.PolicyRevision != expectedPolicy {
		return "", fmt.Errorf(
			"%w: expected trace=%s policy=%s, got trace=%q policy=%q",
			ErrEvidenceAvailabilityTraceInvalid,
			evidenceAvailabilityTraceV2,
			expectedPolicy,
			trace.TraceRevision,
			trace.PolicyRevision,
		)
	}

	externalAttempts := 0
	returned := 0
	for index, attempt := range trace.Attempts {
		if attempt.Gap.Kind != "external_knowledge" {
			continue
		}
		externalAttempts++
		switch attempt.Status {
		case "evidence_returned":
			returned++
			if attempt.RetrievalStatus == nil || *attempt.RetrievalStatus != "results_returned" || !attempt.SearchPerformed {
				return "", fmt.Errorf("%w: attempt %d returned evidence without results_returned retrieval", ErrEvidenceAvailabilityTraceInvalid, index)
			}
		case "unresolved":
			if err := validateUnresolvedExternalAttempt(index, attempt); err != nil {
				return "", err
			}
		default:
			return "", fmt.Errorf("%w: attempt %d has unknown status %q", ErrEvidenceAvailabilityTraceInvalid, index, attempt.Status)
		}
	}

	computed := externalEvidenceNotRequired
	switch {
	case externalAttempts == 0:
		computed = externalEvidenceNotRequired
	case returned == externalAttempts:
		computed = externalEvidenceAvailable
	case returned > 0:
		computed = externalEvidencePartiallyAvailable
	default:
		computed = externalEvidenceUnresolved
	}
	if trace.ExternalEvidenceStatus != computed {
		return "", fmt.Errorf(
			"%w: external_evidence_status=%q does not match computed=%q",
			ErrEvidenceAvailabilityTraceInvalid,
			trace.ExternalEvidenceStatus,
			computed,
		)
	}
	return computed, nil
}

func validateUnresolvedExternalAttempt(index int, attempt evidenceAvailabilityAttempt) error {
	switch attempt.StopReason {
	case "published_corpus_empty":
		return requireRetrievalStatus(index, attempt, "published_corpus_empty", true)
	case "no_relevant_results":
		return requireRetrievalStatus(index, attempt, "no_relevant_results", true)
	case "search_unavailable":
		if attempt.RetrievalStatus == nil || *attempt.RetrievalStatus != "search_unavailable" {
			return fmt.Errorf("%w: attempt %d expected retrieval=search_unavailable", ErrEvidenceAvailabilityTraceInvalid, index)
		}
		return nil
	case "budget_exhausted":
		if attempt.SearchPerformed || attempt.RetrievalStatus != nil {
			return fmt.Errorf("%w: attempt %d exhausted budget but claims retrieval", ErrEvidenceAvailabilityTraceInvalid, index)
		}
		return nil
	default:
		return fmt.Errorf("%w: attempt %d has unsupported unresolved stop_reason %q", ErrEvidenceAvailabilityTraceInvalid, index, attempt.StopReason)
	}
}

func requireRetrievalStatus(index int, attempt evidenceAvailabilityAttempt, expected string, searchPerformed bool) error {
	if attempt.RetrievalStatus == nil || *attempt.RetrievalStatus != expected || attempt.SearchPerformed != searchPerformed {
		return fmt.Errorf(
			"%w: attempt %d expected retrieval=%s search_performed=%t",
			ErrEvidenceAvailabilityTraceInvalid,
			index,
			expected,
			searchPerformed,
		)
	}
	return nil
}
