package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

var (
	ErrTreatmentDiagnosisNotReady           = errors.New("diagnosis analysis is not eligible for treatment")
	ErrTreatmentSafetyBlocked               = errors.New("active safety state blocks ordinary treatment generation or acceptance")
	ErrTreatmentAnalysisStale               = errors.New("diagnosis analysis requires review before treatment")
	ErrTreatmentCandidateAssessmentRequired = errors.New("at least one diagnosis candidate must be assessed as confirmed or unsure")
	ErrTreatmentProposalOutdated            = errors.New("treatment proposal no longer matches the current durable health state")
	ErrTreatmentConfigurationMismatch       = errors.New("treatment Agent configuration identity mismatch")
)

type treatmentRepository interface {
	CreateProposal(ctx context.Context, userID uuid.UUID, revision model.TreatmentRevision, interventions []model.Intervention) (*model.Treatment, *model.TreatmentRevision, error)
	AcceptRevision(ctx context.Context, userID, revisionID uuid.UUID, expectedBodyStateRevision int64, acceptanceDecisionTrace datatypes.JSON) (*model.Treatment, *model.TreatmentRevision, bool, error)
	RejectRevision(ctx context.Context, userID, revisionID uuid.UUID) error
	SetStatus(ctx context.Context, userID uuid.UUID, status string, reasons datatypes.JSON) (*model.Treatment, error)
	GetCurrent(ctx context.Context, userID uuid.UUID) (*model.Treatment, error)
	GetRevision(ctx context.Context, userID, revisionID uuid.UUID) (*model.TreatmentRevision, error)
	ListRevisions(ctx context.Context, userID uuid.UUID, limit int) ([]model.TreatmentRevision, error)
	GetIntervention(ctx context.Context, userID, interventionID uuid.UUID) (*model.Intervention, error)
	CreateOutcome(ctx context.Context, outcome *model.Outcome) (*model.Outcome, bool, error)
	UpdateOutcomeBodyStateRevision(ctx context.Context, outcomeID, userID uuid.UUID, revision int64) error
	ListOutcomes(ctx context.Context, userID uuid.UUID, limit int) ([]model.Outcome, error)
}

type treatmentDiagnosisSource interface {
	GetByID(ctx context.Context, analysisID, userID uuid.UUID) (*model.DiagnosisAnalysisRecord, error)
	GetLatest(ctx context.Context, userID uuid.UUID) (*model.DiagnosisAnalysisRecord, error)
	ListAssessments(ctx context.Context, userID, analysisID uuid.UUID) ([]model.DiagnosisCandidateAssessment, error)
	PublicPayload(analysis *model.DiagnosisAnalysisRecord) map[string]any
}

type treatmentBodyStateSource interface {
	GetSnapshot(ctx context.Context, userID uuid.UUID, historyLimit int) (*BodyStateSnapshot, error)
	ListRevisionsAfter(ctx context.Context, userID uuid.UUID, afterRevision int64, limit int) ([]model.BodyStateRevision, error)
	ListEvidence(ctx context.Context, userID uuid.UUID, limit int) ([]model.BodyStateEvidence, error)
	RecordOutcome(ctx context.Context, userID uuid.UUID, outcome model.Outcome) (*model.BodyStateRevision, error)
}

type treatmentFreshnessSource interface {
	GetOrEvaluate(ctx context.Context, userID uuid.UUID, analysis *model.DiagnosisAnalysisRecord) (*model.DiagnosisAnalysisFreshness, error)
}

type treatmentReasoner interface {
	RecommendTreatment(ctx context.Context, req TreatmentRecommendationRequest) (json.RawMessage, error)
}

type treatmentUnitOfWork interface {
	WithinTransaction(ctx context.Context, fn func(context.Context) error) error
}

type treatmentDeploymentPolicy interface {
	SelectTreatmentRoute(subjectID string) TreatmentRouteSelection
}

type treatmentRolloutObserver interface {
	ObserveProposal(
		ctx context.Context,
		userID uuid.UUID,
		route TreatmentRouteSelection,
		revisionID uuid.UUID,
	) error
}

// TreatmentService owns application orchestration around typed AI proposals.
// The reasoner can recommend; only this service/repository accepts a revision.
type TreatmentService struct {
	repo       treatmentRepository
	diagnosis  treatmentDiagnosisSource
	bodyState  treatmentBodyStateSource
	freshness  treatmentFreshnessSource
	profiles   *ProfileService
	reasoner   treatmentReasoner
	unitOfWork treatmentUnitOfWork
	deployment treatmentDeploymentPolicy
	rollout    treatmentRolloutObserver
}

