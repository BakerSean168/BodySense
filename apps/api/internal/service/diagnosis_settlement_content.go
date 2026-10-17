package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type diagnosisContentPatch struct {
	Field  string          `json:"field"`
	Before json.RawMessage `json:"before"`
	After  json.RawMessage `json:"after"`
}
type diagnosisRelationContent struct {
	FromConditionID   uuid.UUID                    `json:"fromConditionId"`
	ToConditionID     uuid.UUID                    `json:"toConditionId"`
	Kind              model.ConditionRelationKind  `json:"kind"`
	BasisEvidenceRefs []model.DiagnosisEvidenceRef `json:"basisEvidenceRefs"`
	Rationale         string                       `json:"rationale,omitempty"`
}
type diagnosisContentEnvelope struct {
	ProposedEntry     json.RawMessage             `json:"proposedEntry,omitempty"`
	ChangeSet         []diagnosisContentPatch     `json:"changeSet,omitempty"`
	NewDisposition    *model.ConditionDisposition `json:"newDisposition,omitempty"`
	DispositionReason string                      `json:"dispositionReason,omitempty"`
	RelationChange    *diagnosisRelationContent   `json:"relationChange,omitempty"`
}
type diagnosisProposedEvidence struct {
	EvidenceType    model.DiagnosisEvidenceType `json:"evidenceType"`
	EvidenceID      uuid.UUID                   `json:"evidenceId"`
	EvidenceVersion string                      `json:"evidenceVersion"`
	Role            model.DiagnosisEvidenceRole `json:"role"`
	Rationale       string                      `json:"rationale"`
	AnalyzedByRunID uuid.UUID                   `json:"analyzedByRunId"`
}
type diagnosisClinicalContent struct {
	MedicalName     string                          `json:"medicalName"`
	ConceptRef      *model.MedicalConceptRef        `json:"conceptRef"`
	Qualifiers      *model.ConditionQualifiers      `json:"qualifiers"`
	Assessment      *model.ConditionAssessment      `json:"assessment"`
	SourceClaimRefs []model.DiagnosisSourceClaimRef `json:"sourceClaimRefs"`
	EvidenceLinks   []diagnosisProposedEvidence     `json:"evidenceLinks"`
	CauseAnalysis   model.ConditionCauseAnalysis    `json:"causeAnalysis"`
	InformationGaps []string                        `json:"informationGaps"`
	HandlingPlan    model.ConditionHandlingPlan     `json:"handlingPlan"`
}

var diagnosisClinicalFields = map[string]bool{"medicalName": true, "conceptRef": true, "qualifiers": true, "assessment": true, "sourceClaimRefs": true, "evidenceLinks": true, "causeAnalysis": true, "informationGaps": true, "handlingPlan": true}

