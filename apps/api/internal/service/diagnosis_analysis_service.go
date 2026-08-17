package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type diagnosisAnalysisRepository interface {
	Create(ctx context.Context, analysis *model.DiagnosisAnalysisRecord, candidates []model.DiagnosisCandidateRecord) error
	ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]model.DiagnosisAnalysisRecord, error)
	GetLatestByUser(ctx context.Context, userID uuid.UUID) (*model.DiagnosisAnalysisRecord, error)
	GetByID(ctx context.Context, analysisID, userID uuid.UUID) (*model.DiagnosisAnalysisRecord, error)
	UpsertAssessment(ctx context.Context, assessment *model.DiagnosisCandidateAssessment) error
	ListAssessments(ctx context.Context, analysisID, userID uuid.UUID) ([]model.DiagnosisCandidateAssessment, error)
}

// DiagnosisAnalysisService turns an AI reasoning result into immutable Go-owned
// business identity. Python proposes candidate content; it never invents durable
// analysis/candidate IDs used by user confirmation or historical references.
type DiagnosisAnalysisService struct {
	repo diagnosisAnalysisRepository
}

func NewDiagnosisAnalysisService(repo diagnosisAnalysisRepository) *DiagnosisAnalysisService {
	return &DiagnosisAnalysisService{repo: repo}
}

type aiDiagnosisPayload struct {
	Status               string            `json:"status"`
	Scope                string            `json:"scope"`
	Summary              string            `json:"summary"`
	Candidates           []aiDiagnosisItem `json:"candidates"`
	CrossConcernPatterns json.RawMessage   `json:"cross_concern_patterns"`
	InformationGaps      json.RawMessage   `json:"information_gaps"`
	SafetySummary        json.RawMessage   `json:"safety_summary"`
	Citations            json.RawMessage   `json:"citations"`
	Governance           json.RawMessage   `json:"governance"`
}

type aiDiagnosisItem struct {
	ConcernKey            string          `json:"concern_key"`
	Name                  string          `json:"name"`
	Confidence            string          `json:"confidence"`
	Severity              *string         `json:"severity"`
	EvidenceStrength      *string         `json:"evidence_strength"`
	Impact                *string         `json:"impact"`
	Basis                 string          `json:"basis"`
	TypicalSymptoms       string          `json:"typical_symptoms"`
	Differential          *string         `json:"differential"`
	BasisFactIDs          json.RawMessage `json:"basis_fact_ids"`
	BasisObservationIDs   json.RawMessage `json:"basis_observation_ids"`
	SupportingEvidenceIDs json.RawMessage `json:"supporting_evidence_ids"`
	CounterevidenceIDs    json.RawMessage `json:"counterevidence_ids"`
	ReasoningSummary      string          `json:"reasoning_summary"`
	MissingInformation    json.RawMessage `json:"missing_information"`
	SafetyNotes           json.RawMessage `json:"safety_notes"`
}

