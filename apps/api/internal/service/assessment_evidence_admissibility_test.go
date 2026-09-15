package service

import (
	"encoding/json"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

func reportIndicatorRequest(value string) AssessmentGenerationRequest {
	return AssessmentGenerationRequest{
		BodyState:        json.RawMessage(`{}`),
		PostureAnalysis:  json.RawMessage(`{}`),
		ReportIndicators: json.RawMessage(value),
	}
}

func TestAssessmentEvidenceRequiresReportIndicatorAdmissibility(t *testing.T) {
	req := reportIndicatorRequest(`[{"upload_id":"11111111-1111-1111-1111-111111111111","indicator_index":0,"value":{"name":"Vitamin D","value":"25.3","unit":"ng/mL"}}]`)
	ref := "report:upload:11111111-1111-1111-1111-111111111111:indicator:0"
	if catalog := buildAssessmentEvidenceCatalog(req); catalog[ref].Source != "" {
		t.Fatalf("report indicator without admissibility provenance must fail closed: %#v", catalog)
	}
}

func TestAssessmentEvidenceExcludesReviewRequiredAndAcceptsAdmissibleReportIndicator(t *testing.T) {
	req := reportIndicatorRequest(`[
		{"upload_id":"22222222-2222-2222-2222-222222222222","indicator_index":0,"value":{"name":"Vitamin D","value":"25.3","unit":"ng/mL","evidence_admissibility":{"status":"needs_review","policy_revision":"ocr-indicator-admissibility-v1","reason_codes":["indicator_confidence_medium"]}}},
		{"upload_id":"22222222-2222-2222-2222-222222222222","indicator_index":1,"value":{"name":"Ferritin","value":"50","unit":"ng/mL","evidence_admissibility":{"status":"admissible","policy_revision":"ocr-indicator-admissibility-v1","reason_codes":["high_confidence_ocr_and_indicator"]}}}
	]`)
	catalog := buildAssessmentEvidenceCatalog(req)
	if _, ok := catalog["report:upload:22222222-2222-2222-2222-222222222222:indicator:0"]; ok {
		t.Fatalf("review-required report indicator must not enter current catalog: %#v", catalog)
	}
	if _, ok := catalog["report:upload:22222222-2222-2222-2222-222222222222:indicator:1"]; !ok {
		t.Fatalf("admissible report indicator must enter current catalog: %#v", catalog)
	}
}

func TestAssessmentEvidenceRejectsForgedAdmissibilityPolicyRevision(t *testing.T) {
	req := reportIndicatorRequest(`[{"value":{"name":"Vitamin D","value":"25.3","unit":"ng/mL","evidence_admissibility":{"status":"admissible","policy_revision":"made-up-policy"}}}]`)
	if catalog := buildAssessmentEvidenceCatalog(req); len(catalog) != 0 {
		t.Fatalf("forged admissibility policy must fail closed: %#v", catalog)
	}
}

func TestAssessmentUploadOCRAdmissibilitySurvivesInputAssemblyAndGatesDurableEvidence(t *testing.T) {
	uploadID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	uploads := []model.UserUpload{{
		ID:        uploadID,
		FileType:  "report",
		OCRStatus: "completed",
		OCRResult: json.RawMessage(`{"status":"completed","result":{"confidence":"medium","indicators":[{"name":"Vitamin D","value":"25.3","unit":"ng/mL","confidence":"high","evidence_admissibility":{"status":"needs_review","policy_revision":"ocr-indicator-admissibility-v1","reason_codes":["ocr_confidence_medium"]}}]}}`),
	}}

	indicators, posture := assessmentInputsFromUploads(uploads)
	if len(posture) != 0 || len(indicators) != 1 {
		t.Fatalf("completed OCR indicator must remain available for review/transport: indicators=%#v posture=%#v", indicators, posture)
	}
	encoded, err := json.Marshal(indicators)
	if err != nil {
		t.Fatal(err)
	}
	catalog := buildAssessmentEvidenceCatalog(AssessmentGenerationRequest{
		BodyState:        json.RawMessage(`{}`),
		PostureAnalysis:  json.RawMessage(`{}`),
		ReportIndicators: encoded,
	})
	ref := "report:upload:33333333-3333-3333-3333-333333333333:indicator:0"
	if _, ok := catalog[ref]; ok {
		t.Fatalf("review-required indicator must not become durable Assessment evidence: %#v", catalog)
	}
}

func TestAssessmentMachineAdmissibleLaneRejectsUnknownPolicy(t *testing.T) {
	req := reportIndicatorRequest(`[{"upload_id":"44444444-4444-4444-4444-444444444444","indicator_index":0,"value":{"name":"Vitamin D","value":"25.3","unit":"ng/mL","evidence_admissibility":{"status":"admissible","policy_revision":"ocr-indicator-admissibility-v1","reason_codes":["high_confidence_ocr_and_indicator"]}}}]`)
	catalog := buildAssessmentEvidenceCatalog(req)
	if _, ok := catalog["report:upload:44444444-4444-4444-4444-444444444444:indicator:0"]; !ok {
		t.Fatalf("current contract must keep the machine-admissible lane: %#v", catalog)
	}

	req = reportIndicatorRequest(`[{"indicator_index":0,"value":{"name":"Vitamin D","value":"25.3","unit":"ng/mL","evidence_admissibility":{"status":"admissible","policy_revision":"ocr-indicator-admissibility-v2"}}}]`)
	if catalog := buildAssessmentEvidenceCatalog(req); len(catalog) != 0 {
		t.Fatalf("unknown admissibility policy must not become production authority: %#v", catalog)
	}
}
