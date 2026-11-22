package service

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// This file is the Go durable-defense implementation of the current Assessment
// evidence contract. AssessmentService owns orchestration; this module owns exact
// evidence identity, source/kind policy, deterministic rendering and coverage.

type assessmentEvidenceItem struct {
	Source string
	Kind   string
	Value  any
}

var assessmentObservationAllowedSources = map[string]map[string]bool{
	"posture_alignment": {"posture_analysis": true},
	"posture_asymmetry": {"posture_analysis": true},
	"lifestyle_pattern": {"body_state": true},
	"exercise_pattern":  {"body_state": true},
	"report_indicator":  {"report": true},
	"anthropometry":     {"body_state": true},
}

var assessmentEvidenceDomainOrder = []string{
	"posture", "exercise", "lifestyle", "anthropometry", "health_report", "injury_symptoms",
}

func validateAssessmentEvidencePayload(
	payload *assessmentAgentPayload,
	req AssessmentGenerationRequest,
) (*assessmentEvidenceProjection, error) {
	catalog := buildAssessmentEvidenceCatalog(req)
	rendered := make([]assessmentObservationDraft, 0, len(payload.Observations))
	usedRefs := map[string]bool{}
	for index, observation := range payload.Observations {
		allowedSources, ok := assessmentObservationAllowedSources[observation.Kind]
		if !ok {
			return nil, fmt.Errorf("assessment observation %d has unsupported kind %q", index, observation.Kind)
		}
		if len(observation.EvidenceRefs) != 1 {
			return nil, fmt.Errorf("assessment observation %d must reference exactly one evidence item", index)
		}
		ref := observation.EvidenceRefs[0]
		if usedRefs[ref] {
			return nil, fmt.Errorf("assessment evidence %q was selected more than once", ref)
		}
		usedRefs[ref] = true
		item, exists := catalog[ref]
		if !exists {
			return nil, fmt.Errorf("assessment observation %d references unavailable evidence %q", index, ref)
		}
		if !allowedSources[item.Source] {
			return nil, fmt.Errorf(
				"assessment observation %d kind %q cannot use %s evidence",
				index, observation.Kind, item.Source,
			)
		}
		if item.Source == "body_state" && !assessmentBodyStateEvidenceKindAllowed(observation.Kind, item.Kind) {
			return nil, fmt.Errorf(
				"assessment observation %d kind %q is not supported by BodyState kind %q",
				index, observation.Kind, item.Kind,
			)
		}
		rendered = append(rendered, renderAssessmentObservation(observation.Kind, ref, item))
	}

	coverage := buildAssessmentEvidenceCoverage(catalog)
	status := "completed"
	if coverageStatus, _ := coverage["status"].(string); coverageStatus == "insufficient" {
		status = "insufficient_information"
	}
	if payload.Status != status {
		return nil, fmt.Errorf("assessment status %q does not match evidence-derived status %q", payload.Status, status)
	}
	gaps := buildAssessmentEvidenceGaps(coverage)
	summary := buildAssessmentEvidenceSummary(len(rendered), coverage)
	return &assessmentEvidenceProjection{
		Status: status, Observations: rendered, Coverage: coverage, Gaps: gaps, Summary: summary,
	}, nil
}

func assessmentBodyStateEvidenceKindAllowed(observationKind, evidenceKind string) bool {
	switch observationKind {
	case "exercise_pattern":
		return evidenceKind == "lifestyle.exercise"
	case "lifestyle_pattern":
		return strings.HasPrefix(evidenceKind, "lifestyle.") && evidenceKind != "lifestyle.exercise"
	case "anthropometry":
		return strings.HasPrefix(evidenceKind, "anthropometry.")
	default:
		return true
	}
}

// assessmentBodyStateCategory classifies evidence for coverage/rendering; it
// never changes the native record kind or source in the frozen input.
func assessmentBodyStateCategory(item map[string]any) string {
	kind, _ := item["kind"].(string)
	if kind == "lifestyle" {
		details, _ := item["details"].(map[string]any)
		if habit, ok := details["habit_type"].(string); ok && habit != "" {
			return "lifestyle." + habit
		}
	}
	if kind == "height" || kind == "weight" {
		return "anthropometry." + kind
	}
	return kind
}

func assessmentBodyStateEvidenceEligible(item map[string]any) bool {
	if status, ok := item["assertion_status"].(string); ok && status != "asserted" {
		return false
	}
	if excluded, ok := item["excluded_from_reasoning"].(bool); ok && excluded {
		return false
	}
	if reviewState, _ := item["review_state"].(string); reviewState != "" && reviewState != "confirmed" {
		return false
	}
	if lifecycleState, _ := item["lifecycle_state"].(string); lifecycleState != "" && lifecycleState != "active" {
		return false
	}
	return true
}

func assessmentDurableID(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" || text == "00000000-0000-0000-0000-000000000000" {
		return ""
	}
	return text
}

func assessmentBodyStateEvidenceRef(prefix string, item map[string]any, index int) string {
	if identity := assessmentDurableID(item["id"]); identity != "" {
		return prefix + ":" + identity
	}
	return fmt.Sprintf("%s:%d", prefix, index)
}

func assessmentInteger(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case float64:
		return int(typed), typed == float64(int(typed))
	case json.Number:
		parsed, err := strconv.Atoi(string(typed))
		return parsed, err == nil
	default:
		return 0, false
	}
}

func assessmentBodyStateCompactValue(item map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"id", "source_evidence_id", "created_revision", "kind", "value", "details", "body_region", "method", "review_state", "statement", "raw_value", "raw_unit", "authorship", "observed_at", "source", "temporal"} {
		if value, ok := item[key]; ok {
			out[key] = value
		}
	}
	return out
}