// PersistAIResult validates the minimal application invariant and freezes the
// complete analysis against the exact BodyState revision used for reasoning.
func (s *DiagnosisAnalysisService) PersistAIResult(
	ctx context.Context,
	userID uuid.UUID,
	bodyStateRevision int64,
	raw json.RawMessage,
) (*model.DiagnosisAnalysisRecord, error) {
	var payload aiDiagnosisPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode diagnosis result: %w", err)
	}
	if payload.Status == "" {
		return nil, fmt.Errorf("diagnosis analysis status is required")
	}
	if payload.Scope == "" {
		payload.Scope = "full_body"
	}
	allowedStatuses := map[string]bool{
		"completed":                true,
		"partial":                  true,
		"insufficient_information": true,
		"safety_blocked":           true,
	}
	if !allowedStatuses[payload.Status] {
		return nil, fmt.Errorf("invalid diagnosis analysis status %q", payload.Status)
	}
	if payload.Status == "completed" && len(payload.Candidates) == 0 {
		return nil, fmt.Errorf("completed diagnosis analysis requires at least one candidate")
	}

	now := time.Now().UTC()
	analysis := &model.DiagnosisAnalysisRecord{
		ID: uuid.New(), UserID: userID, BodyStateRevision: bodyStateRevision,
		Status: payload.Status, Scope: payload.Scope, Summary: payload.Summary,
		CrossConcernPatterns: diagnosisJSON(payload.CrossConcernPatterns, `[]`),
		InformationGaps:      diagnosisJSON(payload.InformationGaps, `[]`),
		SafetySummary:        diagnosisJSON(payload.SafetySummary, `{}`),
		Citations:            diagnosisJSON(payload.Citations, `[]`),
		Governance:           diagnosisJSON(payload.Governance, `{}`),
		RawOutput:            datatypes.JSON(raw),
		CreatedAt:            now,
	}

	candidates := make([]model.DiagnosisCandidateRecord, 0, len(payload.Candidates))
	for index, candidate := range payload.Candidates {
		if candidate.Name == "" || candidate.Confidence == "" {
			return nil, fmt.Errorf("candidate %d is missing name/confidence", index)
		}
		candidateRaw, _ := json.Marshal(candidate)
		candidates = append(candidates, model.DiagnosisCandidateRecord{
			ID: uuid.New(), AnalysisID: analysis.ID, Ordinal: index,
			ConcernKey: candidate.ConcernKey, Name: candidate.Name, Confidence: candidate.Confidence,
			Severity: candidate.Severity, EvidenceStrength: candidate.EvidenceStrength, Impact: candidate.Impact,
			Basis: candidate.Basis, TypicalSymptoms: candidate.TypicalSymptoms, Differential: candidate.Differential,
			BasisFactIDs:          diagnosisJSON(candidate.BasisFactIDs, `[]`),
			BasisObservationIDs:   diagnosisJSON(candidate.BasisObservationIDs, `[]`),
			SupportingEvidenceIDs: diagnosisJSON(candidate.SupportingEvidenceIDs, `[]`),
			CounterevidenceIDs:    diagnosisJSON(candidate.CounterevidenceIDs, `[]`),
			ReasoningSummary:      candidate.ReasoningSummary,
			MissingInformation:    diagnosisJSON(candidate.MissingInformation, `[]`),
			SafetyNotes:           diagnosisJSON(candidate.SafetyNotes, `[]`),
			RawPayload:            datatypes.JSON(candidateRaw),
			CreatedAt:             now,
		})
	}
	if err := s.repo.Create(ctx, analysis, candidates); err != nil {
		return nil, fmt.Errorf("persist diagnosis analysis: %w", err)
	}
	analysis.Candidates = candidates
	return analysis, nil
}

func (s *DiagnosisAnalysisService) List(ctx context.Context, userID uuid.UUID, limit int) ([]model.DiagnosisAnalysisRecord, error) {
	return s.repo.ListByUser(ctx, userID, limit)
}

func (s *DiagnosisAnalysisService) AssessCandidates(ctx context.Context, userID, analysisID uuid.UUID, states map[uuid.UUID]string) ([]model.DiagnosisCandidateAssessment, error) {
	analysis, err := s.repo.GetByID(ctx, analysisID, userID)
	if err != nil {
		return nil, err
	}
	if analysis == nil {
		return nil, fmt.Errorf("diagnosis analysis not found")
	}
	allowed := map[string]bool{"confirmed": true, "unsure": true, "not_applicable": true}
	candidateIDs := make(map[uuid.UUID]struct{}, len(analysis.Candidates))
	for _, candidate := range analysis.Candidates {
		candidateIDs[candidate.ID] = struct{}{}
	}
	for candidateID, state := range states {
		if _, ok := candidateIDs[candidateID]; !ok {
			return nil, fmt.Errorf("candidate %s does not belong to analysis %s", candidateID, analysisID)
		}
		if !allowed[state] {
			return nil, fmt.Errorf("invalid candidate assessment state %q", state)
		}
		assessment := &model.DiagnosisCandidateAssessment{
			ID: uuid.New(), AnalysisID: analysisID, CandidateID: candidateID, UserID: userID, State: state,
			AssessedAt: time.Now().UTC(),
		}
		if err := s.repo.UpsertAssessment(ctx, assessment); err != nil {
			return nil, err
		}
	}
	return s.repo.ListAssessments(ctx, analysisID, userID)
}