func NewTreatmentService(
	repo treatmentRepository,
	diagnosis treatmentDiagnosisSource,
	bodyState treatmentBodyStateSource,
	freshness treatmentFreshnessSource,
	profiles *ProfileService,
	reasoner treatmentReasoner,
	unitOfWork treatmentUnitOfWork,
	deployment treatmentDeploymentPolicy,
) *TreatmentService {
	return &TreatmentService{
		repo: repo, diagnosis: diagnosis, bodyState: bodyState,
		freshness: freshness, profiles: profiles, reasoner: reasoner,
		unitOfWork: unitOfWork, deployment: deployment,
	}
}

func (s *TreatmentService) AttachRolloutObserver(observer treatmentRolloutObserver) {
	if s != nil {
		s.rollout = observer
	}
}

type TreatmentProposalInput struct {
	DiagnosisAnalysisID uuid.UUID
	UserConstraints     map[string]any
	ChangeReason        string
}

type treatmentAgentPayload struct {
	Status              string                             `json:"status"`
	Summary             string                             `json:"summary"`
	Goal                string                             `json:"goal"`
	DurationWeeks       int                                `json:"duration_weeks"`
	Interventions       []model.TreatmentInterventionDraft `json:"interventions"`
	DailyHabits         []string                           `json:"daily_habits"`
	ExpectedTimeline    string                             `json:"expected_timeline"`
	WarningSigns        []string                           `json:"warning_signs"`
	ReviewTriggers      []string                           `json:"review_triggers"`
	SafetyNotes         []string                           `json:"safety_notes"`
	EvidenceIDs         []string                           `json:"evidence_ids"`
	Governance          json.RawMessage                    `json:"governance"`
	AgentConfiguration  json.RawMessage                    `json:"agent_configuration"`
	ExecutionProvenance json.RawMessage                    `json:"execution_provenance"`
	EvidenceAcquisition json.RawMessage                    `json:"evidence_acquisition"`
}

func (s *TreatmentService) GenerateProposalForLatest(
	ctx context.Context,
	userID uuid.UUID,
	constraints map[string]any,
	changeReason string,
) (*model.TreatmentRevision, error) {
	analysis, err := s.diagnosis.GetLatest(ctx, userID)
	if err != nil {
		return nil, err
	}
	if analysis == nil {
		return nil, ErrTreatmentDiagnosisNotReady
	}
	return s.GenerateProposal(ctx, userID, TreatmentProposalInput{
		DiagnosisAnalysisID: analysis.ID,
		UserConstraints:     constraints,
		ChangeReason:        changeReason,
	})
}