func assessmentReportIndicatorAdmissible(value any) bool {
	indicator, ok := value.(map[string]any)
	if !ok {
		return false
	}
	admissibility, ok := indicator["evidence_admissibility"].(map[string]any)
	if !ok {
		return false
	}
	policyRevision, _ := admissibility["policy_revision"].(string)
	status, _ := admissibility["status"].(string)
	return policyRevision == "ocr-indicator-admissibility-v1" && status == "admissible"
}

func buildAssessmentEvidenceCatalog(req AssessmentGenerationRequest) map[string]assessmentEvidenceItem {
	catalog := map[string]assessmentEvidenceItem{}

	var bodyState map[string]any
	if json.Unmarshal(req.BodyState, &bodyState) == nil {
		if facts, ok := bodyState["facts"].([]any); ok {
			for index, raw := range facts {
				item, _ := raw.(map[string]any)
				if item == nil || !assessmentBodyStateEvidenceEligible(item) {
					continue
				}
				kind := assessmentBodyStateCategory(item)
				catalog[assessmentBodyStateEvidenceRef("body_state:fact", item, index)] = assessmentEvidenceItem{
					Source: "body_state", Kind: firstNonEmpty(kind, "fact"), Value: assessmentBodyStateCompactValue(item),
				}
			}
		}
		if observations, ok := bodyState["observations"].([]any); ok {
			for index, raw := range observations {
				item, _ := raw.(map[string]any)
				if item == nil || !assessmentBodyStateEvidenceEligible(item) {
					continue
				}
				kind := assessmentBodyStateCategory(item)
				catalog[assessmentBodyStateEvidenceRef("body_state:observation", item, index)] = assessmentEvidenceItem{
					Source: "body_state", Kind: firstNonEmpty(kind, "observation"), Value: assessmentBodyStateCompactValue(item),
				}
			}
		}
	}

	var indicators []any
	if json.Unmarshal(req.ReportIndicators, &indicators) == nil {
		for index, raw := range indicators {
			ref := fmt.Sprintf("report:%d", index)
			value := raw
			if item, ok := raw.(map[string]any); ok {
				uploadID := assessmentDurableID(item["upload_id"])
				if indicatorIndex, ok := assessmentInteger(item["indicator_index"]); ok && uploadID != "" {
					ref = fmt.Sprintf("report:upload:%s:indicator:%d", uploadID, indicatorIndex)
				}
				if nested, exists := item["value"]; exists {
					value = nested
				}
			}
			// Current Assessment admits machine-extracted report indicators only
			// when the OCR admissibility contract marks the exact indicator admissible.
			if !assessmentReportIndicatorAdmissible(value) {
				continue
			}
			catalog[ref] = assessmentEvidenceItem{Source: "report", Kind: "report_indicator", Value: value}
		}
	}

	// Reviewed report evidence is a separate durable lane assembled only from
	// the append-only review projection. Confirmed/corrected latest reviews may
	// supply the reviewed payload with exact provenance. Rejected or unresolved
	// candidates and unknown/missing provenance fail closed.
	for index, raw := range assessmentReviewedReportIndicators(req) {
		item, ok := raw.(map[string]any)
		if !ok || !assessmentReviewedProvenanceComplete(item) {
			continue
		}
		if ref, ok := assessmentReviewedRef(item, index); ok {
			catalog[ref] = assessmentEvidenceItem{
				Source: "report",
				Kind:   "report_indicator",
				Value:  assessmentReviewedIndicatorValue(item),
			}
		}
	}

	seenPostureSummaries := map[string]bool{}
	var posture map[string]any
	if json.Unmarshal(req.PostureAnalysis, &posture) == nil {
		if views, ok := posture["views"].([]any); ok {
			for viewIndex, rawView := range views {
				view, _ := rawView.(map[string]any)
				if view == nil {
					continue
				}
				viewRef := fmt.Sprintf("posture:view:%d", viewIndex)
				if uploadID := assessmentDurableID(view["upload_id"]); uploadID != "" {
					viewRef = "posture:upload:" + uploadID
				}
				analysis, _ := view["analysis"].(map[string]any)
				if analysis == nil {
					continue
				}
				if findings, ok := analysis["findings"].([]any); ok {
					for findingIndex, rawFinding := range findings {
						finding, _ := rawFinding.(map[string]any)
						if finding == nil {
							continue
						}
						kind, _ := finding["key"].(string)
						value := map[string]any{}
						for key, item := range finding {
							value[key] = item
						}
						value["view"] = firstNonEmpty(fmt.Sprint(view["view"]), fmt.Sprint(view["file_type"]))
						catalog[fmt.Sprintf("%s:finding:%d", viewRef, findingIndex)] = assessmentEvidenceItem{
							Source: "posture_analysis", Kind: firstNonEmpty(kind, "posture_finding"), Value: value,
						}
					}
				}
				if summary, _ := analysis["summary_markdown"].(string); strings.TrimSpace(summary) != "" {
					summary = strings.TrimSpace(summary)
					catalog[viewRef+":summary"] = assessmentEvidenceItem{Source: "posture_analysis", Kind: "posture_summary", Value: summary}
					seenPostureSummaries[summary] = true
				}
			}
		}
		if summaries, ok := posture["summaries"].([]any); ok {
			for index, value := range summaries {
				summary := strings.TrimSpace(fmt.Sprint(value))
				if summary != "" && !seenPostureSummaries[summary] {
					catalog[fmt.Sprintf("posture:summary:%d", index)] = assessmentEvidenceItem{Source: "posture_analysis", Kind: "posture_summary", Value: summary}
					seenPostureSummaries[summary] = true
				}
			}
		}
	}

	return catalog
}