func strictDiagnosisJSON(raw []byte, value any) error {
	if len(raw) == 0 || len(raw) > 1<<20 {
		return ErrDiagnosisSettlementInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return ErrDiagnosisSettlementInvalid
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return ErrDiagnosisSettlementInvalid
	}
	return nil
}
func diagnosisJSONEqual(a, b any) bool {
	left, err := json.Marshal(a)
	if err != nil {
		return false
	}
	right, err := json.Marshal(b)
	if err != nil {
		return false
	}
	var x, y any
	dx := json.NewDecoder(bytes.NewReader(left))
	dx.UseNumber()
	dy := json.NewDecoder(bytes.NewReader(right))
	dy.UseNumber()
	if dx.Decode(&x) != nil || dy.Decode(&y) != nil {
		return false
	}
	left, _ = json.Marshal(x)
	right, _ = json.Marshal(y)
	return bytes.Equal(left, right)
}
func normalizeDiagnosisClinical(c *diagnosisClinicalContent) {
	if c.SourceClaimRefs == nil {
		c.SourceClaimRefs = []model.DiagnosisSourceClaimRef{}
	}
	if c.EvidenceLinks == nil {
		c.EvidenceLinks = []diagnosisProposedEvidence{}
	}
	if c.InformationGaps == nil {
		c.InformationGaps = []string{}
	}
	if c.HandlingPlan.Actions == nil {
		c.HandlingPlan.Actions = []string{}
	}
	if c.HandlingPlan.CautionNotes == nil {
		c.HandlingPlan.CautionNotes = []string{}
	}
}
func diagnosisClinicalSnapshot(entry model.ConditionEntry, links []model.ConditionEntryEvidence) diagnosisClinicalContent {
	out := diagnosisClinicalContent{MedicalName: entry.MedicalName, ConceptRef: entry.ConceptRef, Qualifiers: entry.Qualifiers, Assessment: entry.Assessment, SourceClaimRefs: entry.SourceClaimRefs, CauseAnalysis: entry.CauseAnalysis, InformationGaps: entry.InformationGaps, HandlingPlan: entry.HandlingPlan, EvidenceLinks: []diagnosisProposedEvidence{}}
	for _, link := range links {
		out.EvidenceLinks = append(out.EvidenceLinks, diagnosisProposedEvidence{link.EvidenceType, link.EvidenceID, strconv.FormatInt(link.EvidenceVersion, 10), link.Role, link.Rationale, link.AnalyzedByRunID})
	}
	normalizeDiagnosisClinical(&out)
	return out
}
func diagnosisUpdatedClinical(before diagnosisClinicalContent, envelope diagnosisContentEnvelope) (diagnosisClinicalContent, map[string]bool, error) {
	encoded, err := json.Marshal(before)
	if err != nil {
		return before, nil, err
	}
	var merged map[string]json.RawMessage
	if json.Unmarshal(encoded, &merged) != nil {
		return before, nil, ErrDiagnosisSettlementInvalid
	}
	changed := map[string]bool{}
	if len(envelope.ProposedEntry) > 0 {
		var fields map[string]json.RawMessage
		if json.Unmarshal(envelope.ProposedEntry, &fields) != nil || fields == nil {
			return before, nil, ErrDiagnosisSettlementInvalid
		}
		for key, value := range fields {
			if !diagnosisClinicalFields[key] {
				return before, nil, ErrDiagnosisSettlementInvalid
			}
			merged[key] = value
			changed[key] = true
		}
	}
	var originals map[string]json.RawMessage
	_ = json.Unmarshal(encoded, &originals)
	seen := map[string]bool{}
	for _, patch := range envelope.ChangeSet {
		if !diagnosisClinicalFields[patch.Field] || seen[patch.Field] || len(patch.Before) == 0 || len(patch.After) == 0 {
			return before, nil, ErrDiagnosisSettlementInvalid
		}
		seen[patch.Field] = true
		if !diagnosisJSONEqual(originals[patch.Field], patch.Before) {
			return before, nil, ErrDiagnosisDecisionConflict
		}
		if changed[patch.Field] && !diagnosisJSONEqual(merged[patch.Field], patch.After) {
			return before, nil, ErrDiagnosisSettlementInvalid
		}
		merged[patch.Field] = patch.After
		changed[patch.Field] = true
	}
	if len(changed) == 0 {
		return before, nil, ErrDiagnosisSettlementInvalid
	}
	encoded, err = json.Marshal(merged)
	if err != nil {
		return before, nil, err
	}
	var after diagnosisClinicalContent
	if err := strictDiagnosisJSON(encoded, &after); err != nil {
		return before, nil, err
	}
	normalizeDiagnosisClinical(&after)
	return after, changed, nil
}
func sameDiagnosisMedicalIdentity(before, after diagnosisClinicalContent) bool {
	if before.ConceptRef != nil || after.ConceptRef != nil {
		return before.ConceptRef != nil && after.ConceptRef != nil && before.ConceptRef.System == after.ConceptRef.System && before.ConceptRef.Code == after.ConceptRef.Code
	}
	// Without a verified stable code, do not infer that two different names
	// denote one condition. Only whitespace/case normalization is unambiguous.
	normalize := func(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }
	return normalize(before.MedicalName) == normalize(after.MedicalName)
}