func (s *TreatmentService) GenerateProposal(
	ctx context.Context,
	userID uuid.UUID,
	input TreatmentProposalInput,
) (*model.TreatmentRevision, error) {
	if s.deployment == nil {
		return nil, errors.New("treatment deployment policy is not configured")
	}
	route := s.deployment.SelectTreatmentRoute(userID.String())
	configurationID := route.ServedConfigurationID
	policyRevision := route.ServedDecisionPolicyRevision
	if configurationID == "" || policyRevision == "" {
		return nil, errors.New("treatment deployment policy returned an invalid route")
	}

	analysis, err := s.diagnosis.GetByID(ctx, input.DiagnosisAnalysisID, userID)
	if err != nil {
		return nil, err
	}
	facts := TreatmentDecisionFacts{
		SafetyState:              json.RawMessage(`{}`),
		CandidateAssessmentReady: true,
		MaterialReviewStatus:     model.TreatmentStatusActive,
	}
	if analysis != nil {
		facts.DiagnosisStatus = analysis.Status
		facts.CandidateCount = len(analysis.Candidates)
	}
	if decision := EvaluateTreatmentDecision(policyRevision, TreatmentDecisionGeneration, facts); decision.Outcome == TreatmentBlock {
		return nil, treatmentDecisionError(decision)
	}

	if s.freshness != nil {
		freshness, freshnessErr := s.freshness.GetOrEvaluate(ctx, userID, analysis)
		if freshnessErr != nil {
			return nil, freshnessErr
		}
		if freshness != nil {
			facts.FreshnessState = freshness.State
		}
		if decision := EvaluateTreatmentDecision(policyRevision, TreatmentDecisionGeneration, facts); decision.Outcome == TreatmentBlock {
			return nil, treatmentDecisionError(decision)
		}
	}

	snapshot, err := s.bodyState.GetSnapshot(ctx, userID, 50)
	if err != nil {
		return nil, err
	}
	facts.SafetyState = snapshot.SafetyState
	facts.CurrentBodyStateRevision = snapshot.CurrentRevision
	if decision := EvaluateTreatmentDecision(policyRevision, TreatmentDecisionGeneration, facts); decision.Outcome == TreatmentBlock {
		return nil, treatmentDecisionError(decision)
	}

	assessments, err := s.diagnosis.ListAssessments(ctx, userID, analysis.ID)
	if err != nil {
		return nil, err
	}
	facts.CandidateAssessmentReady = treatmentCandidateAssessmentsReady(analysis, assessments)
	generationDecision := EvaluateTreatmentDecision(
		policyRevision,
		TreatmentDecisionGeneration,
		facts,
	)
	if generationDecision.Outcome != TreatmentAllowProposal {
		return nil, treatmentDecisionError(generationDecision)
	}

	profilePayload := json.RawMessage(`{}`)
	if s.profiles != nil {
		if profile, profileErr := s.profiles.GetProfile(ctx, userID); profileErr == nil && profile != nil {
			profilePayload = rawJSON(profile, `{}`)
		}
	}
	evidence, err := s.bodyState.ListEvidence(ctx, userID, 50)
	if err != nil {
		return nil, err
	}
	bodyStatePayload := rawJSON(snapshot, `{}`)
	diagnosisPayload := rawJSON(s.diagnosis.PublicPayload(analysis), `{}`)
	assessmentsPayload := rawJSON(assessments, `[]`)
	constraintsPayload := rawJSON(input.UserConstraints, `{}`)
	evidencePayload := rawJSON(evidence, `[]`)
	replayInput, err := EncodeTreatmentReplayInput(
		snapshot.CurrentRevision,
		bodyStatePayload,
		diagnosisPayload,
		assessmentsPayload,
		profilePayload,
		constraintsPayload,
		evidencePayload,
		facts,
	)
	if err != nil {
		return nil, fmt.Errorf("freeze Treatment replay input: %w", err)
	}

	raw, err := s.reasoner.RecommendTreatment(ctx, TreatmentRecommendationRequest{
		UserID:               userID.String(),
		ConfigurationID:      configurationID,
		BodyStateRevision:    snapshot.CurrentRevision,
		BodyState:            bodyStatePayload,
		DiagnosisAnalysis:    diagnosisPayload,
		CandidateAssessments: assessmentsPayload,
		Profile:              profilePayload,
		UserConstraints:      constraintsPayload,
		Evidence:             evidencePayload,
	})
	if err != nil {
		return nil, err
	}
	payload, err := normalizeTreatmentAgentPayload(raw)
	if err != nil {
		return nil, err
	}
	if err := validateTreatmentAgentIdentity(payload, configurationID); err != nil {
		return nil, err
	}
	plan := model.TreatmentPlanContent{
		Summary: payload.Summary, Goal: payload.Goal, DurationWeeks: payload.DurationWeeks,
		Interventions: payload.Interventions, DailyHabits: payload.DailyHabits,
		ExpectedTimeline: payload.ExpectedTimeline, WarningSigns: payload.WarningSigns,
		ReviewTriggers: payload.ReviewTriggers, SafetyNotes: payload.SafetyNotes,
	}
	if err := validateTreatmentPlan(plan); err != nil {
		return nil, err
	}
	interventions := make([]model.Intervention, 0, len(plan.Interventions))
	for _, item := range plan.Interventions {
		interventions = append(interventions, model.Intervention{
			Kind: item.Kind, Title: item.Title, Description: item.Description,
			Prescription: datatypes.JSON(rawJSON(item.Prescription, `{}`)),
		})
	}
	evidenceIDs := make([]uuid.UUID, 0, len(payload.EvidenceIDs))
	for _, rawID := range payload.EvidenceIDs {
		if parsed, parseErr := uuid.Parse(rawID); parseErr == nil {
			evidenceIDs = append(evidenceIDs, parsed)
		}
	}
	generationTrace := buildTreatmentDecisionTrace(
		generationDecision,
		facts,
		analysis.ID,
		configurationID,
	)
	_, revision, err := s.repo.CreateProposal(ctx, userID, model.TreatmentRevision{
		SourceBodyStateRevision:   snapshot.CurrentRevision,
		SourceDiagnosisAnalysisID: analysis.ID,
		Goal:                      plan.Goal,
		DurationWeeks:             plan.DurationWeeks,
		Plan:                      datatypes.JSON(rawJSON(plan, `{}`)),
		UserConstraints:           datatypes.JSON(rawJSON(input.UserConstraints, `{}`)),
		EvidenceIDs:               datatypes.JSON(rawJSON(evidenceIDs, `[]`)),
		Governance:                datatypes.JSON(normalizeRaw(payload.Governance, `{}`)),
		AgentConfigurationID:      configurationID,
		AgentConfiguration:        datatypes.JSON(normalizeRaw(payload.AgentConfiguration, `{}`)),
		ExecutionProvenance:       datatypes.JSON(normalizeRaw(payload.ExecutionProvenance, `{}`)),
		EvidenceAcquisitionTrace:  datatypes.JSON(normalizeRaw(payload.EvidenceAcquisition, `{}`)),
		GenerationDecisionTrace:   generationTrace,
		ReplayInput:               datatypes.JSON(replayInput),
		RolloutProvenance:         datatypes.JSON(rawJSON(route, `{}`)),
		ChangeReason:              strings.TrimSpace(input.ChangeReason),
	}, interventions)
	if err != nil {
		return nil, err
	}
	if s.rollout != nil && route.ShadowConfigurationID != "" {
		if observeErr := s.rollout.ObserveProposal(ctx, userID, route, revision.ID); observeErr != nil {
			log.Printf("failed to record Treatment %s shadow observation for revision %s: %v", route.Stage, revision.ID, observeErr)
		}
	}
	return revision, nil
}

