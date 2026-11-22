package service

import (
	"bytes"
	"encoding/json"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

func diagnosisFrozenInputIndex(refs []model.DiagnosisEvidenceRef) (map[string]string, []model.DiagnosisEvidenceRef, error) {
	if len(refs) > 2000 {
		return nil, nil, ErrDiagnosisIngestionInvalid
	}
	index := map[string]string{}
	out := make([]model.DiagnosisEvidenceRef, 0, len(refs))
	for _, ref := range refs {
		v, err := strconv.ParseInt(ref.Version, 10, 64)
		if err != nil || v <= 0 || strconv.FormatInt(v, 10) != ref.Version || ref.EvidenceID == uuid.Nil {
			return nil, nil, ErrDiagnosisIngestionInvalid
		}
		switch ref.EvidenceType {
		case model.DiagnosisBodyFact, model.DiagnosisBodyObservation:
		default:
			return nil, nil, ErrDiagnosisIngestionInvalid
		}
		key := string(ref.EvidenceType) + ":" + ref.EvidenceID.String()
		if _, dup := index[key]; dup {
			return nil, nil, ErrDiagnosisIngestionInvalid
		}
		index[key] = ref.Version
		out = append(out, ref)
	}
	return index, out, nil
}

func decodeDiagnosisAgentProposals(raw json.RawMessage, goBlocked bool) (*DiagnosisAgentProposals, error) {
	if goBlocked && len(raw) == 0 {
		return &DiagnosisAgentProposals{SchemaRevision: DiagnosisAgentProposalsSchemaV1, Outcome: DiagnosisAgentOutcomeInsufficient, Proposals: []DiagnosisAgentProposal{}}, nil
	}
	if len(raw) == 0 || len(raw) > 1<<20 {
		return nil, ErrDiagnosisAgentOutputInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var out DiagnosisAgentProposals
	if err := decoder.Decode(&out); err != nil {
		return nil, ErrDiagnosisAgentOutputInvalid
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return nil, ErrDiagnosisAgentOutputInvalid
	}
	if out.SchemaRevision != DiagnosisAgentProposalsSchemaV1 || utf8.RuneCountInString(out.Summary) > diagnosisAgentMaxText {
		return nil, ErrDiagnosisAgentOutputInvalid
	}
	switch out.Outcome {
	case DiagnosisAgentOutcomeProposals:
		if len(out.Proposals) == 0 || len(out.Proposals) > diagnosisAgentMaxProposals {
			return nil, ErrDiagnosisAgentOutputInvalid
		}
	case DiagnosisAgentOutcomeInsufficient:
		if len(out.Proposals) != 0 {
			return nil, ErrDiagnosisAgentOutputInvalid
		}
	default:
		return nil, ErrDiagnosisAgentOutputInvalid
	}
	return &out, nil
}

type diagnosisAgentDraft struct {
	statement string
	proposal  model.DiagnosisProposal
	clinical  diagnosisClinicalContent
}

func diagnosisAgentText(s string, required bool) bool {
	n := utf8.RuneCountInString(s)
	return n <= diagnosisAgentMaxText && (!required || strings.TrimSpace(s) != "")
}
func diagnosisAgentList(items []string) ([]string, bool) {
	if len(items) > diagnosisAgentMaxListItems {
		return nil, false
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if !diagnosisAgentText(item, true) {
			return nil, false
		}
		out = append(out, item)
	}
	return out, true
}

// buildDiagnosisAgentDrafts turns Agent proposals into exact immutable content.
// Every evidence reference must resolve to the frozen input the Agent saw; a
// foreign, unseen or later evidence ID rejects the whole output.
func buildDiagnosisAgentDrafts(run *model.ConditionAnalysisRun, out *DiagnosisAgentProposals, versions map[string]string) ([]diagnosisAgentDraft, error) {
	drafts := make([]diagnosisAgentDraft, 0, len(out.Proposals))
	names := map[string]bool{}
	for _, ap := range out.Proposals {
		if ap.ChangeKind != model.DiagnosisCreate {
			return nil, ErrDiagnosisAgentOutputInvalid
		}
		name := strings.TrimSpace(ap.MedicalName)
		normalized := strings.ToLower(strings.Join(strings.Fields(name), " "))
		if name == "" || utf8.RuneCountInString(name) > 240 || names[normalized] {
			return nil, ErrDiagnosisAgentOutputInvalid
		}
		names[normalized] = true
		if ap.Assessment.Level != model.DiagnosisSuspected && ap.Assessment.Level != model.DiagnosisLikely {
			return nil, ErrDiagnosisAgentOutputInvalid
		}
		if !diagnosisAgentText(ap.Assessment.Rationale, true) || !diagnosisAgentText(ap.CauseAnalysis.Summary, true) ||
			!diagnosisAgentText(ap.CauseAnalysis.Rationale, true) || !diagnosisAgentText(ap.HandlingPlan.Summary, true) {
			return nil, ErrDiagnosisAgentOutputInvalid
		}
		gaps, ok1 := diagnosisAgentList(ap.Assessment.InformationGaps)
		diffs, ok2 := diagnosisAgentList(ap.Assessment.DifferentialNotes)
		entryGaps, ok3 := diagnosisAgentList(ap.InformationGaps)
		actions, ok4 := diagnosisAgentList(ap.HandlingPlan.Actions)
		cautions, ok5 := diagnosisAgentList(ap.HandlingPlan.CautionNotes)
		if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || len(ap.Evidence) == 0 || len(ap.Evidence) > diagnosisAgentMaxEvidence {
			return nil, ErrDiagnosisAgentOutputInvalid
		}
		supporting := []model.DiagnosisEvidenceRef{}
		counter := []model.DiagnosisEvidenceRef{}
		links := make([]diagnosisProposedEvidence, 0, len(ap.Evidence))
		seen := map[string]bool{}
		for _, ev := range ap.Evidence {
			version, ok := versions[string(ev.EvidenceType)+":"+ev.EvidenceID.String()]
			if !ok || ev.EvidenceID == uuid.Nil || !diagnosisAgentText(ev.Rationale, true) {
				return nil, ErrDiagnosisAgentOutputInvalid
			}
			ref := model.DiagnosisEvidenceRef{EvidenceType: ev.EvidenceType, EvidenceID: ev.EvidenceID, Version: version}
			key := evidenceKey(ref) + ":" + string(ev.Role)
			if seen[key] {
				return nil, ErrDiagnosisAgentOutputInvalid
			}
			seen[key] = true
			switch ev.Role {
			case model.DiagnosisSupporting, model.DiagnosisContext:
				supporting = append(supporting, ref)
			case model.DiagnosisCounter:
				counter = append(counter, ref)
			default:
				return nil, ErrDiagnosisAgentOutputInvalid
			}
			links = append(links, diagnosisProposedEvidence{EvidenceType: ev.EvidenceType, EvidenceID: ev.EvidenceID, EvidenceVersion: version, Role: ev.Role, Rationale: ev.Rationale, AnalyzedByRunID: run.ID})
		}
		hasSupport := false
		for _, ev := range ap.Evidence {
			hasSupport = hasSupport || ev.Role == model.DiagnosisSupporting
		}
		if !hasSupport {
			return nil, ErrDiagnosisAgentOutputInvalid
		}
		clinical := diagnosisClinicalContent{
			MedicalName: name, ConceptRef: ap.ConceptRef, Qualifiers: ap.Qualifiers,
			Assessment:      &model.ConditionAssessment{Level: ap.Assessment.Level, Rationale: ap.Assessment.Rationale, InformationGaps: gaps, DifferentialNotes: diffs, BasedOnRunID: run.ID},
			SourceClaimRefs: []model.DiagnosisSourceClaimRef{}, EvidenceLinks: links,
			CauseAnalysis: ap.CauseAnalysis, InformationGaps: entryGaps,
			HandlingPlan: model.ConditionHandlingPlan{Summary: ap.HandlingPlan.Summary, Actions: actions, CautionNotes: cautions},
		}
		normalizeDiagnosisClinical(&clinical)
		entry, err := json.Marshal(clinical)
		if err != nil {
			return nil, ErrDiagnosisAgentOutputInvalid
		}
		content, err := json.Marshal(map[string]json.RawMessage{"proposedEntry": entry})
		if err != nil {
			return nil, ErrDiagnosisAgentOutputInvalid
		}
		p := model.DiagnosisProposal{
			UserID: run.UserID, ID: uuid.New(), RunID: run.ID, ChangeKind: model.DiagnosisCreate,
			Content: datatypes.JSON(content), BaseRevision: run.BasedOnDiagnosisRevision,
			EvidenceRefs: supporting, CounterEvidenceRefs: counter,
		}
		if p.ContentFingerprint, err = model.DiagnosisProposalContentFingerprint(&p); err != nil {
			return nil, ErrDiagnosisAgentOutputInvalid
		}
		drafts = append(drafts, diagnosisAgentDraft{statement: name, proposal: p, clinical: clinical})
	}
	return drafts, nil
}
