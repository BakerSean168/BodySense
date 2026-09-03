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

func TestAssessmentEvidenceV3RequiresReportIndicatorAdmissibility(t *testing.T) {
	req := reportIndicatorRequest(`[{
		"upload_id":"11111111-1111-1111-1111-111111111111",
		"indicator_index":0,
		"value":{"name":"Vitamin D","value":"25.3","unit":"ng/mL"}
	}]`)
	ref := "report:upload:11111111-1111-1111-1111-111111111111:indicator:0"

	legacy := buildAssessmentEvidenceCatalog(req, assessmentEvidencePolicyV2)
	if _, ok := legacy[ref]; !ok {
		t.Fatalf("historical evidence-v2 must keep legacy report indicator replayable: %#v", legacy)
	}

	current := buildAssessmentEvidenceCatalog(req, assessmentEvidencePolicyV3)
	if _, ok := current[ref]; ok {
		t.Fatalf("evidence-v3 must fail closed when admissibility provenance is absent: %#v", current)
	}
}

func TestAssessmentEvidenceV3ExcludesReviewRequiredAndAcceptsAdmissibleReportIndicator(t *testing.T) {
	req := reportIndicatorRequest(`[{
		"upload_id":"22222222-2222-2222-2222-222222222222",
		"indicator_index":0,
		"value":{
			"name":"Vitamin D","value":"25.3","unit":"ng/mL",
			"evidence_admissibility":{
				"status":"needs_review",
				"policy_revision":"ocr-indicator-admissibility-v1",
				"reason_codes":["indicator_confidence_medium"]
			}
		}
	},{
		"upload_id":"22222222-2222-2222-2222-222222222222",
		"indicator_index":1,
		"value":{
			"name":"Ferritin","value":"50","unit":"ng/mL",
			"evidence_admissibility":{
				"status":"admissible",
				"policy_revision":"ocr-indicator-admissibility-v1",
				"reason_codes":["high_confidence_ocr_and_indicator"]
			}
		}
	}]`)
	catalog := buildAssessmentEvidenceCatalog(req, assessmentEvidencePolicyV3)
	if _, ok := catalog["report:upload:22222222-2222-2222-2222-222222222222:indicator:0"]; ok {
		t.Fatalf("review-required report indicator must not enter current catalog: %#v", catalog)
	}
	if _, ok := catalog["report:upload:22222222-2222-2222-2222-222222222222:indicator:1"]; !ok {
		t.Fatalf("admissible report indicator must enter current catalog: %#v", catalog)
	}
}

func TestAssessmentEvidenceV3RejectsForgedAdmissibilityPolicyRevision(t *testing.T) {
	req := reportIndicatorRequest(`[{
		"value":{
			"name":"Vitamin D","value":"25.3","unit":"ng/mL",
			"evidence_admissibility":{
				"status":"admissible",
				"policy_revision":"made-up-policy"
			}
		}
	}]`)
	if catalog := buildAssessmentEvidenceCatalog(req, assessmentEvidencePolicyV3); len(catalog) != 0 {
		t.Fatalf("forged admissibility policy must fail closed: %#v", catalog)
	}
}

func TestAssessmentUploadOCRAdmissibilitySurvivesInputAssemblyAndGatesDurableEvidence(t *testing.T) {
	uploadID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	uploads := []model.UserUpload{{
		ID:        uploadID,
		FileType:  "report",
		OCRStatus: "completed",
		OCRResult: json.RawMessage(`{
			"status":"completed",
			"result":{
				"confidence":"medium",
				"indicators":[{
					"name":"Vitamin D","value":"25.3","unit":"ng/mL","confidence":"high",
					"evidence_admissibility":{
						"status":"needs_review",
						"policy_revision":"ocr-indicator-admissibility-v1",
						"reason_codes":["ocr_confidence_medium"]
					}
				}]
			}
		}`),
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
	}, assessmentEvidencePolicyV3)
	ref := "report:upload:33333333-3333-3333-3333-333333333333:indicator:0"
	if _, ok := catalog[ref]; ok {
		t.Fatalf("review-required indicator must not become durable Assessment evidence: %#v", catalog)
	}
}

func TestAssessmentMachineAdmissibleLaneKeepsWorkingUnderV4(t *testing.T) {
	// The machine-admissible Champion lane (ocr-indicator-admissibility-v1 +
	// admissible) continues to work under evidence-contract-v4 alongside the new
	// reviewed lane, and admissibility-v2 does not become production authority.
	req := reportIndicatorRequest(`[{
		"upload_id":"44444444-4444-4444-4444-444444444444",
		"indicator_index":0,
		"value":{
			"name":"Vitamin D","value":"25.3","unit":"ng/mL",
			"evidence_admissibility":{
				"status":"admissible",
				"policy_revision":"ocr-indicator-admissibility-v1",
				"reason_codes":["high_confidence_ocr_and_indicator"]
			}
		}
	}]`)
	catalog := buildAssessmentEvidenceCatalog(req, assessmentEvidencePolicyV4)
	if _, ok := catalog["report:upload:44444444-4444-4444-4444-444444444444:indicator:0"]; !ok {
		t.Fatalf("v4 must keep the machine-admissible Champion lane: %#v", catalog)
	}
	// A forged/unknown admissibility (e.g. admissibility-v2) still fails closed.
	req = reportIndicatorRequest(`[{
		"indicator_index":0,
		"value":{
			"name":"Vitamin D","value":"25.3","unit":"ng/mL",
			"evidence_admissibility":{"status":"admissible","policy_revision":"ocr-indicator-admissibility-v2"}
		}
	}]`)
	if catalog := buildAssessmentEvidenceCatalog(req, assessmentEvidencePolicyV4); len(catalog) != 0 {
		t.Fatalf("admissibility-v2 must not become production authority under v4: %#v", catalog)
	}
}