func (s *TreatmentService) AcceptProposal(ctx context.Context, userID, revisionID uuid.UUID) (*model.Treatment, error) {
	revision, err := s.repo.GetRevision(ctx, userID, revisionID)
	if err != nil {
		return nil, err
	}
	if revision == nil {
		return nil, ErrTreatmentDiagnosisNotReady
	}
	if revision.AcceptanceState != model.TreatmentAcceptanceProposed {
		return nil, fmt.Errorf("only proposed treatment revisions can be accepted")
	}

	snapshot, err := s.bodyState.GetSnapshot(ctx, userID, 0)
	if err != nil {
		return nil, err
	}
	facts := TreatmentDecisionFacts{
		SafetyState:              snapshot.SafetyState,
		CandidateAssessmentReady: true,
		ProposalAcceptanceState:  revision.AcceptanceState,
		SourceBodyStateRevision:  revision.SourceBodyStateRevision,
		CurrentBodyStateRevision: snapshot.CurrentRevision,
		MaterialReviewStatus:     model.TreatmentStatusActive,
		MaterialReviewReasons:    []TreatmentReviewReason{},
		DiagnosisStatus:          "completed",
		CandidateCount:           1,
		FreshnessState:           model.DiagnosisFreshnessFresh,
	}
	if decision := EvaluateTreatmentDecision(TreatmentDecisionPolicyV1, TreatmentDecisionAcceptance, facts); decision.Outcome == TreatmentBlock {
		return nil, treatmentDecisionError(decision)
	}

	analysis, err := s.diagnosis.GetByID(ctx, revision.SourceDiagnosisAnalysisID, userID)
	if err != nil {
		return nil, err
	}
	facts.DiagnosisStatus = ""
	facts.CandidateCount = 0
	if analysis != nil {
		facts.DiagnosisStatus = analysis.Status
		facts.CandidateCount = len(analysis.Candidates)
	}
	if decision := EvaluateTreatmentDecision(TreatmentDecisionPolicyV1, TreatmentDecisionAcceptance, facts); decision.Outcome == TreatmentBlock {
		return nil, treatmentDecisionError(decision)
	}

	if s.freshness != nil {
		freshness, freshnessErr := s.freshness.GetOrEvaluate(ctx, userID, analysis)
		if freshnessErr != nil {
			return nil, freshnessErr
		}
		facts.FreshnessState = ""
		if freshness != nil {
			facts.FreshnessState = freshness.State
		}
		if decision := EvaluateTreatmentDecision(TreatmentDecisionPolicyV1, TreatmentDecisionAcceptance, facts); decision.Outcome == TreatmentBlock {
			return nil, treatmentDecisionError(decision)
		}
	}
	assessments, err := s.diagnosis.ListAssessments(ctx, userID, analysis.ID)
	if err != nil {
		return nil, err
	}
	facts.CandidateAssessmentReady = treatmentCandidateAssessmentsReady(analysis, assessments)
	if decision := EvaluateTreatmentDecision(TreatmentDecisionPolicyV1, TreatmentDecisionAcceptance, facts); decision.Outcome == TreatmentBlock {
		return nil, treatmentDecisionError(decision)
	}

	if snapshot.CurrentRevision > revision.SourceBodyStateRevision {
		revisions, revisionErr := s.bodyState.ListRevisionsAfter(ctx, userID, revision.SourceBodyStateRevision, 500)
		if revisionErr != nil {
			return nil, revisionErr
		}
		facts.MaterialReviewStatus, facts.MaterialReviewReasons = EvaluateTreatmentReviewPolicy(analysis, revisions)
	}
	acceptanceDecision := EvaluateTreatmentDecision(
		TreatmentDecisionPolicyV1,
		TreatmentDecisionAcceptance,
		facts,
	)
	if acceptanceDecision.Outcome != TreatmentAllowAcceptance {
		return nil, treatmentDecisionError(acceptanceDecision)
	}
	acceptanceTrace := buildTreatmentDecisionTrace(
		acceptanceDecision,
		facts,
		analysis.ID,
		revision.AgentConfigurationID,
	)

	treatment, _, accepted, err := s.repo.AcceptRevision(
		ctx,
		userID,
		revisionID,
		snapshot.CurrentRevision,
		acceptanceTrace,
	)
	if err != nil {
		return nil, err
	}
	if !accepted {
		return nil, ErrTreatmentProposalOutdated
	}
	return treatment, nil
}

