package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

func reviewedTestReview(
	action string,
	uploadID, runID, reviewID uuid.UUID,
	index int,
	machine, reviewed json.RawMessage,
	created time.Time,
) model.DocumentIndicatorReview {
	return model.DocumentIndicatorReview{
		ID:               reviewID,
		UserID:           uuid.New(),
		UploadID:         uploadID,
		ExtractionRunID:  runID,
		IndicatorIndex:   index,
		Action:           action,
		ReviewedPayload:  datatypes.JSON(reviewed),
		MachineCandidate: datatypes.JSON(machine),
		SourceRefs:       datatypes.JSON(`["src:a"]`),
		PageRef:          datatypes.JSON(`{"src:a":{"page_number":1,"bbox":[0,0,1,1]}}`),
		ReviewerUserID:   uuid.New(),
		CreatedAt:        created,
	}
}

func mustAssessmentJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	encoded, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func TestAssessmentReviewedEvidenceRequiresExactProvenance(t *testing.T) {
	uploadID := uuid.New()
	runID := uuid.New()
	reviewID := uuid.New()
	corrected := json.RawMessage(`{"indicator_id":"ferritin","name":"Ferritin","value":"50.0","unit":"ng/mL"}`)
	row := reviewedTestReview(
		string(model.ReviewActionCorrect), uploadID, runID, reviewID, 0,
		json.RawMessage(`{"indicator_id":"ferritin","name":"Ferritin","value":"55.0","unit":"ng/mL"}`),
		corrected, time.Now(),
	)
	entry, ok := buildReviewedCandidate(row)
	if !ok {
		t.Fatalf("a correct review must produce a reviewed candidate")
	}
	if v, _ := entry["action"].(string); v != string(model.ReviewActionCorrect) {
		t.Fatalf("action=%q want correct", v)
	}
	if v, _ := entry["review_id"].(string); v != reviewID.String() {
		t.Fatalf("review_id=%q want %q", v, reviewID)
	}
	if entry["indicator_index"] != 0 {
		t.Fatalf("indicator_index=%v", entry["indicator_index"])
	}
	if v, _ := entry["indicator_id"].(string); v != "ferritin" {
		t.Fatalf("indicator_id=%q", v)
	}
	if refs, ok := entry["source_refs"].([]string); !ok || len(refs) != 1 || refs[0] != "src:a" {
		t.Fatalf("source_refs=%#v want exact source provenance", entry["source_refs"])
	}
	if page, ok := entry["page_ref"].(map[string]any); !ok || len(page) != 1 {
		t.Fatalf("page_ref=%#v want privacy-reduced page provenance", entry["page_ref"])
	}

	req := AssessmentGenerationRequest{
		BodyState:              json.RawMessage(`{}`),
		PostureAnalysis:        json.RawMessage(`{}`),
		ReportIndicators:       json.RawMessage(`[]`),
		ReviewedReportEvidence: mustAssessmentJSON(t, []any{entry}),
	}
	catalog := buildAssessmentEvidenceCatalog(req, assessmentEvidencePolicyV4)
	ref := "report:upload:" + uploadID.String() + ":indicator:0"
	if _, ok := catalog[ref]; !ok {
		t.Fatalf("v4 must admit a reviewed indicator with exact provenance: %#v", catalog)
	}

	// The same entry must NOT satisfy the v3 machine-only catalog.
	if len(buildAssessmentEvidenceCatalog(req, assessmentEvidencePolicyV3)) != 0 {
		t.Fatalf("v3 must not admit reviewed evidence")
	}

	// Unknown/missing provenance fails closed at the catalog boundary.
	values, _ := entry["value"].(map[string]any)
	forged := map[string]any{
		"reviewed":  true,
		"upload_id": uploadID.String(), "indicator_index": 0,
		"indicator_id":      "ferritin",
		"extraction_run_id": "not-a-uuid",
		"review_id":         reviewID.String(),
		"action":            "correct",
		"source_refs":       []string{"src:a"},
		"page_ref":          map[string]any{"src:a": map[string]any{"page_number": 1}},
	}
	forged["value"] = values
	req.ReviewedReportEvidence = mustAssessmentJSON(t, []any{forged})
	if len(buildAssessmentEvidenceCatalog(req, assessmentEvidencePolicyV4)) != 0 {
		t.Fatalf("invalid run provenance must fail closed")
	}

	forged["extraction_run_id"] = runID.String()
	forged["source_refs"] = []string{}
	req.ReviewedReportEvidence = mustAssessmentJSON(t, []any{forged})
	if len(buildAssessmentEvidenceCatalog(req, assessmentEvidencePolicyV4)) != 0 {
		t.Fatalf("missing source provenance must fail closed")
	}

	forged["source_refs"] = []string{"src:a"}
	tampered := map[string]any{}
	for k, v := range values {
		tampered[k] = v
	}
	tampered["indicator_id"] = "different-indicator"
	forged["value"] = tampered
	req.ReviewedReportEvidence = mustAssessmentJSON(t, []any{forged})
	if len(buildAssessmentEvidenceCatalog(req, assessmentEvidencePolicyV4)) != 0 {
		t.Fatalf("candidate identity mismatch must fail closed")
	}
}