func (s *DiagnosisSettlementService) validateSettlementClinical(ctx context.Context, owner uuid.UUID, run model.ConditionAnalysisRun, p model.DiagnosisProposal, c diagnosisClinicalContent, changed map[string]bool, bodyRevision int64) ([]model.ConditionEntryEvidence, error) {
	if strings.TrimSpace(c.MedicalName) == "" || utf8.RuneCountInString(c.MedicalName) > 240 || strings.TrimSpace(c.CauseAnalysis.Summary) == "" || strings.TrimSpace(c.CauseAnalysis.Rationale) == "" || strings.TrimSpace(c.HandlingPlan.Summary) == "" {
		return nil, ErrDiagnosisSettlementInvalid
	}
	if c.ConceptRef != nil && (strings.TrimSpace(c.ConceptRef.System) == "" || strings.TrimSpace(c.ConceptRef.Code) == "") {
		return nil, ErrDiagnosisSettlementInvalid
	}
	if len(c.SourceClaimRefs) > 1000 || len(c.EvidenceLinks) > 1000 || (len(c.SourceClaimRefs) == 0 && c.Assessment == nil) {
		return nil, ErrDiagnosisSettlementInvalid
	}
	if c.Assessment != nil {
		if (c.Assessment.Level != model.DiagnosisSuspected && c.Assessment.Level != model.DiagnosisLikely) || strings.TrimSpace(c.Assessment.Rationale) == "" || c.Assessment.BasedOnRunID == uuid.Nil {
			return nil, ErrDiagnosisSettlementInvalid
		}
		if changed["assessment"] && c.Assessment.BasedOnRunID != run.ID {
			return nil, ErrDiagnosisSettlementInvalid
		}
		if err := s.sources.Analysis(ctx, owner, c.Assessment.BasedOnRunID); err != nil {
			return nil, diagnosisEvidenceError(err)
		}
	}
	claims := map[string]bool{}
	for _, claim := range c.SourceClaimRefs {
		if strings.TrimSpace(claim.ClaimID) == "" || claims[claim.ClaimID] || strings.TrimSpace(claim.Statement) == "" || claim.RecordedAt.IsZero() {
			return nil, ErrDiagnosisSettlementInvalid
		}
		claims[claim.ClaimID] = true
		switch claim.ValidityAtAssessment {
		case model.DiagnosisSourceCurrent, model.DiagnosisSourceCorrected, model.DiagnosisSourceWithdrawn, model.DiagnosisSourceIneligible:
		default:
			return nil, ErrDiagnosisSettlementInvalid
		}
		if err := s.sources.Claim(ctx, owner, claim); err != nil {
			return nil, diagnosisEvidenceError(err)
		}
	}
	declared := map[string]bool{}
	refs, err := diagnosisDecidedEvidence(&p)
	if err != nil {
		return nil, err
	}
	for _, ref := range refs {
		declared[evidenceKey(ref)] = true
	}
	links := make([]model.ConditionEntryEvidence, 0, len(c.EvidenceLinks))
	seen := map[string]bool{}
	for _, link := range c.EvidenceLinks {
		ref := model.DiagnosisEvidenceRef{EvidenceType: link.EvidenceType, EvidenceID: link.EvidenceID, Version: link.EvidenceVersion}
		if strings.TrimSpace(link.Rationale) == "" || link.AnalyzedByRunID == uuid.Nil {
			return nil, ErrDiagnosisSettlementInvalid
		}
		switch link.Role {
		case model.DiagnosisSupporting, model.DiagnosisCounter, model.DiagnosisContext:
		default:
			return nil, ErrDiagnosisSettlementInvalid
		}
		if changed["evidenceLinks"] && !declared[evidenceKey(ref)] {
			return nil, ErrDiagnosisDecisionConflict
		}
		key := evidenceKey(ref) + ":" + string(link.Role)
		if seen[key] {
			return nil, ErrDiagnosisSettlementInvalid
		}
		seen[key] = true
		if err := s.sources.Evidence(ctx, owner, ref, bodyRevision); err != nil {
			return nil, diagnosisEvidenceError(err)
		}
		if err := s.sources.Analysis(ctx, owner, link.AnalyzedByRunID); err != nil {
			return nil, diagnosisEvidenceError(err)
		}
		version, err := strconv.ParseInt(link.EvidenceVersion, 10, 64)
		if err != nil {
			return nil, ErrDiagnosisSettlementInvalid
		}
		row := model.ConditionEntryEvidence{UserID: owner, EvidenceType: link.EvidenceType, EvidenceID: link.EvidenceID, EvidenceVersion: version, Role: link.Role, Rationale: link.Rationale, AnalyzedByRunID: link.AnalyzedByRunID}
		if link.EvidenceType == model.DiagnosisDocumentEvidence {
			action, err := s.repo.SettlementDocumentAction(ctx, owner, link.EvidenceID)
			if err != nil {
				return nil, err
			}
			row.DocumentReviewAction = &action
		}
		links = append(links, row)
	}
	return links, nil
}
