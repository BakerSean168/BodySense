package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/bodysense/api/internal/model"
	"gorm.io/datatypes"
)

// decodeIndicatorSnapshot decodes the persisted indicator_snapshot JSON array
// of an extraction run into replayable candidate snapshots indexed by their
// array position (the indicator index the review is bound to).
func decodeIndicatorSnapshot(snapshot datatypes.JSON) ([]reviewCandidateSnapshot, error) {
	if len(snapshot) == 0 {
		return nil, errors.New("indicator snapshot is empty")
	}
	var items []json.RawMessage
	if err := json.Unmarshal(snapshot, &items); err != nil {
		return nil, fmt.Errorf("invalid indicator snapshot array: %w", err)
	}
	out := make([]reviewCandidateSnapshot, 0, len(items))
	for position, raw := range items {
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			return nil, fmt.Errorf("indicator snapshot item %d malformed: %w", position, err)
		}
		indicatorID, _ := obj["indicator_id"].(string)
		if indicatorID == "" {
			return nil, fmt.Errorf("indicator snapshot item %d missing indicator_id", position)
		}
		name, _ := obj["name"].(string)
		value := obj["value"]
		unit, _ := obj["unit"].(string)
		referenceRange, _ := obj["reference_range"].(string)
		admissibility, err := reviewEvidenceAdmissibility(obj["evidence_admissibility"])
		if err != nil {
			return nil, fmt.Errorf("indicator snapshot item %d evidence_admissibility: %w", position, err)
		}
		refs, err := reviewStringSlice(obj["source_refs"])
		if err != nil {
			return nil, fmt.Errorf("indicator snapshot item %d source_refs: %w", position, err)
		}
		if len(refs) == 0 {
			return nil, fmt.Errorf("indicator snapshot item %d has no source_refs", position)
		}
		out = append(out, reviewCandidateSnapshot{
			raw:                   obj,
			IndicatorID:           indicatorID,
			Name:                  name,
			Value:                 value,
			Unit:                  unit,
			ReferenceRange:        referenceRange,
			SourceRefs:            refs,
			EvidenceAdmissibility: admissibility,
		})
	}
	return out, nil
}

func reviewEvidenceAdmissibility(value any) (DocumentIndicatorEvidenceAdmissibility, error) {
	obj, ok := value.(map[string]any)
	if !ok {
		return DocumentIndicatorEvidenceAdmissibility{}, errors.New("missing admissibility object")
	}
	status, _ := obj["status"].(string)
	if status != "admissible" && status != "needs_review" && status != "rejected" {
		return DocumentIndicatorEvidenceAdmissibility{}, fmt.Errorf("unsupported status %q", status)
	}
	policyRevision, _ := obj["policy_revision"].(string)
	if strings.TrimSpace(policyRevision) == "" {
		return DocumentIndicatorEvidenceAdmissibility{}, errors.New("missing policy_revision")
	}
	reasonCodes, err := reviewStringSlice(obj["reason_codes"])
	if err != nil {
		return DocumentIndicatorEvidenceAdmissibility{}, fmt.Errorf("reason_codes: %w", err)
	}
	return DocumentIndicatorEvidenceAdmissibility{
		Status: status, PolicyRevision: policyRevision, ReasonCodes: reasonCodes,
	}, nil
}

// decodeSourceBlocks decodes the source_blocks portion of the persisted source
// summary into a lookup keyed by source_ref, capturing the page-level bbox
// context already vetted by the extraction contract. It never contains storage
// backend/key or raw OCR text.
func decodeSourceBlocks(summary datatypes.JSON) (map[string]map[string]any, error) {
	if len(summary) == 0 {
		return nil, errors.New("source summary is empty")
	}
	var summaryObj map[string]json.RawMessage
	if err := json.Unmarshal(summary, &summaryObj); err != nil {
		return nil, errors.New("source summary is not an object")
	}
	blocksJSON, ok := summaryObj["source_blocks"]
	if !ok {
		return nil, errors.New("source summary missing source_blocks")
	}
	var blocks []json.RawMessage
	if err := json.Unmarshal(blocksJSON, &blocks); err != nil {
		return nil, errors.New("source summary source_blocks is not an array")
	}
	out := make(map[string]map[string]any, len(blocks))
	for _, raw := range blocks {
		var block map[string]any
		if err := json.Unmarshal(raw, &block); err != nil {
			return nil, errors.New("source block is malformed")
		}
		ref, _ := block["source_ref"].(string)
		if strings.TrimSpace(ref) == "" {
			return nil, errors.New("source block missing source_ref")
		}
		out[ref] = block
	}
	return out, nil
}

