package service

import (
	"encoding/json"
	"strings"
	"testing"
)

func currentHealthDocumentResponseBody(t *testing.T) []byte {
	t.Helper()
	reg := knownHealthDocumentConfigurations[healthDocumentCandidateConfigurationID]
	artifacts := make([]map[string]any, 0, len(reg.ModelArtifacts))
	for _, item := range reg.ModelArtifacts {
		artifacts = append(artifacts, map[string]any{"role": item.Role, "filename": item.Filename, "sha256": item.SHA256})
	}
	verifierArtifacts := make([]map[string]any, 0, len(reg.VerifierLanguageArtifacts))
	for _, item := range reg.VerifierLanguageArtifacts {
		verifierArtifacts = append(verifierArtifacts, map[string]any{"language": item.Language, "sha256": item.SHA256})
	}
	mechanism := map[string]any{
		"status": "verified", "configuration_id": healthDocumentCandidateConfigurationID,
		"mechanism_revision": reg.MechanismRevision, "output_schema_revision": reg.OutputSchemaRevision,
		"execution_topology_revision": reg.ExecutionTopologyRevision, "pdf_strategy_revision": reg.PDFStrategyRevision,
		"native_text_engine": reg.NativeTextEngine, "native_text_engine_version": reg.NativeTextEngineVersion,
		"native_text_quality_policy_revision": reg.NativeTextQualityPolicyRevision,
		"native_text_quality_policy_sha256":   reg.NativeTextQualityPolicySHA256,
		"ocr_engine":                          reg.OCREngine, "ocr_engine_version": reg.OCREngineVersion,
		"runtime_engine": reg.RuntimeEngine, "runtime_version": reg.RuntimeVersion,
		"model_family": reg.ModelFamily, "model_type": reg.ModelType, "model_artifacts": artifacts,
		"pdf_raster_dpi": reg.PDFRasterDPI, "detector_limit_type": reg.DetectorLimitType,
		"detector_limit_side_len": reg.DetectorLimitSideLen,
		"global_max_side_len":     reg.GlobalMaxSideLen,
		"rec_batch_num":           reg.RecBatchNum, "cls_batch_num": reg.ClsBatchNum,
		"ort_intra_op_num_threads":  reg.ORTIntraOpNumThreads,
		"ort_inter_op_num_threads":  reg.ORTInterOpNumThreads,
		"indicator_parser_revision": reg.IndicatorParserRevision, "indicator_parser_sha256": reg.IndicatorParserSHA256,
		"admissibility_policy_revision": reg.AdmissibilityPolicyRevision,
		"admissibility_policy_sha256":   reg.AdmissibilityPolicySHA256,
		"engine_adapter_sha256":         reg.EngineAdapterSHA256, "worker_sha256": reg.WorkerSHA256,
		"verification_revision": reg.VerificationRevision, "verifier_engine": reg.VerifierEngine,
		"verifier_engine_version": reg.VerifierEngineVersion, "verifier_languages": []any{"chi_sim", "eng"},
		"verifier_language_artifacts": verifierArtifacts, "verifier_psm": reg.VerifierPSM,
		"verifier_strategy_revision": reg.VerifierStrategyRevision,
		"verifier_adapter_sha256":    reg.VerifierAdapterSHA256, "verifier_worker_sha256": reg.VerifierWorkerSHA256,
		"verification_policy_sha256": reg.VerificationPolicySHA256, "orchestrator_sha256": reg.OrchestratorSHA256,
	}
	body := map[string]any{"status": "completed", "result": map[string]any{
		"raw_text": "private report text",
		"source_blocks": []any{map[string]any{
			"source_ref": "page:1:ocr-block:1",
		}},
		"indicators": []any{map[string]any{
			"indicator_id": "hemoglobin", "name": "血红蛋白", "value": "142", "unit": "g/L",
			"parser_revision": reg.IndicatorParserRevision, "source_refs": []any{"page:1:ocr-block:1"},
			"evidence_admissibility": map[string]any{"status": "admissible", "policy_revision": reg.AdmissibilityPolicyRevision},
			"evidence_verification":  map[string]any{"status": "verified_consensus", "revision": reg.VerificationRevision},
		}},
		"mechanism_provenance": mechanism,
	}}
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func legacyHealthDocumentResponseBody(t *testing.T) []byte {
	t.Helper()
	reg := knownHealthDocumentConfigurations[legacyTesseractConfigurationID]
	body := map[string]any{"status": "completed", "result": map[string]any{
		"raw_text":   "legacy",
		"indicators": []any{},
		"mechanism_provenance": map[string]any{
			"status": "verified", "configuration_id": legacyTesseractConfigurationID,
			"mechanism_revision": reg.MechanismRevision, "execution_topology_revision": reg.ExecutionTopologyRevision,
			"engine": reg.OCREngine, "engine_version": reg.OCREngineVersion,
			"wrapper": reg.Wrapper, "wrapper_version": reg.WrapperVersion,
			"languages": []any{"chi_sim", "eng"}, "pdf_strategy_revision": reg.PDFStrategyRevision,
			"pdf_raster_dpi": reg.PDFRasterDPI, "indicator_parser_revision": reg.IndicatorParserRevision,
			"indicator_parser_sha256":       reg.IndicatorParserSHA256,
			"admissibility_policy_revision": reg.AdmissibilityPolicyRevision,
			"ocr_service_sha256":            reg.OCRServiceSHA256, "worker_sha256": strings.Repeat("a", 64),
		},
	}}
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func TestValidateCurrentHealthDocumentResponseAddsGoDecisionTrace(t *testing.T) {
	validated, err := validateHealthDocumentResponse(currentHealthDocumentResponseBody(t), healthDocumentCandidateConfigurationID)
	if err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(validated, &envelope); err != nil {
		t.Fatal(err)
	}
	result := envelope["result"].(map[string]any)
	trace := result["generation_decision_trace"].(map[string]any)
	if trace["authority"] != "go" || trace["decision"] != "persist" {
		t.Fatalf("unexpected decision trace: %#v", trace)
	}
}

func TestValidateCurrentHealthDocumentResponseRejectsProvenanceDrift(t *testing.T) {
	body := currentHealthDocumentResponseBody(t)
	var envelope map[string]any
	_ = json.Unmarshal(body, &envelope)
	result := envelope["result"].(map[string]any)
	mechanism := result["mechanism_provenance"].(map[string]any)
	mechanism["engine_adapter_sha256"] = strings.Repeat("0", 64)
	body, _ = json.Marshal(envelope)
	if _, err := validateHealthDocumentResponse(body, healthDocumentCandidateConfigurationID); err == nil {
		t.Fatal("expected mechanism hash mismatch")
	}
}

func TestValidateCurrentHealthDocumentResponseRejectsUngroundedIndicator(t *testing.T) {
	body := currentHealthDocumentResponseBody(t)
	var envelope map[string]any
	_ = json.Unmarshal(body, &envelope)
	result := envelope["result"].(map[string]any)
	indicator := result["indicators"].([]any)[0].(map[string]any)
	indicator["source_refs"] = []any{}
	body, _ = json.Marshal(envelope)
	if _, err := validateHealthDocumentResponse(body, healthDocumentCandidateConfigurationID); err == nil {
		t.Fatal("current indicator without source ref must fail closed")
	}
}

func TestValidateLegacyTesseractResponseSupportsDurableRecovery(t *testing.T) {
	if _, err := validateHealthDocumentResponse(legacyHealthDocumentResponseBody(t), legacyTesseractConfigurationID); err != nil {
		t.Fatal(err)
	}
}

func TestBuildDocumentExtractionRunStoresHashesAndStructuredSnapshotOnly(t *testing.T) {
	reg := knownHealthDocumentConfigurations[healthDocumentCandidateConfigurationID]
	body, err := validateHealthDocumentResponse(currentHealthDocumentResponseBody(t), healthDocumentCandidateConfigurationID)
	if err != nil {
		t.Fatal(err)
	}
	uploadID := mustParseUUID("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	userID := mustParseUUID("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	jobID := mustParseUUID("cccccccc-cccc-cccc-cccc-cccccccccccc")
	run, err := buildDocumentExtractionRun(
		body,
		strings.Repeat("d", 64),
		uploadID,
		userID,
		jobID,
		healthDocumentCandidateConfigurationID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if run.ConfigurationID != healthDocumentCandidateConfigurationID || run.MechanismRevision != reg.MechanismRevision {
		t.Fatalf("unexpected run identity: %+v", run)
	}
	if len(run.ResultSHA256) != 64 || len(run.RawTextSHA256) != 64 || run.DocumentSHA256 != strings.Repeat("d", 64) {
		t.Fatalf("missing durable hashes: %+v", run)
	}
	if strings.Contains(string(run.SourceSummary), "private report text") || strings.Contains(string(run.MechanismProvenance), "private report text") {
		t.Fatal("raw report text leaked into privacy-bounded audit metadata")
	}
	if !strings.Contains(string(run.IndicatorSnapshot), "hemoglobin") {
		t.Fatalf("indicator snapshot missing structured candidate: %s", run.IndicatorSnapshot)
	}
}

func TestValidateCurrentHealthDocumentResponseRejectsUnknownSourceRef(t *testing.T) {
	body := currentHealthDocumentResponseBody(t)
	var envelope map[string]any
	_ = json.Unmarshal(body, &envelope)
	result := envelope["result"].(map[string]any)
	indicator := result["indicators"].([]any)[0].(map[string]any)
	indicator["source_refs"] = []any{"page:1:ocr-block:999"}
	body, _ = json.Marshal(envelope)
	if _, err := validateHealthDocumentResponse(body, healthDocumentCandidateConfigurationID); err == nil {
		t.Fatal("indicator source ref not present in source_blocks must fail closed")
	}
}
