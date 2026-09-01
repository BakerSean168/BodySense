package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// This file is the Go durable-defense implementation of Assessment evidence
// contract v2. AssessmentService owns orchestration; this module owns exact
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
	catalog := buildAssessmentEvidenceCatalog(req, payload.EvidencePolicyRevision)
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

func assessmentBodyStateEvidenceEligible(item map[string]any) bool {
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
	for _, key := range []string{"kind", "value", "details", "body_region", "method", "review_state"} {
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

func buildAssessmentEvidenceCatalog(req AssessmentGenerationRequest, evidencePolicyRevision string) map[string]assessmentEvidenceItem {
	catalog := map[string]assessmentEvidenceItem{}

	var bodyState map[string]any
	if json.Unmarshal(req.BodyState, &bodyState) == nil {
		if facts, ok := bodyState["facts"].([]any); ok {
			for index, raw := range facts {
				item, _ := raw.(map[string]any)
				if item == nil || !assessmentBodyStateEvidenceEligible(item) {
					continue
				}
				kind, _ := item["kind"].(string)
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
				kind, _ := item["kind"].(string)
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
			if evidencePolicyRevision == assessmentEvidencePolicyV3 && !assessmentReportIndicatorAdmissible(value) {
				continue
			}
			catalog[ref] = assessmentEvidenceItem{Source: "report", Kind: "report_indicator", Value: value}
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

var assessmentBodyStateLabels = map[string]string{
	"lifestyle.activity":     "日常活动记录",
	"lifestyle.sleep":        "睡眠作息记录",
	"lifestyle.exercise":     "运动记录",
	"lifestyle.nutrition":    "饮食节律记录",
	"lifestyle.substances":   "相关摄入记录",
	"lifestyle.recovery":     "恢复与压力记录",
	"anthropometry.height":   "身高记录",
	"anthropometry.weight":   "体重记录",
	"history.injury_summary": "既往伤病记录",
	"discomfort":             "不适记录",
	"symptom":                "症状记录",
}

func assessmentEvidenceText(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(typed)
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return strings.TrimSpace(fmt.Sprint(typed))
		}
		return string(encoded)
	}
}

func assessmentMeasurementText(value any) string {
	if object, ok := value.(map[string]any); ok {
		if raw, exists := object["value"]; exists {
			text := assessmentEvidenceText(raw)
			unit := strings.TrimSpace(fmt.Sprint(object["unit"]))
			if unit != "" && unit != "<nil>" {
				return strings.TrimSpace(text + " " + unit)
			}
			return text
		}
	}
	return assessmentEvidenceText(value)
}

func renderAssessmentObservation(kind, ref string, item assessmentEvidenceItem) assessmentObservationDraft {
	draft := assessmentObservationDraft{Kind: kind, EvidenceRefs: []string{ref}}
	switch item.Source {
	case "posture_analysis":
		if value, ok := item.Value.(map[string]any); ok {
			draft.Label = firstNonEmpty(strings.TrimSpace(fmt.Sprint(value["label"])), "体态分析观察")
			evidence := firstNonEmpty(strings.TrimSpace(fmt.Sprint(value["evidence"])), draft.Label)
			draft.Description = "体态分析记录：" + evidence + "。"
			draft.BodyRegion = strings.TrimSpace(fmt.Sprint(value["body_region"]))
			if draft.BodyRegion == "<nil>" {
				draft.BodyRegion = ""
			}
		} else {
			draft.Label = "体态分析摘要"
			draft.Description = "体态分析记录：" + assessmentEvidenceText(item.Value)
		}
	case "report":
		value, _ := item.Value.(map[string]any)
		name := firstNonEmpty(strings.TrimSpace(fmt.Sprint(value["name"])), "报告指标")
		measured := strings.TrimSpace(fmt.Sprint(value["value"]))
		if measured == "<nil>" {
			measured = ""
		}
		unit := strings.TrimSpace(fmt.Sprint(value["unit"]))
		if unit == "<nil>" {
			unit = ""
		}
		measurement := strings.TrimSpace(measured + " " + unit)
		content := name
		if measurement != "" {
			content += "=" + measurement
		}
		if reference := strings.TrimSpace(fmt.Sprint(value["reference_range"])); reference != "" && reference != "<nil>" {
			content += "；参考范围=" + reference
		}
		draft.Label = name
		draft.Description = "报告记录：" + content + "。"
	default:
		value, _ := item.Value.(map[string]any)
		draft.Label = firstNonEmpty(assessmentBodyStateLabels[item.Kind], "身体状态记录")
		rawValue := value["value"]
		text := assessmentEvidenceText(rawValue)
		if strings.HasPrefix(item.Kind, "anthropometry.") {
			text = assessmentMeasurementText(rawValue)
		}
		if text != "" {
			draft.Description = "来源记录：" + text + "。"
		} else {
			draft.Description = "来源记录已存在。"
		}
		draft.BodyRegion = strings.TrimSpace(fmt.Sprint(value["body_region"]))
		if draft.BodyRegion == "<nil>" {
			draft.BodyRegion = ""
		}
	}
	return draft
}

func assessmentDomainRefs(catalog map[string]assessmentEvidenceItem, domain string) []string {
	refs := make([]string, 0)
	for ref, item := range catalog {
		switch domain {
		case "posture":
			if item.Source == "posture_analysis" {
				refs = append(refs, ref)
			}
		case "exercise":
			if item.Source == "body_state" && item.Kind == "lifestyle.exercise" {
				refs = append(refs, ref)
			}
		case "lifestyle":
			if item.Source == "body_state" && strings.HasPrefix(item.Kind, "lifestyle.") && item.Kind != "lifestyle.exercise" {
				refs = append(refs, ref)
			}
		case "anthropometry":
			if item.Source == "body_state" && strings.HasPrefix(item.Kind, "anthropometry.") {
				refs = append(refs, ref)
			}
		case "health_report":
			if item.Source == "report" {
				refs = append(refs, ref)
			}
		case "injury_symptoms":
			if item.Source == "body_state" && assessmentKindContainsAny(item.Kind, "injury", "symptom", "pain", "discomfort", "history") {
				refs = append(refs, ref)
			}
		}
	}
	sort.Strings(refs)
	return refs
}

func assessmentKindContainsAny(kind string, tokens ...string) bool {
	for _, token := range tokens {
		if strings.Contains(kind, token) {
			return true
		}
	}
	return false
}

func buildAssessmentEvidenceCoverage(catalog map[string]assessmentEvidenceItem) map[string]any {
	domains := map[string]any{}
	availableCount := 0
	for _, domain := range assessmentEvidenceDomainOrder {
		refs := assessmentDomainRefs(catalog, domain)
		status := "missing"
		if len(refs) > 0 {
			status = "available"
			availableCount++
		}
		domains[domain] = map[string]any{"status": status, "evidence_refs": refs}
	}
	status := "partial"
	if availableCount == 0 {
		status = "insufficient"
	} else if availableCount == len(assessmentEvidenceDomainOrder) {
		status = "complete"
	}
	sourceSet := map[string]bool{}
	for _, item := range catalog {
		sourceSet[item.Source] = true
	}
	sources := make([]string, 0, len(sourceSet))
	for source := range sourceSet {
		sources = append(sources, source)
	}
	sort.Strings(sources)
	return map[string]any{"status": status, "available_sources": sources, "domains": domains}
}

func buildAssessmentEvidenceGaps(coverage map[string]any) []map[string]any {
	type gapSpec struct {
		Description   string
		NeededSources []string
	}
	specs := map[string]gapSpec{
		"posture":         {Description: "当前未提供已完成的体态分析。", NeededSources: []string{"posture_analysis"}},
		"exercise":        {Description: "当前未提供运动方式或频率记录。", NeededSources: []string{"body_state"}},
		"lifestyle":       {Description: "当前未提供其它生活方式记录。", NeededSources: []string{"body_state"}},
		"anthropometry":   {Description: "当前未提供身体测量记录。", NeededSources: []string{"body_state"}},
		"health_report":   {Description: "当前未提供结构化健康报告指标。", NeededSources: []string{"report"}},
		"injury_symptoms": {Description: "当前未提供伤病史或症状记录。", NeededSources: []string{"body_state"}},
	}
	domains, _ := coverage["domains"].(map[string]any)
	gaps := make([]map[string]any, 0, len(assessmentEvidenceDomainOrder))
	for _, domain := range assessmentEvidenceDomainOrder {
		state, _ := domains[domain].(map[string]any)
		if status, _ := state["status"].(string); status != "missing" {
			continue
		}
		spec := specs[domain]
		gaps = append(gaps, map[string]any{
			"dimension": domain, "description": spec.Description,
			"needed_sources": spec.NeededSources, "required": false,
		})
	}
	return gaps
}

func buildAssessmentEvidenceSummary(observationCount int, coverage map[string]any) string {
	domains, _ := coverage["domains"].(map[string]any)
	available := 0
	for _, domain := range assessmentEvidenceDomainOrder {
		state, _ := domains[domain].(map[string]any)
		if status, _ := state["status"].(string); status == "available" {
			available++
		}
	}
	total := len(assessmentEvidenceDomainOrder)
	return fmt.Sprintf(
		"当前资料支持 %d 项待审核观察；%d/%d 个证据领域已有资料，%d/%d 个领域当前未提供资料。",
		observationCount, available, total, total-available, total,
	)
}