func (s *TreatmentService) RejectProposal(ctx context.Context, userID, revisionID uuid.UUID) error {
	return s.repo.RejectRevision(ctx, userID, revisionID)
}

func (s *TreatmentService) GetCurrent(ctx context.Context, userID uuid.UUID) (*model.Treatment, error) {
	return s.repo.GetCurrent(ctx, userID)
}

// PreviewCurrentReview returns the deterministic current review projection
// without mutating Treatment or Intervention rows. GET/read-model paths use this
// method; only explicit review and mutation boundaries persist status changes.
func (s *TreatmentService) PreviewCurrentReview(ctx context.Context, userID uuid.UUID) (*model.Treatment, error) {
	current, err := s.repo.GetCurrent(ctx, userID)
	if err != nil || current == nil || current.Current == nil {
		return current, err
	}
	status, reasons, err := s.evaluateCurrentReviewState(ctx, userID, current)
	if err != nil {
		return nil, err
	}
	projected := *current
	projected.Status = status
	projected.StatusReasons = datatypes.JSON(rawJSON(reasons, `[]`))
	return &projected, nil
}

func (s *TreatmentService) GetRevision(ctx context.Context, userID, revisionID uuid.UUID) (*model.TreatmentRevision, error) {
	return s.repo.GetRevision(ctx, userID, revisionID)
}

func (s *TreatmentService) ListRevisions(ctx context.Context, userID uuid.UUID, limit int) ([]model.TreatmentRevision, error) {
	return s.repo.ListRevisions(ctx, userID, limit)
}

func (s *TreatmentService) ListOutcomes(ctx context.Context, userID uuid.UUID, limit int) ([]model.Outcome, error) {
	return s.repo.ListOutcomes(ctx, userID, limit)
}