func (s *DiagnosisAnalysisService) ListAssessments(ctx context.Context, userID, analysisID uuid.UUID) ([]model.DiagnosisCandidateAssessment, error) {
	return s.repo.ListAssessments(ctx, analysisID, userID)
}

func (s *DiagnosisAnalysisService) GetLatest(ctx context.Context, userID uuid.UUID) (*model.DiagnosisAnalysisRecord, error) {
	return s.repo.GetLatestByUser(ctx, userID)
}

func (s *DiagnosisAnalysisService) GetByID(ctx context.Context, analysisID, userID uuid.UUID) (*model.DiagnosisAnalysisRecord, error) {
	return s.repo.GetByID(ctx, analysisID, userID)
}

// PublicPayload exposes the canonical candidate-oriented DiagnosisAnalysis read model.
func (s *DiagnosisAnalysisService) PublicPayload(analysis *model.DiagnosisAnalysisRecord) map[string]any {
	candidates := make([]map[string]any, 0, len(analysis.Candidates))
	for _, candidate := range analysis.Candidates {
		item := map[string]any{
			"candidate_id":            candidate.ID,
			"concern_key":             candidate.ConcernKey,
			"name":                    candidate.Name,
			"confidence":              candidate.Confidence,
			"basis":                   candidate.Basis,
			"typical_symptoms":        candidate.TypicalSymptoms,
			"basis_fact_ids":          json.RawMessage(candidate.BasisFactIDs),
			"basis_observation_ids":   json.RawMessage(candidate.BasisObservationIDs),
			"supporting_evidence_ids": json.RawMessage(candidate.SupportingEvidenceIDs),
			"counterevidence_ids":     json.RawMessage(candidate.CounterevidenceIDs),
			"reasoning_summary":       candidate.ReasoningSummary,
			"missing_information":     json.RawMessage(candidate.MissingInformation),
			"safety_notes":            json.RawMessage(candidate.SafetyNotes),
		}
		if candidate.Severity != nil {
			item["severity"] = *candidate.Severity
		}
		if candidate.EvidenceStrength != nil {
			item["evidence_strength"] = *candidate.EvidenceStrength
		}
		if candidate.Impact != nil {
			item["impact"] = *candidate.Impact
		}
		if candidate.Differential != nil {
			item["differential"] = *candidate.Differential
		}
		candidates = append(candidates, item)
	}
	return map[string]any{
		"analysis_id":            analysis.ID,
		"body_state_revision":    analysis.BodyStateRevision,
		"status":                 analysis.Status,
		"scope":                  analysis.Scope,
		"summary":                analysis.Summary,
		"candidates":             candidates,
		"cross_concern_patterns": json.RawMessage(analysis.CrossConcernPatterns),
		"information_gaps":       json.RawMessage(analysis.InformationGaps),
		"safety_summary":         json.RawMessage(analysis.SafetySummary),
		"citations":              json.RawMessage(analysis.Citations),
		"governance":             json.RawMessage(analysis.Governance),
		"created_at":             analysis.CreatedAt,
	}
}

func diagnosisJSON(raw json.RawMessage, fallback string) datatypes.JSON {
	if len(raw) == 0 || string(raw) == "null" {
		return datatypes.JSON(fallback)
	}
	return datatypes.JSON(raw)
}