func TestAssessmentReviewedDerivesOnlyLatestEffectiveReview(t *testing.T) {
	uploadID := uuid.New()
	runID := uuid.New()
	rejectID := uuid.New()
	correctID := uuid.New()
	now := time.Now()
	rejected := reviewedTestReview(
		string(model.ReviewActionReject), uploadID, runID, rejectID, 0,
		json.RawMessage(`{"indicator_id":"hb","name":"hb","value":"142.0","unit":"g/L"}`),
		nil, now.Add(-time.Minute),
	)
	corrected := reviewedTestReview(
		string(model.ReviewActionCorrect), uploadID, runID, correctID, 0,
		json.RawMessage(`{"indicator_id":"hb","name":"hb","value":"142.0","unit":"g/L"}`),
		json.RawMessage(`{"indicator_id":"hb","name":"hb","value":"140.0","unit":"g/L"}`),
		now,
	)
	uploads := []model.UserUpload{{ID: uploadID, FileType: "report"}}
	entries := buildAssessmentReviewedIndicators(uploads, map[uuid.UUID][]model.DocumentIndicatorReview{
		uploadID: {rejected, corrected},
	})
	if len(entries) != 1 {
		t.Fatalf("entries=%d want 1 (only latest correct supersedes the reject)", len(entries))
	}
	if v, _ := entries[0]["action"].(string); v != string(model.ReviewActionCorrect) {
		t.Fatalf("latest action=%q want correct", v)
	}
}

func TestAssessmentReviewedEvidenceNeverExposesRejectedOrUnresolved(t *testing.T) {
	uploadID := uuid.New()
	runID := uuid.New()
	rejected := reviewedTestReview(
		string(model.ReviewActionReject), uploadID, runID, uuid.New(), 0,
		json.RawMessage(`{"indicator_id":"ind","name":"x","value":"1","unit":"u"}`),
		nil, time.Now(),
	)
	entries := buildAssessmentReviewedIndicators(
		[]model.UserUpload{{ID: uploadID, FileType: "report"}},
		map[uuid.UUID][]model.DocumentIndicatorReview{uploadID: {rejected}},
	)
	if len(entries) != 0 {
		t.Fatalf("a rejected review must never become Assessment evidence")
	}
	// An unresolved candidate (no review rows) yields nothing either.
	if res := buildAssessmentReviewedIndicators(
		[]model.UserUpload{{ID: uploadID, FileType: "report"}}, nil,
	); len(res) != 0 {
		t.Fatalf("an unresolved candidate must be absent: %#v", res)
	}
}