// RecordOutcome closes the feedback loop with idempotent source identity. The
// Outcome is persisted first, then its accepted semantic effect creates a
// BodyState revision and may recommend review of the current plan.
func (s *TreatmentService) RecordOutcome(ctx context.Context, userID uuid.UUID, outcome model.Outcome) (*model.Outcome, bool, error) {
	outcome.UserID = userID
	if outcome.SourceType == "" || strings.TrimSpace(outcome.SourceKey) == "" || outcome.Kind == "" {
		return nil, false, errors.New("outcome source_type, source_key and kind are required")
	}
	if outcome.CausalityLevel == "" {
		outcome.CausalityLevel = "association_only"
	}
	if outcome.CausalityLevel == "association_only" && outcome.AssociationStatement == "" {
		outcome.AssociationStatement = "该变化发生在干预之后，表示时间关联，不代表已经证明因果关系。"
	}
	if outcome.InterventionID != nil {
		intervention, err := s.repo.GetIntervention(ctx, userID, *outcome.InterventionID)
		if err != nil {
			return nil, false, err
		}
		if intervention == nil {
			return nil, false, errors.New("intervention not found")
		}
		outcome.TreatmentID = &intervention.TreatmentID
		outcome.TreatmentRevisionID = &intervention.TreatmentRevisionID
	}
	if s.unitOfWork == nil {
		return nil, false, errors.New("treatment unit of work is not configured")
	}
	var stored *model.Outcome
	var created bool
	err := s.unitOfWork.WithinTransaction(ctx, func(txCtx context.Context) error {
		var createErr error
		stored, created, createErr = s.repo.CreateOutcome(txCtx, &outcome)
		if createErr != nil {
			return createErr
		}
		if stored == nil {
			return errors.New("outcome repository returned no record")
		}
		if !created && stored.BodyStateRevision != nil {
			return nil
		}

		revision, projectionErr := s.bodyState.RecordOutcome(txCtx, userID, *stored)
		if projectionErr != nil {
			return projectionErr
		}
		if revision == nil || revision.Revision <= 0 {
			return errors.New("outcome projection did not produce a BodyState revision")
		}
		stored.BodyStateRevision = &revision.Revision
		if updateErr := s.repo.UpdateOutcomeBodyStateRevision(txCtx, stored.ID, userID, revision.Revision); updateErr != nil {
			return updateErr
		}
		_, reviewErr := s.EvaluateCurrentReview(txCtx, userID)
		return reviewErr
	})
	if err != nil {
		return nil, false, err
	}
	return stored, created, nil
}

// EvaluateCurrentReview is the explicit write boundary for deterministic review
// policy. It never asks the model whether a safety gate should be enforced.
func (s *TreatmentService) EvaluateCurrentReview(ctx context.Context, userID uuid.UUID) (*model.Treatment, error) {
	current, err := s.repo.GetCurrent(ctx, userID)
	if err != nil || current == nil || current.Current == nil {
		return current, err
	}
	status, reasons, err := s.evaluateCurrentReviewState(ctx, userID, current)
	if err != nil {
		return nil, err
	}
	reasonJSON := datatypes.JSON(rawJSON(reasons, `[]`))
	if status == current.Status && string(normalizeRaw(json.RawMessage(current.StatusReasons), `[]`)) == string(reasonJSON) {
		return current, nil
	}
	return s.repo.SetStatus(ctx, userID, status, reasonJSON)
}

func (s *TreatmentService) evaluateCurrentReviewState(
	ctx context.Context,
	userID uuid.UUID,
	current *model.Treatment,
) (string, []TreatmentReviewReason, error) {
	if current == nil || current.Current == nil {
		return "", nil, nil
	}
	if current.Status == model.TreatmentStatusCompleted || current.Status == model.TreatmentStatusSuperseded {
		return current.Status, decodeTreatmentReviewReasons(json.RawMessage(current.StatusReasons)), nil
	}
	snapshot, err := s.bodyState.GetSnapshot(ctx, userID, 0)
	if err != nil {
		return "", nil, err
	}
	if bodyStateRequiresSafetyReview(snapshot.SafetyState) {
		return model.TreatmentStatusPaused, []TreatmentReviewReason{{
			Code:    "active_safety_state",
			Message: "当前安全状态要求暂停普通干预并优先审核。",
		}}, nil
	}
	revisions, err := s.bodyState.ListRevisionsAfter(ctx, userID, current.Current.SourceBodyStateRevision, 500)
	if err != nil {
		return "", nil, err
	}
	analysis, err := s.diagnosis.GetByID(ctx, current.Current.SourceDiagnosisAnalysisID, userID)
	if err != nil {
		return "", nil, err
	}
	status, reasons := EvaluateTreatmentReviewPolicy(analysis, revisions)
	return status, reasons, nil
}

func decodeTreatmentReviewReasons(raw json.RawMessage) []TreatmentReviewReason {
	var reasons []TreatmentReviewReason
	_ = json.Unmarshal(raw, &reasons)
	return reasons
}

