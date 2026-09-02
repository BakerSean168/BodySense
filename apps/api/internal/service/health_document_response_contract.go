package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

func validateHealthDocumentResponse(respBody []byte, expectedConfigurationID string) ([]byte, error) {
	registration, ok := knownHealthDocumentConfigurations[strings.TrimSpace(expectedConfigurationID)]
	if !ok {
		return nil, fmt.Errorf("unknown health-document configuration id %q", expectedConfigurationID)
	}
	var envelope struct {
		Status string         `json:"status"`
		Result map[string]any `json:"result"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return nil, fmt.Errorf("decode health-document response: %w", err)
	}
	if envelope.Status != "completed" || envelope.Result == nil {
		return nil, errors.New("health-document response is incomplete")
	}
	mechanism, ok := envelope.Result["mechanism_provenance"].(map[string]any)
	if !ok {
		return nil, errors.New("health-document response missing mechanism_provenance")
	}
	if status, _ := mechanism["status"].(string); status != "verified" {
		return nil, fmt.Errorf("health-document mechanism status mismatch: %q", status)
	}
	if id, _ := mechanism["configuration_id"].(string); id != expectedConfigurationID {
		return nil, fmt.Errorf("health-document configuration mismatch: got %q want %q", id, expectedConfigurationID)
	}
	if registration.Legacy {
		if err := validateLegacyTesseractProvenance(mechanism, registration); err != nil {
			return nil, err
		}
	} else {
		if err := validateCurrentHealthDocumentProvenance(mechanism, registration); err != nil {
			return nil, err
		}
		if err := validateCurrentHealthDocumentIndicators(envelope.Result, registration); err != nil {
			return nil, err
		}
	}
	envelope.Result["generation_decision_trace"] = map[string]any{
		"decision":               "persist",
		"authority":              "go",
		"configuration_id":       expectedConfigurationID,
		"mechanism_revision":     registration.MechanismRevision,
		"output_schema_revision": registration.OutputSchemaRevision,
	}
	validated, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("encode validated health-document response: %w", err)
	}
	return validated, nil
}

func validateCurrentHealthDocumentProvenance(
	mechanism map[string]any,
	registration healthDocumentConfigurationRegistration,
) error {
	expected := map[string]string{
		"mechanism_revision":                  registration.MechanismRevision,
		"output_schema_revision":              registration.OutputSchemaRevision,
		"execution_topology_revision":         registration.ExecutionTopologyRevision,
		"pdf_strategy_revision":               registration.PDFStrategyRevision,
		"native_text_engine":                  registration.NativeTextEngine,
		"native_text_engine_version":          registration.NativeTextEngineVersion,
		"native_text_quality_policy_revision": registration.NativeTextQualityPolicyRevision,
		"native_text_quality_policy_sha256":   registration.NativeTextQualityPolicySHA256,
		"ocr_engine":                          registration.OCREngine,
		"ocr_engine_version":                  registration.OCREngineVersion,
		"runtime_engine":                      registration.RuntimeEngine,
		"runtime_version":                     registration.RuntimeVersion,
		"model_family":                        registration.ModelFamily,
		"model_type":                          registration.ModelType,
		"detector_limit_type":                 registration.DetectorLimitType,
		"indicator_parser_revision":           registration.IndicatorParserRevision,
		"indicator_parser_sha256":             registration.IndicatorParserSHA256,
		"admissibility_policy_revision":       registration.AdmissibilityPolicyRevision,
		"admissibility_policy_sha256":         registration.AdmissibilityPolicySHA256,
		"engine_adapter_sha256":               registration.EngineAdapterSHA256,
		"worker_sha256":                       registration.WorkerSHA256,
		"verification_revision":               registration.VerificationRevision,
		"verifier_engine":                     registration.VerifierEngine,
		"verifier_engine_version":             registration.VerifierEngineVersion,
		"verifier_strategy_revision":          registration.VerifierStrategyRevision,
		"verifier_adapter_sha256":             registration.VerifierAdapterSHA256,
		"verifier_worker_sha256":              registration.VerifierWorkerSHA256,
		"verification_policy_sha256":          registration.VerificationPolicySHA256,
		"orchestrator_sha256":                 registration.OrchestratorSHA256,
	}
	for field, want := range expected {
		got, _ := mechanism[field].(string)
		if got != want {
			return fmt.Errorf("health-document mechanism %s mismatch: got %q want %q", field, got, want)
		}
	}
	if got := intFromJSON(mechanism["pdf_raster_dpi"]); got != registration.PDFRasterDPI {
		return fmt.Errorf("health-document pdf_raster_dpi mismatch: got %d want %d", got, registration.PDFRasterDPI)
	}
	if got := intFromJSON(mechanism["detector_limit_side_len"]); got != registration.DetectorLimitSideLen {
		return fmt.Errorf("health-document detector_limit_side_len mismatch: got %d want %d", got, registration.DetectorLimitSideLen)
	}
	if registration.VerificationRevision != "" {
		if got := intFromJSON(mechanism["verifier_psm"]); got != registration.VerifierPSM {
			return fmt.Errorf("health-document verifier_psm mismatch: got %d want %d", got, registration.VerifierPSM)
		}
		if err := validateStringList(mechanism["verifier_languages"], registration.VerifierLanguages, "verifier_languages"); err != nil {
			return err
		}
		if err := validateHealthDocumentVerifierArtifacts(mechanism["verifier_language_artifacts"], registration.VerifierLanguageArtifacts); err != nil {
			return err
		}
	}
	intFields := map[string]int{
		"global_max_side_len":      registration.GlobalMaxSideLen,
		"rec_batch_num":            registration.RecBatchNum,
		"cls_batch_num":            registration.ClsBatchNum,
		"ort_intra_op_num_threads": registration.ORTIntraOpNumThreads,
		"ort_inter_op_num_threads": registration.ORTInterOpNumThreads,
	}
	for field, want := range intFields {
		if got := intFromJSON(mechanism[field]); got != want {
			return fmt.Errorf("health-document mechanism %s mismatch: got %d want %d", field, got, want)
		}
	}
	return validateHealthDocumentModelArtifacts(mechanism["model_artifacts"], registration.ModelArtifacts)
}

func validateLegacyTesseractProvenance(
	mechanism map[string]any,
	registration healthDocumentConfigurationRegistration,
) error {
	expected := map[string]string{
		"mechanism_revision":            registration.MechanismRevision,
		"execution_topology_revision":   registration.ExecutionTopologyRevision,
		"pdf_strategy_revision":         registration.PDFStrategyRevision,
		"engine":                        registration.OCREngine,
		"engine_version":                registration.OCREngineVersion,
		"wrapper":                       registration.Wrapper,
		"wrapper_version":               registration.WrapperVersion,
		"indicator_parser_revision":     registration.IndicatorParserRevision,
		"indicator_parser_sha256":       registration.IndicatorParserSHA256,
		"admissibility_policy_revision": registration.AdmissibilityPolicyRevision,
		"ocr_service_sha256":            registration.OCRServiceSHA256,
	}
	for field, want := range expected {
		got, _ := mechanism[field].(string)
		if got != want {
			return fmt.Errorf("legacy health-document mechanism %s mismatch: got %q want %q", field, got, want)
		}
	}
	if got := intFromJSON(mechanism["pdf_raster_dpi"]); got != registration.PDFRasterDPI {
		return fmt.Errorf("legacy health-document pdf_raster_dpi mismatch: got %d want %d", got, registration.PDFRasterDPI)
	}
	languages, ok := mechanism["languages"].([]any)
	if !ok {
		return errors.New("legacy health-document provenance missing languages")
	}
	gotLanguages := make([]string, 0, len(languages))
	for _, value := range languages {
		if text, ok := value.(string); ok {
			gotLanguages = append(gotLanguages, text)
		}
	}
	if !slices.Equal(gotLanguages, registration.Languages) {
		return fmt.Errorf("legacy health-document languages mismatch: got %v want %v", gotLanguages, registration.Languages)
	}
	return nil
}

func validateStringList(value any, expected []string, field string) error {
	items, ok := value.([]any)
	if !ok || len(items) != len(expected) {
		return fmt.Errorf("health-document %s mismatch", field)
	}
	got := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			return fmt.Errorf("health-document %s is malformed", field)
		}
		got = append(got, text)
	}
	if !slices.Equal(got, expected) {
		return fmt.Errorf("health-document %s mismatch: got %v want %v", field, got, expected)
	}
	return nil
}

func validateHealthDocumentVerifierArtifacts(value any, expected []healthDocumentVerifierArtifact) error {
	items, ok := value.([]any)
	if !ok || len(items) != len(expected) {
		return errors.New("health-document verifier_language_artifacts mismatch")
	}
	byLanguage := make(map[string]map[string]any, len(items))
	for _, item := range items {
		artifact, ok := item.(map[string]any)
		if !ok {
			return errors.New("health-document verifier language artifact is malformed")
		}
		language, _ := artifact["language"].(string)
		byLanguage[language] = artifact
	}
	for _, want := range expected {
		artifact, ok := byLanguage[want.Language]
		if !ok {
			return fmt.Errorf("health-document verifier language artifact %q missing", want.Language)
		}
		if got, _ := artifact["sha256"].(string); got != want.SHA256 {
			return fmt.Errorf("health-document verifier language %s sha256 mismatch", want.Language)
		}
	}
	return nil
}

func validateHealthDocumentModelArtifacts(value any, expected []healthDocumentModelArtifact) error {
	items, ok := value.([]any)
	if !ok || len(items) != len(expected) {
		return errors.New("health-document model_artifacts mismatch")
	}
	byRole := make(map[string]map[string]any, len(items))
	for _, item := range items {
		artifact, ok := item.(map[string]any)
		if !ok {
			return errors.New("health-document model artifact is malformed")
		}
		role, _ := artifact["role"].(string)
		byRole[role] = artifact
	}
	for _, want := range expected {
		artifact, ok := byRole[want.Role]
		if !ok {
			return fmt.Errorf("health-document model artifact %q missing", want.Role)
		}
		if got, _ := artifact["filename"].(string); got != want.Filename {
			return fmt.Errorf("health-document model %s filename mismatch", want.Role)
		}
		if got, _ := artifact["sha256"].(string); got != want.SHA256 {
			return fmt.Errorf("health-document model %s sha256 mismatch", want.Role)
		}
	}
	return nil
}

func validateCurrentHealthDocumentIndicators(
	result map[string]any,
	registration healthDocumentConfigurationRegistration,
) error {
	sourceItems, ok := result["source_blocks"].([]any)
	if !ok {
		return errors.New("health-document response missing source_blocks")
	}
	sourceRefs := make(map[string]struct{}, len(sourceItems))
	for _, value := range sourceItems {
		block, ok := value.(map[string]any)
		if !ok {
			return errors.New("health-document source block is malformed")
		}
		ref, _ := block["source_ref"].(string)
		if strings.TrimSpace(ref) == "" {
			return errors.New("health-document source block is missing source_ref")
		}
		sourceRefs[ref] = struct{}{}
	}

	items, ok := result["indicators"].([]any)
	if !ok {
		return errors.New("health-document response missing indicators")
	}
	for _, value := range items {
		indicator, ok := value.(map[string]any)
		if !ok {
			return errors.New("health-document indicator is malformed")
		}
		if parser, _ := indicator["parser_revision"].(string); parser != registration.IndicatorParserRevision {
			return fmt.Errorf("health-document indicator parser revision mismatch: %q", parser)
		}
		refs, ok := indicator["source_refs"].([]any)
		if !ok || len(refs) == 0 {
			return errors.New("health-document current indicator is missing source_refs")
		}
		for _, value := range refs {
			ref, ok := value.(string)
			if !ok || strings.TrimSpace(ref) == "" {
				return errors.New("health-document indicator source_ref is malformed")
			}
			if _, exists := sourceRefs[ref]; !exists {
				return fmt.Errorf("health-document indicator source_ref %q is not present in source_blocks", ref)
			}
		}
		admission, ok := indicator["evidence_admissibility"].(map[string]any)
		if !ok {
			return errors.New("health-document indicator missing evidence_admissibility")
		}
		if policy, _ := admission["policy_revision"].(string); policy != registration.AdmissibilityPolicyRevision {
			return fmt.Errorf("health-document admissibility policy mismatch: %q", policy)
		}
		if registration.VerificationRevision != "" {
			verification, ok := indicator["evidence_verification"].(map[string]any)
			if !ok {
				return errors.New("health-document indicator missing evidence_verification")
			}
			if revision, _ := verification["revision"].(string); revision != registration.VerificationRevision {
				return fmt.Errorf("health-document verification revision mismatch: %q", revision)
			}
		}
	}
	return nil
}

func intFromJSON(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	default:
		return -1
	}
}
