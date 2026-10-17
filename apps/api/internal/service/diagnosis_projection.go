package service

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
)

type diagnosisReadRepository interface {
	ReadSnapshot(context.Context, uuid.UUID) (*repository.DiagnosisReadSnapshot, error)
}

// NativeDiagnosisReadService projects only Go-committed Conditions. A proposal,
// recorded user choice, or unfinished settlement is never a diagnosis.
type NativeDiagnosisReadService struct{ repo diagnosisReadRepository }

func NewNativeDiagnosisReadService(repo diagnosisReadRepository) *NativeDiagnosisReadService {
	return &NativeDiagnosisReadService{repo: repo}
}

type NativeDiagnosisView struct {
	UserID           uuid.UUID                  `json:"userId"`
	CurrentRevision  int64                      `json:"currentRevision"`
	UpdatedAt        *time.Time                 `json:"updatedAt,omitempty"`
	ConditionEntries []NativeDiagnosisCondition `json:"conditionEntries"`
}
type NativeDiagnosisCondition struct {
	ConditionID       uuid.UUID                       `json:"conditionId"`
	MedicalName       string                          `json:"medicalName"`
	ConceptRef        *model.MedicalConceptRef        `json:"conceptRef,omitempty"`
	Qualifiers        *model.ConditionQualifiers      `json:"qualifiers,omitempty"`
	Assessment        *model.ConditionAssessment      `json:"assessment,omitempty"`
	SourceClaimRefs   []model.DiagnosisSourceClaimRef `json:"sourceClaimRefs"`
	EvidenceLinks     []NativeDiagnosisEvidenceLink   `json:"evidenceLinks"`
	CauseAnalysis     model.ConditionCauseAnalysis    `json:"causeAnalysis"`
	InformationGaps   []string                        `json:"informationGaps"`
	HandlingPlan      model.ConditionHandlingPlan     `json:"handlingPlan"`
	Disposition       model.ConditionDisposition      `json:"disposition"`
	LatestDecisionID  uuid.UUID                       `json:"latestDecisionId"`
	LastAnalysisRunID *uuid.UUID                      `json:"lastAnalysisRunId,omitempty"`
	CommittedRevision int64                           `json:"committedRevision"`
	CreatedAt         time.Time                       `json:"createdAt"`
	UpdatedAt         time.Time                       `json:"updatedAt"`
}
type NativeDiagnosisEvidenceLink struct {
	EvidenceType    model.DiagnosisEvidenceType `json:"evidenceType"`
	EvidenceID      uuid.UUID                   `json:"evidenceId"`
	EvidenceVersion string                      `json:"evidenceVersion"`
	Role            model.DiagnosisEvidenceRole `json:"role"`
	Rationale       string                      `json:"rationale"`
	AnalyzedByRunID uuid.UUID                   `json:"analyzedByRunId"`
}

func (s *NativeDiagnosisReadService) Read(ctx context.Context, owner uuid.UUID) (*NativeDiagnosisView, error) {
	snapshot, err := s.repo.ReadSnapshot(ctx, owner)
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, errors.New("diagnosis snapshot unavailable")
	}
	view := &NativeDiagnosisView{UserID: owner, ConditionEntries: []NativeDiagnosisCondition{}}
	if snapshot.Head == nil {
		if len(snapshot.Conditions) != 0 || len(snapshot.Evidence) != 0 {
			return nil, errors.New("diagnosis snapshot has conditions without head")
		}
		return view, nil
	}
	if snapshot.Head.UserID != owner || snapshot.Head.CurrentRevision < 0 {
		return nil, errors.New("diagnosis head is not owned by reader")
	}
	view.CurrentRevision = snapshot.Head.CurrentRevision
	view.UpdatedAt = &snapshot.Head.UpdatedAt
	for _, condition := range snapshot.Conditions {
		if condition.UserID != owner || condition.CommittedRevision < 1 || condition.CommittedRevision > snapshot.Head.CurrentRevision {
			return nil, errors.New("diagnosis condition outside owner's revision")
		}
		entry := NativeDiagnosisCondition{
			ConditionID: condition.ID, MedicalName: condition.MedicalName,
			ConceptRef: condition.ConceptRef, Qualifiers: condition.Qualifiers, Assessment: condition.Assessment,
			SourceClaimRefs: condition.SourceClaimRefs, EvidenceLinks: []NativeDiagnosisEvidenceLink{},
			CauseAnalysis: condition.CauseAnalysis, InformationGaps: condition.InformationGaps,
			HandlingPlan: condition.HandlingPlan, Disposition: condition.Disposition,
			LatestDecisionID: condition.LatestDecisionID, LastAnalysisRunID: condition.LastAnalysisRunID,
			CommittedRevision: condition.CommittedRevision, CreatedAt: condition.CreatedAt, UpdatedAt: condition.UpdatedAt,
		}
		if entry.SourceClaimRefs == nil {
			entry.SourceClaimRefs = []model.DiagnosisSourceClaimRef{}
		}
		if entry.InformationGaps == nil {
			entry.InformationGaps = []string{}
		}
		if entry.HandlingPlan.Actions == nil {
			entry.HandlingPlan.Actions = []string{}
		}
		if entry.HandlingPlan.CautionNotes == nil {
			entry.HandlingPlan.CautionNotes = []string{}
		}
		if entry.Assessment != nil {
			a := *entry.Assessment
			if a.InformationGaps == nil {
				a.InformationGaps = []string{}
			}
			if a.DifferentialNotes == nil {
				a.DifferentialNotes = []string{}
			}
			entry.Assessment = &a
		}
		for _, ref := range snapshot.Evidence[condition.ID] {
			if ref.UserID != owner || ref.ConditionID != condition.ID || ref.EvidenceVersion < 1 {
				return nil, errors.New("diagnosis evidence outside owner's revision")
			}
			entry.EvidenceLinks = append(entry.EvidenceLinks, NativeDiagnosisEvidenceLink{
				EvidenceType: ref.EvidenceType, EvidenceID: ref.EvidenceID, EvidenceVersion: strconv.FormatInt(ref.EvidenceVersion, 10),
				Role: ref.Role, Rationale: ref.Rationale, AnalyzedByRunID: ref.AnalyzedByRunID,
			})
		}
		view.ConditionEntries = append(view.ConditionEntries, entry)
	}
	return view, nil
}