type TreatmentReviewReason struct {
	Code       string `json:"code"`
	Revision   int64  `json:"revision,omitempty"`
	ChangeType string `json:"change_type,omitempty"`
	ConcernKey string `json:"concern_key,omitempty"`
	Message    string `json:"message"`
}

func EvaluateTreatmentReviewPolicy(
	analysis *model.DiagnosisAnalysisRecord,
	revisions []model.BodyStateRevision,
) (string, []TreatmentReviewReason) {
	if len(revisions) == 0 {
		return model.TreatmentStatusActive, []TreatmentReviewReason{}
	}
	concerns := map[string]struct{}{}
	referencedFacts := map[string]struct{}{}
	if analysis != nil {
		for _, candidate := range analysis.Candidates {
			if candidate.ConcernKey != "" && candidate.ConcernKey != "general" {
				concerns[candidate.ConcernKey] = struct{}{}
			}
			for _, id := range decodeStringList(candidate.BasisFactIDs) {
				referencedFacts[id] = struct{}{}
			}
		}
	}
	reasons := make([]TreatmentReviewReason, 0)
	status := model.TreatmentStatusActive
	for _, revision := range revisions {
		if strings.HasPrefix(revision.ChangeType, "safety.") {
			return model.TreatmentStatusPaused, []TreatmentReviewReason{{
				Code: "safety_state_changed", Revision: revision.Revision,
				ChangeType: revision.ChangeType, Message: "安全状态变化要求暂停并审核当前方案。",
			}}
		}
		var changes map[string]any
		_ = json.Unmarshal(revision.Changes, &changes)
		concernKey := ""
		itemID := ""
		switch revision.ChangeType {
		case "fact.added":
			fact := mapValue(changes["fact"])
			concernKey = valueString(fact["concern_key"])
			if isSafetyKind(valueString(fact["kind"])) {
				return model.TreatmentStatusPaused, []TreatmentReviewReason{{Code: "new_safety_fact", Revision: revision.Revision, ChangeType: revision.ChangeType, ConcernKey: concernKey, Message: "新增安全相关事实要求暂停当前方案。"}}
			}
		case "fact.updated", "fact.temporal_changed":
			itemID = valueString(changes["fact_id"])
			concernKey = valueString(mapValue(changes["after"])["concern_key"])
		case "fact.corrected":
			itemID = valueString(changes["corrected_fact_id"])
			concernKey = mapConcern(changes["replacement"])
		case "observation.added":
			concernKey = mapConcern(changes["observation"])
		case "observation.updated":
			concernKey = mapConcern(changes["after"])
		default:
			continue
		}
		_, referenced := referencedFacts[itemID]
		_, related := concerns[concernKey]
		if referenced || related {
			status = model.TreatmentStatusReviewRecommended
			reasons = append(reasons, TreatmentReviewReason{
				Code: "material_related_body_state_change", Revision: revision.Revision,
				ChangeType: revision.ChangeType, ConcernKey: concernKey,
				Message: "与当前方案相关的身体状态发生变化，建议审核而不是自动改写方案。",
			})
		}
	}
	return status, reasons
}

func treatmentCandidateAssessmentsReady(
	analysis *model.DiagnosisAnalysisRecord,
	assessments []model.DiagnosisCandidateAssessment,
) bool {
	if analysis == nil || len(analysis.Candidates) == 0 {
		return false
	}
	candidateIDs := make(map[uuid.UUID]struct{}, len(analysis.Candidates))
	for _, candidate := range analysis.Candidates {
		candidateIDs[candidate.ID] = struct{}{}
	}
	for _, assessment := range assessments {
		if _, belongs := candidateIDs[assessment.CandidateID]; !belongs {
			continue
		}
		if assessment.State == "confirmed" || assessment.State == "unsure" {
			return true
		}
	}
	return false
}

func bodyStateRequiresSafetyReview(raw json.RawMessage) bool {
	requiresReview, err := strictTreatmentSafetyReview(raw)
	return err != nil || requiresReview
}