// validateReviewSourceRefs fails closed when the request references a source
// ref that is neither attached to the candidate nor present in the persisted
// run source snapshot. Empty submissions are rejected too, mirroring the
// extraction contract requirement that candidates carry source refs.
func validateReviewSourceRefs(submitted []string, blocks map[string]map[string]any, candidate []string) error {
	if len(submitted) == 0 {
		return fmt.Errorf("%w: source refs are required", ErrReviewCandidateMismatch)
	}
	expected := make(map[string]struct{}, len(candidate))
	for _, ref := range candidate {
		if strings.TrimSpace(ref) != "" {
			expected[ref] = struct{}{}
		}
	}
	for _, ref := range submitted {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			return fmt.Errorf("%w: empty source ref", ErrReviewCandidateMismatch)
		}
		if _, ok := expected[ref]; !ok {
			return fmt.Errorf("%w: source ref %q is not attached to the candidate", ErrReviewCandidateMismatch, ref)
		}
		if _, ok := blocks[ref]; !ok {
			return fmt.Errorf("%w: source ref %q is absent from the persisted run snapshot", ErrReviewCandidateMismatch, ref)
		}
	}
	return nil
}

func reviewSourceRegions(blocks map[string]map[string]any, refs []string) ([]DocumentIndicatorSourceRegion, error) {
	regions := make([]DocumentIndicatorSourceRegion, 0, len(refs))
	for _, ref := range refs {
		block, ok := blocks[ref]
		if !ok {
			return nil, fmt.Errorf("source ref %q is absent from persisted source blocks", ref)
		}
		region := DocumentIndicatorSourceRegion{SourceRef: ref}
		pageValue, hasPage := block["page_number"]
		if !hasPage {
			pageValue, hasPage = block["page"]
		}
		if hasPage {
			pageFloat, ok := pageValue.(float64)
			if !ok || pageFloat < 1 || pageFloat != float64(int(pageFloat)) {
				return nil, fmt.Errorf("source ref %q has invalid page number", ref)
			}
			page := int(pageFloat)
			region.PageNumber = &page
		}
		if bboxValue, ok := block["bbox"]; ok {
			items, ok := bboxValue.([]any)
			if !ok || len(items) != 4 {
				return nil, fmt.Errorf("source ref %q has invalid bbox", ref)
			}
			region.BBox = make([]float64, 4)
			for i, item := range items {
				value, ok := item.(float64)
				if !ok {
					return nil, fmt.Errorf("source ref %q has non-numeric bbox", ref)
				}
				region.BBox[i] = value
			}
		}
		regions = append(regions, region)
	}
	return regions, nil
}

// pageRefForCandidate captures the page + bbox context for the reviewed
// candidate by resolving its source refs against the run's persisted blocks.
func pageRefForCandidate(blocks map[string]map[string]any, refs []string) ([]byte, error) {
	pageRef := map[string]any{}
	for _, ref := range refs {
		block, ok := blocks[ref]
		if !ok {
			continue
		}
		entry := map[string]any{}
		if page, exists := block["page_number"]; exists {
			entry["page_number"] = page
		}
		if page, exists := block["page"]; exists {
			entry["page_number"] = page
		}
		if bbox, exists := block["bbox"]; exists {
			entry["bbox"] = bbox
		}
		if text, exists := block["source_block_text"]; exists {
			// Keep only structural/summary fields; never embed raw private text.
			entry["_private_text_omitted"] = true
			_ = text
		}
		if len(entry) > 0 {
			pageRef[ref] = entry
		}
	}
	return json.Marshal(pageRef)
}

