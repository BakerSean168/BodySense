package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

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
		if statement, ok := value["statement"].(string); ok && statement != "" {
			text = statement
		} else if strings.HasPrefix(item.Kind, "anthropometry.") {
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
