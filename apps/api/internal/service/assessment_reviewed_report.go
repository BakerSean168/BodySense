package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// reviewedAssessmentActions are the only review decisions that can promote a
// health-document candidate into Assessment report evidence. Confirmed and
// corrected latest-effective reviews qualify; rejected or unresolved
// candidates never do.
var reviewedAssessmentActions = map[string]bool{
	string(model.ReviewActionConfirm): true,
	string(model.ReviewActionCorrect): true,
}

// AssessmentReviewSource exposes the durable append-only review projection for
// one user's upload. Assessment uses this authoritative source instead of
// mutating OCRResult/evidence_admissibility.
type AssessmentReviewSource interface {
	ListByUpload(ctx context.Context, uploadID, userID uuid.UUID) ([]model.DocumentIndicatorReview, error)
}

// buildReviewedCandidate projects one retained (confirm/correct) review row into
// the Assessment report lane with immutable candidate and source provenance.
func buildReviewedCandidate(row model.DocumentIndicatorReview) (map[string]any, bool) {
	if !reviewedAssessmentActions[row.Action] ||
		row.ID == uuid.Nil || row.UserID == uuid.Nil || row.UploadID == uuid.Nil ||
		row.ExtractionRunID == uuid.Nil || row.ReviewerUserID == uuid.Nil || row.IndicatorIndex < 0 {
		return nil, false
	}
	value, ok := reviewedValue(row)
	if !ok {
		return nil, false
	}
	machineID := reviewedMachineIndicatorID(row)
	valueID, _ := value["indicator_id"].(string)
	valueID = strings.TrimSpace(valueID)
	// A correction may change the measurement, never the candidate identity.
	// Re-check this here so historical malformed rows also fail closed.
	if machineID == "" || valueID == "" || valueID != machineID {
		return nil, false
	}
	sourceRefs := reviewedRefs(row.SourceRefs)
	if len(sourceRefs) == 0 {
		return nil, false
	}
	pageRef, ok := reviewedPageRef(row.PageRef)
	if !ok {
		return nil, false
	}
	return map[string]any{
		"upload_id":         row.UploadID.String(),
		"indicator_index":   row.IndicatorIndex,
		"indicator_id":      machineID,
		"extraction_run_id": row.ExtractionRunID.String(),
		"review_id":         row.ID.String(),
		"reviewer_user_id":  row.ReviewerUserID.String(),
		"action":            row.Action,
		"source_refs":       sourceRefs,
		"page_ref":          pageRef,
		"reviewed":          true,
		"value":             value,
	}, true
}

func reviewedMachineIndicatorID(row model.DocumentIndicatorReview) string {
	if len(row.MachineCandidate) == 0 {
		return ""
	}
	var obj map[string]any
	if json.Unmarshal(row.MachineCandidate, &obj) != nil {
		return ""
	}
	id, _ := obj["indicator_id"].(string)
	return strings.TrimSpace(id)
}

// reviewedValue returns the authoritative measurement payload for a candidate:
// the frozen machine candidate for confirm, the frozen corrected payload for
// correct. Candidate identity is validated separately against MachineCandidate.
func reviewedValue(row model.DocumentIndicatorReview) (map[string]any, bool) {
	src := row.MachineCandidate
	if row.Action == string(model.ReviewActionCorrect) {
		src = row.ReviewedPayload
	}
	if len(src) == 0 {
		return nil, false
	}
	var obj map[string]any
	if err := json.Unmarshal(src, &obj); err != nil || obj == nil {
		return nil, false
	}
	return obj, true
}

func reviewedRefs(raw []byte) []string {
	var refs []any
	if json.Unmarshal(raw, &refs) != nil {
		return nil
	}
	out := make([]string, 0, len(refs))
	seen := map[string]bool{}
	for _, r := range refs {
		s, _ := r.(string)
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func reviewedPageRef(raw []byte) (map[string]any, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var pageRef map[string]any
	if json.Unmarshal(raw, &pageRef) != nil || pageRef == nil {
		return nil, false
	}
	return pageRef, true
}

type reviewedScope struct {
	upload uuid.UUID
	run    uuid.UUID
	index  int
}

func reviewedRowNewer(candidate, current model.DocumentIndicatorReview) bool {
	if candidate.CreatedAt.After(current.CreatedAt) {
		return true
	}
	if candidate.CreatedAt.Before(current.CreatedAt) {
		return false
	}
	// UUIDv7 is the storage default; the ID tie-breaker makes projection stable
	// even when two rows share the database timestamp precision.
	return candidate.ID.String() > current.ID.String()
}

// buildAssessmentReviewedIndicators projects every latest-effective
// confirm/correct review into durable report evidence. Rejected, unresolved and
// superseded candidates are excluded because only the latest decision per
// (upload, extraction-run, indicator-index) is considered.
func buildAssessmentReviewedIndicators(
	uploads []model.UserUpload,
	reviewsByUpload map[uuid.UUID][]model.DocumentIndicatorReview,
) []map[string]any {
	latest := map[reviewedScope]model.DocumentIndicatorReview{}
	for _, upload := range uploads {
		for _, row := range reviewsByUpload[upload.ID] {
			// Never let an unexpected repository row cross upload scope.
			if row.UploadID != upload.ID {
				continue
			}
			key := reviewedScope{upload: row.UploadID, run: row.ExtractionRunID, index: row.IndicatorIndex}
			if existing, ok := latest[key]; ok && !reviewedRowNewer(row, existing) {
				continue
			}
			latest[key] = row
		}
	}
	keys := make([]reviewedScope, 0, len(latest))
	for key := range latest {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		as, bs := a.upload.String(), b.upload.String()
		if as != bs {
			return as < bs
		}
		ars, brs := a.run.String(), b.run.String()
		if ars != brs {
			return ars < brs
		}
		return a.index < b.index
	})
	out := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		if entry, ok := buildReviewedCandidate(latest[key]); ok {
			out = append(out, entry)
		}
	}
	return out
}