// normalizeReviewAction maps the wire action to the canonical review action.
func normalizeReviewAction(action string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case string(model.ReviewActionConfirm):
		return string(model.ReviewActionConfirm), nil
	case string(model.ReviewActionCorrect):
		return string(model.ReviewActionCorrect), nil
	case string(model.ReviewActionReject):
		return string(model.ReviewActionReject), nil
	default:
		return "", fmt.Errorf("%w: unsupported action %q", ErrReviewValidation, action)
	}
}

// normalizeReviewedPayload shapes the stored payload per action. Confirm and
// reject always store the machine candidate untouched, so no untrusted caller
// payload can corrupt audit history; correct stores the submitted corrected
// indicator JSON only after basic validation.
func normalizeReviewedPayload(action string, payload json.RawMessage) (json.RawMessage, error) {
	switch action {
	case string(model.ReviewActionConfirm), string(model.ReviewActionReject):
		return nil, nil
	case string(model.ReviewActionCorrect):
		trimmed := bytes.TrimSpace(payload)
		if len(trimmed) == 0 {
			return nil, fmt.Errorf("%w: corrected payload is required for correct", ErrReviewValidation)
		}
		var object map[string]any
		if err := json.Unmarshal(trimmed, &object); err != nil {
			return nil, fmt.Errorf("%w: corrected payload must be a JSON object", ErrReviewValidation)
		}
		indicatorID, _ := object["indicator_id"].(string)
		if indicatorID == "" {
			return nil, fmt.Errorf("%w: corrected payload missing indicator_id", ErrReviewValidation)
		}
		value, hasValue := object["value"]
		if !hasValue {
			return nil, fmt.Errorf("%w: corrected payload missing value", ErrReviewValidation)
		}
		_ = value
		// Freeze the payload so the immutable row never references later OCR.
		return json.Marshal(object)
	default:
		return nil, fmt.Errorf("%w: unsupported action %q", ErrReviewValidation, action)
	}
}

// sameJSON reports whether two JSON payloads are semantically identical.
// sameJSON reports whether two JSON payloads are semantically identical.
// nil/empty payloads (confirm and reject store no reviewed payload) are equal,
// so an idempotent replay of those actions returns the stored row.
func sameJSON(a, b json.RawMessage) bool {
	ta := bytes.TrimSpace(a)
	tb := bytes.TrimSpace(b)
	if len(ta) == 0 && len(tb) == 0 {
		return true
	}
	if len(ta) == 0 || len(tb) == 0 {
		return false
	}
	var av, bv any
	if json.Unmarshal(ta, &av) != nil {
		return false
	}
	if json.Unmarshal(tb, &bv) != nil {
		return false
	}
	ab, errA := json.Marshal(av)
	bb, errB := json.Marshal(bv)
	if errA != nil || errB != nil {
		return false
	}
	return bytes.Equal(ab, bb)
}

// reviewRecordFromRow converts a persisted immutable row into the public
// projection record. IndicatorID is reconstructed from the machine candidate
// snapshot rather than from any mutable upload column.
func reviewRecordFromRow(row model.DocumentIndicatorReview) DocumentIndicatorReviewRecord {
	record := DocumentIndicatorReviewRecord{
		ReviewID:        row.ID,
		ExtractionRunID: row.ExtractionRunID,
		UploadID:        row.UploadID,
		IndicatorIndex:  row.IndicatorIndex,
		Action:          row.Action,
		Note:            row.Note,
		ReviewerUserID:  row.ReviewerUserID,
		CreatedAt:       row.CreatedAt,
		IdempotencyKey:  row.IdempotencyKey,
		ReviewedPayload: json.RawMessage(row.ReviewedPayload),
	}
	if len(row.MachineCandidate) > 0 {
		var candidate map[string]any
		if err := json.Unmarshal(row.MachineCandidate, &candidate); err == nil {
			if id, ok := candidate["indicator_id"].(string); ok {
				record.IndicatorID = id
			}
		}
	}
	return record
}

// stringSlice converts a JSON value into a string slice, failing closed on
// wrong shapes so stale or fabricated snapshot content cannot pass validation.
func reviewStringSlice(value any) ([]string, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, errors.New("expected an array of strings")
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		ref, ok := item.(string)
		if !ok || strings.TrimSpace(ref) == "" {
			return nil, errors.New("expected non-empty strings")
		}
		out = append(out, ref)
	}
	return out, nil
}