func treatmentDecisionError(decision TreatmentDecision) error {
	if len(decision.Reasons) == 0 {
		return ErrTreatmentSafetyBlocked
	}
	switch decision.Reasons[0] {
	case "diagnosis_not_ready":
		return ErrTreatmentDiagnosisNotReady
	case "diagnosis_not_fresh":
		return ErrTreatmentAnalysisStale
	case "active_body_state_safety_concern", "malformed_or_unknown_safety_state",
		"unsupported_decision_policy_revision", "unsupported_decision_phase":
		return fmt.Errorf("%w: %s", ErrTreatmentSafetyBlocked, decision.Reasons[0])
	case "candidate_assessment_required":
		return ErrTreatmentCandidateAssessmentRequired
	case "proposal_not_proposed":
		return errors.New("only proposed treatment revisions can be accepted")
	case "body_state_revision_regressed", "material_related_body_state_change":
		return ErrTreatmentProposalOutdated
	default:
		return fmt.Errorf("%w: %s", ErrTreatmentSafetyBlocked, decision.Reasons[0])
	}
}

func buildTreatmentDecisionTrace(
	decision TreatmentDecision,
	facts TreatmentDecisionFacts,
	diagnosisAnalysisID uuid.UUID,
	agentConfigurationID string,
) datatypes.JSON {
	trace, _ := json.Marshal(map[string]any{
		"trace_revision":         TreatmentDecisionTraceV1,
		"policy_revision":        decision.PolicyRevision,
		"phase":                  decision.Phase,
		"outcome":                decision.Outcome,
		"reasons":                decision.Reasons,
		"diagnosis_analysis_id":  diagnosisAnalysisID,
		"agent_configuration_id": agentConfigurationID,
		"facts":                  facts,
	})
	return datatypes.JSON(trace)
}

func normalizeTreatmentAgentPayload(raw json.RawMessage) (*treatmentAgentPayload, error) {
	var payload treatmentAgentPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode treatment recommendation: %w", err)
	}
	if payload.Status == "" {
		return nil, errors.New("treatment status is required")
	}
	if payload.Status != "proposed" {
		return nil, fmt.Errorf("unexpected treatment status %q", payload.Status)
	}
	return &payload, nil
}

func validateTreatmentAgentIdentity(payload *treatmentAgentPayload, expectedConfigurationID string) error {
	if payload == nil || len(payload.AgentConfiguration) == 0 {
		return ErrTreatmentConfigurationMismatch
	}
	var configuration struct {
		ID                     string `json:"id"`
		Role                   string `json:"role"`
		DecisionPolicyRevision string `json:"decision_policy_revision"`
	}
	if err := json.Unmarshal(payload.AgentConfiguration, &configuration); err != nil {
		return ErrTreatmentConfigurationMismatch
	}
	registration, ok := knownTreatmentConfigurations[expectedConfigurationID]
	if !ok || configuration.ID != expectedConfigurationID || configuration.Role != "treatment" ||
		configuration.DecisionPolicyRevision != registration.DecisionPolicyRevision {
		return ErrTreatmentConfigurationMismatch
	}
	var execution struct {
		Status       string `json:"status"`
		Runtime      string `json:"runtime"`
		LogicalModel string `json:"logical_model"`
	}
	if len(payload.ExecutionProvenance) == 0 ||
		json.Unmarshal(payload.ExecutionProvenance, &execution) != nil ||
		execution.Status != "executed" || execution.Runtime != "pydantic-ai" ||
		execution.LogicalModel != registration.LogicalModel {
		return ErrTreatmentConfigurationMismatch
	}
	return nil
}

func validateTreatmentPlan(plan model.TreatmentPlanContent) error {
	if strings.TrimSpace(plan.Goal) == "" {
		return errors.New("treatment goal is required")
	}
	if plan.DurationWeeks <= 0 || plan.DurationWeeks > 104 {
		return errors.New("treatment duration_weeks must be between 1 and 104")
	}
	if len(plan.Interventions) == 0 {
		return errors.New("treatment requires at least one intervention")
	}
	for index, intervention := range plan.Interventions {
		if strings.TrimSpace(intervention.Kind) == "" || strings.TrimSpace(intervention.Title) == "" {
			return fmt.Errorf("intervention %d requires kind and title", index)
		}
	}
	return nil
}

func rawJSON(value any, fallback string) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil || len(encoded) == 0 || string(encoded) == "null" {
		return json.RawMessage(fallback)
	}
	return json.RawMessage(encoded)
}

func normalizeRaw(value json.RawMessage, fallback string) json.RawMessage {
	if len(value) == 0 || !json.Valid(value) {
		return json.RawMessage(fallback)
	}
	return value
}