func assessmentReviewedReportIndicators(req AssessmentGenerationRequest) []any {
	if len(req.ReviewedReportEvidence) == 0 || !json.Valid(req.ReviewedReportEvidence) {
		return nil
	}
	var out []any
	if json.Unmarshal(req.ReviewedReportEvidence, &out) != nil {
		return nil
	}
	return out
}

func assessmentReviewedUUID(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	parsed, err := uuid.Parse(text)
	if err != nil || parsed == uuid.Nil {
		return ""
	}
	return parsed.String()
}

func assessmentReviewedSourceRefs(value any) ([]string, bool) {
	items, ok := value.([]any)
	if !ok || len(items) == 0 {
		return nil, false
	}
	out := make([]string, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		ref, ok := item.(string)
		ref = strings.TrimSpace(ref)
		if !ok || ref == "" {
			return nil, false
		}
		if !seen[ref] {
			seen[ref] = true
			out = append(out, ref)
		}
	}
	return out, len(out) > 0
}

// assessmentReviewedProvenanceComplete fails closed on reviewed input unless it
// has exact upload/run/review/candidate/source provenance and the reviewed value
// remains bound to that candidate identity.
func assessmentReviewedProvenanceComplete(item map[string]any) bool {
	if reviewed, ok := item["reviewed"].(bool); !ok || !reviewed {
		return false
	}
	if assessmentReviewedUUID(item["upload_id"]) == "" ||
		assessmentReviewedUUID(item["extraction_run_id"]) == "" ||
		assessmentReviewedUUID(item["review_id"]) == "" {
		return false
	}
	if !reviewedAssessmentActions[strings.TrimSpace(fmt.Sprint(item["action"]))] {
		return false
	}
	index, ok := assessmentInteger(item["indicator_index"])
	if !ok || index < 0 {
		return false
	}
	indicatorID := strings.TrimSpace(fmt.Sprint(item["indicator_id"]))
	value, ok := item["value"].(map[string]any)
	if !ok || indicatorID == "" || strings.TrimSpace(fmt.Sprint(value["indicator_id"])) != indicatorID {
		return false
	}
	if _, ok := assessmentReviewedSourceRefs(item["source_refs"]); !ok {
		return false
	}
	if _, ok := item["page_ref"].(map[string]any); !ok {
		return false
	}
	return true
}

// assessmentReviewedIndicatorValue returns the reviewed measurement with a
// structured, non-text source provenance envelope. It contains no raw document
// text; page_ref was already privacy-reduced when the immutable review row was
// created.
func assessmentReviewedIndicatorValue(item map[string]any) map[string]any {
	valueMap, ok := item["value"].(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string]any, len(valueMap)+2)
	for key, value := range valueMap {
		out[key] = value
	}
	sourceRefs, _ := assessmentReviewedSourceRefs(item["source_refs"])
	pageRef, _ := item["page_ref"].(map[string]any)
	out["reviewed"] = true
	out["review_provenance"] = map[string]any{
		"action":            strings.TrimSpace(fmt.Sprint(item["action"])),
		"review_id":         assessmentReviewedUUID(item["review_id"]),
		"reviewer_user_id":  assessmentReviewedUUID(item["reviewer_user_id"]),
		"upload_id":         assessmentReviewedUUID(item["upload_id"]),
		"extraction_run_id": assessmentReviewedUUID(item["extraction_run_id"]),
		"indicator_id":      strings.TrimSpace(fmt.Sprint(item["indicator_id"])),
		"indicator_index":   item["indicator_index"],
		"source_refs":       sourceRefs,
		"page_ref":          pageRef,
	}
	return out
}

func assessmentReviewedRef(item map[string]any, _ int) (string, bool) {
	uploadID := assessmentReviewedUUID(item["upload_id"])
	index, ok := assessmentInteger(item["indicator_index"])
	if uploadID == "" || !ok || index < 0 {
		return "", false
	}
	return fmt.Sprintf("report:upload:%s:indicator:%d", uploadID, index), true
}
