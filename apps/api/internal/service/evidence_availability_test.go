package service

import (
	"encoding/json"
	"errors"
	"testing"
)

func evidenceAvailabilityJSON(policy, externalStatus, attempts string) json.RawMessage {
	return json.RawMessage(`{
		"trace_revision":"evidence-acquisition-trace-v2",
		"policy_revision":"` + policy + `",
		"external_evidence_status":"` + externalStatus + `",
		"attempts":` + attempts + `
	}`)
}

func TestEvidenceAvailabilityTraceDistinguishesNoEvidenceNeededFromUnresolved(t *testing.T) {
	status, err := validateEvidenceAvailabilityForConfiguration(
		diagnosisEvidenceGapConfigurationID,
		evidenceAvailabilityJSON("diagnosis-evidence-gap-v2", "not_required", `[
			{"gap":{"kind":"user_fact"},"status":"unresolved","stop_reason":"user_input_required","search_performed":false}
		]`),
	)
	if err != nil || status != externalEvidenceNotRequired {
		t.Fatalf("not-required status=%q err=%v", status, err)
	}

	status, err = validateEvidenceAvailabilityForConfiguration(
		diagnosisEvidenceGapConfigurationID,
		evidenceAvailabilityJSON("diagnosis-evidence-gap-v2", "unresolved", `[
			{"gap":{"kind":"external_knowledge"},"status":"unresolved","stop_reason":"published_corpus_empty","search_performed":true,"retrieval_status":"published_corpus_empty"}
		]`),
	)
	if err != nil || status != externalEvidenceUnresolved {
		t.Fatalf("unresolved status=%q err=%v", status, err)
	}
}

func TestEvidenceAvailabilityTraceRecomputesAvailableAndPartial(t *testing.T) {
	available := `[
		{"gap":{"kind":"external_knowledge"},"status":"evidence_returned","stop_reason":"evidence_returned","search_performed":true,"retrieval_status":"results_returned"}
	]`
	status, err := validateEvidenceAvailabilityForConfiguration(
		treatmentEvidenceGapConfigurationID,
		evidenceAvailabilityJSON("treatment-evidence-gap-v2", "available", available),
	)
	if err != nil || status != externalEvidenceAvailable {
		t.Fatalf("available status=%q err=%v", status, err)
	}

	partial := `[
		{"gap":{"kind":"external_knowledge"},"status":"evidence_returned","stop_reason":"evidence_returned","search_performed":true,"retrieval_status":"results_returned"},
		{"gap":{"kind":"external_knowledge"},"status":"unresolved","stop_reason":"no_relevant_results","search_performed":true,"retrieval_status":"no_relevant_results"}
	]`
	status, err = validateEvidenceAvailabilityForConfiguration(
		treatmentEvidenceGapConfigurationID,
		evidenceAvailabilityJSON("treatment-evidence-gap-v2", "partially_available", partial),
	)
	if err != nil || status != externalEvidencePartiallyAvailable {
		t.Fatalf("partial status=%q err=%v", status, err)
	}
}

func TestEvidenceAvailabilityTraceRejectsSelfReportedStatusDrift(t *testing.T) {
	_, err := validateEvidenceAvailabilityForConfiguration(
		diagnosisEvidenceGapConfigurationID,
		evidenceAvailabilityJSON("diagnosis-evidence-gap-v2", "available", `[
			{"gap":{"kind":"external_knowledge"},"status":"unresolved","stop_reason":"search_unavailable","search_performed":false,"retrieval_status":"search_unavailable"}
		]`),
	)
	if !errors.Is(err, ErrEvidenceAvailabilityTraceInvalid) {
		t.Fatalf("expected drift rejection, got %v", err)
	}
}

func TestEvidenceAvailabilityTraceRejectsLegacyOrInconsistentRetrieval(t *testing.T) {
	cases := []json.RawMessage{
		evidenceAvailabilityJSON("diagnosis-evidence-gap-v2", "unresolved", `[
			{"gap":{"kind":"external_knowledge"},"status":"unresolved","stop_reason":"no_results","search_performed":true}
		]`),
		evidenceAvailabilityJSON("diagnosis-evidence-gap-v2", "unresolved", `[
			{"gap":{"kind":"external_knowledge"},"status":"unresolved","stop_reason":"search_unavailable","search_performed":true}
		]`),
	}
	for _, raw := range cases {
		if _, err := validateEvidenceAvailabilityForConfiguration(diagnosisEvidenceGapConfigurationID, raw); !errors.Is(err, ErrEvidenceAvailabilityTraceInvalid) {
			t.Fatalf("expected invalid retrieval contract, raw=%s err=%v", raw, err)
		}
	}
}

func TestEvidenceAvailabilityTraceDoesNotAffectLegacyConfigurations(t *testing.T) {
	status, err := validateEvidenceAvailabilityForConfiguration(defaultDiagnosisConfigurationID, nil)
	if err != nil || status != externalEvidenceNotRequired {
		t.Fatalf("legacy status=%q err=%v", status, err)
	}
}
