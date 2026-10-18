package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type workspaceProfileSource interface {
	GetProfile(ctx context.Context, userID uuid.UUID) (*model.UserProfile, error)
}

type workspaceConsultationSource interface {
	GetLatestForUser(ctx context.Context, userID uuid.UUID) (*model.ConsultationSession, error)
}

type workspaceBodyStateSource interface {
	GetBodyState(ctx context.Context, userID uuid.UUID, historyLimit int) (*BodyStateReadModel, error)
}

// workspaceDiagnosisSource is the native Diagnosis head: committed Conditions only.
type workspaceDiagnosisSource interface {
	Read(ctx context.Context, owner uuid.UUID) (*NativeDiagnosisView, error)
}

// workspaceDiagnosisRunSource lists the owner's native analysis runs, newest first.
type workspaceDiagnosisRunSource interface {
	List(ctx context.Context, owner uuid.UUID, limit int, before *uuid.UUID) (*NativeDiagnosisRunPage, error)
}

type workspaceOutcomeSource interface {
	ListOutcomes(ctx context.Context, userID uuid.UUID, limit int) ([]model.Outcome, error)
}

type workspaceTrainingSource interface {
	GetActivePlan(ctx context.Context, userID uuid.UUID) (*model.TrainingPlan, error)
}

// HealthWorkspaceService is the single read model for the continuous product
// loop. It derives capabilities directly from durable domain objects.
type HealthWorkspaceService struct {
	profiles      workspaceProfileSource
	consultations workspaceConsultationSource
	bodyState     workspaceBodyStateSource
	diagnosis     workspaceDiagnosisSource
	diagnosisRuns workspaceDiagnosisRunSource
	outcomes      workspaceOutcomeSource
	training      workspaceTrainingSource
}

func NewHealthWorkspaceService(
	profiles workspaceProfileSource,
	consultations workspaceConsultationSource,
	bodyState workspaceBodyStateSource,
	diagnosis workspaceDiagnosisSource,
	diagnosisRuns workspaceDiagnosisRunSource,
	outcomes workspaceOutcomeSource,
	training workspaceTrainingSource,
) *HealthWorkspaceService {
	return &HealthWorkspaceService{
		profiles: profiles, consultations: consultations, bodyState: bodyState,
		diagnosis: diagnosis, diagnosisRuns: diagnosisRuns, outcomes: outcomes, training: training,
	}
}

func (s *HealthWorkspaceService) AttachTrainingSource(training workspaceTrainingSource) {
	s.training = training
}

func (s *HealthWorkspaceService) Get(
	ctx context.Context,
	userID uuid.UUID,
) (*HealthWorkspace, error) {
	workspace := &HealthWorkspace{
		GeneratedAt:    time.Now().UTC(),
		RecentOutcomes: []model.Outcome{}, Trends: []HealthWorkspaceTrend{},
		Actions: []HealthWorkspaceAction{},
	}
	if s.profiles != nil {
		profile, err := s.profiles.GetProfile(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("load workspace profile: %w", err)
		}
		workspace.ProfileReady = profile != nil
	}
	if s.consultations != nil {
		session, err := s.consultations.GetLatestForUser(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("load workspace consultation: %w", err)
		}
		if session != nil {
			workspace.ConversationID = &session.ConversationID
		}
	}

	snapshot, err := s.bodyState.GetBodyState(ctx, userID, 30)
	if err != nil {
		return nil, fmt.Errorf("load workspace body state: %w", err)
	}

	diagnosis, err := s.diagnosis.Read(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load workspace diagnosis: %w", err)
	}
	if diagnosis == nil || diagnosis.UserID != userID {
		return nil, fmt.Errorf("load workspace diagnosis: owner mismatch")
	}
	var latestRun *NativeDiagnosisRunSummary
	runs, err := s.diagnosisRuns.List(ctx, userID, 1, nil)
	if err != nil {
		return nil, fmt.Errorf("load workspace diagnosis runs: %w", err)
	}
	if runs != nil && len(runs.Runs) > 0 {
		latestRun = &runs.Runs[0]
	}

	if s.training != nil {
		workspace.TrainingPlan, err = s.training.GetActivePlan(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("load workspace training plan: %w", err)
		}
	}
	workspace.RecentOutcomes, err = s.outcomes.ListOutcomes(ctx, userID, 100)
	if err != nil {
		return nil, fmt.Errorf("load workspace outcomes: %w", err)
	}
	workspace.Trends = deriveWorkspaceTrends(workspace.RecentOutcomes)
	workspace.Capabilities = deriveWorkspaceCapabilities(
		snapshot, diagnosis, latestRun, workspace.TrainingPlan,
	)
	workspace.Actions = deriveWorkspaceActions(workspace)
	return workspace, nil
}

// deriveWorkspaceCapabilities gates the loop from durable state. Generation
// only creates a pending PlanProposal and mirrors the Treatment gate: at least
// one adopted native Condition and complete Safety coverage. Execution needs an
// active TrainingPlan and complete Safety coverage. Diagnosis review is owed
// while the newest native analysis run is still open for the user's decisions
// or an adopted Condition is stale (its derived freshness, when the reviewed
// reader supplied one); a stale adopted Condition also blocks generation,
// mirroring the Treatment DIAGNOSIS_STALE gate. review_recommended is a
// dismissible prompt on the Condition itself and owes nothing here.
func deriveWorkspaceCapabilities(
	snapshot *BodyStateReadModel,
	diagnosis *NativeDiagnosisView,
	latestRun *NativeDiagnosisRunSummary,
	trainingPlan *model.TrainingPlan,
) HealthWorkspaceCapabilities {
	envelope, safetyErr := ProjectSafetyEnvelopeV2(snapshot)
	requiresSafety := safetyErr != nil || envelope.RequiresReview || len(envelope.Coverage.IncompleteSourceRefs) > 0
	diagnosisBlocked := safetyErr != nil
	for _, blocker := range envelope.ActiveBlockers {
		if blocker.RiskLevel == "urgent" {
			diagnosisBlocked = true
		}
	}
	trainingSafetyIncomplete := requiresSafety || !envelope.Coverage.Complete
	hasReasoningInput := len(snapshot.Facts) > 0 || len(snapshot.Observations) > 0
	hasDiagnosis := (diagnosis != nil && diagnosis.CurrentRevision > 0) || latestRun != nil
	adoptedConditions := 0
	staleAdopted := false
	if diagnosis != nil {
		adopted := adoptedTreatmentDiagnosis(diagnosis).ConditionEntries
		adoptedConditions = len(adopted)
		for _, condition := range adopted {
			staleAdopted = staleAdopted || condition.Freshness.Stale()
		}
	}
	diagnosisReview := (latestRun != nil && latestRun.State == "open") || staleAdopted
	diagnosisEligible := treatmentDiagnosisFactsReady(TreatmentDecisionFacts{
		DiagnosisRevision: diagnosisRevision(diagnosis), AdoptedConditionCount: adoptedConditions,
	})
	activePlan := trainingPlan != nil && trainingPlan.Status == "active"
	return HealthWorkspaceCapabilities{
		CanContinueConsultation: true,
		CanEditBodyState:        true,
		CanRequestDiagnosis:     hasReasoningInput && !diagnosisBlocked,
		CanReviewDiagnosis:      hasDiagnosis,
		CanGenerateTreatment:    diagnosisEligible && !staleAdopted && !trainingSafetyIncomplete,
		CanExecuteTreatment:     activePlan && !trainingSafetyIncomplete,
		CanRecordOutcome:        activePlan,
		RequiresSafetyReview:    requiresSafety,
		RequiresDiagnosisReview: diagnosisReview,
	}
}

func diagnosisRevision(diagnosis *NativeDiagnosisView) int64 {
	if diagnosis == nil {
		return 0
	}
	return diagnosis.CurrentRevision
}

func deriveWorkspaceActions(workspace *HealthWorkspace) []HealthWorkspaceAction {
	caps := workspace.Capabilities
	actions := make([]HealthWorkspaceAction, 0, 9)
	add := func(kind string, priority int, enabled bool, reason string, target map[string]any) {
		actions = append(actions, HealthWorkspaceAction{
			Kind: kind, Priority: priority, Enabled: enabled, Reason: reason, Target: target,
		})
	}
	if !workspace.ProfileReady {
		add("complete_profile", 110, true, "完善身体档案后再进入长期健康管理。", map[string]any{"route": "/onboarding"})
	}
	if caps.RequiresSafetyReview {
		add("review_safety", 100, true, "存在需要优先审核的安全状态。", map[string]any{"section": "safety"})
	}
	if caps.RequiresDiagnosisReview {
		add("review_diagnosis", 85, true, "Diagnosis 有待你确认的分析变更，或已采纳的条件所依据的资料已变化。", map[string]any{"section": "diagnosis"})
	}
	if caps.CanExecuteTreatment && workspace.TrainingPlan != nil {
		add("open_training", 75, true, "当前训练计划可以继续执行。", map[string]any{"route": workspaceTreatmentRoute(workspace.ConversationID)})
	}
	if caps.CanGenerateTreatment {
		add("generate_treatment", 70, true, "已采纳的 Diagnosis 条件可用于生成一个需你确认的训练计划 proposal。", map[string]any{"section": "diagnosis"})
	}
	if caps.CanRequestDiagnosis {
		add("request_diagnosis", 60, true, "当前 BodyState 已有可分析信息。", map[string]any{"section": "diagnosis"})
	}
	if caps.CanRecordOutcome {
		add("record_outcome", 50, true, "记录干预后的变化以更新长期趋势。", map[string]any{"section": "outcomes"})
	}
	add("continue_consultation", 40, true, "健康状态持续变化，可随时补充或纠正信息。", map[string]any{"conversation_id": workspace.ConversationID})
	sort.SliceStable(actions, func(i, j int) bool { return actions[i].Priority > actions[j].Priority })
	return actions
}

func workspaceTreatmentRoute(conversationID *uuid.UUID) string {
	route := "/consultation"
	if conversationID != nil {
		route += "/" + conversationID.String()
	}
	return route + "?view=treatment"
}

func deriveWorkspaceTrends(
	outcomes []model.Outcome,
) []HealthWorkspaceTrend {
	byKey := map[string]*HealthWorkspaceTrend{}
	for _, outcome := range outcomes {
		key := strings.Join([]string{outcome.ConcernKey, outcome.BodyRegion, outcome.Kind}, "|")
		trend, exists := byKey[key]
		if !exists {
			trend = &HealthWorkspaceTrend{
				Key: key, ConcernKey: outcome.ConcernKey, BodyRegion: outcome.BodyRegion,
				Kind: outcome.Kind, CurrentTrend: outcomeTrend(outcome.Value),
				Points: []HealthWorkspaceTrendPoint{},
			}
			byKey[key] = trend
		}
		trend.Points = append(trend.Points, HealthWorkspaceTrendPoint{
			OccurredAt: outcome.OccurredAt, SourceType: outcome.SourceType,
			Value: json.RawMessage(outcome.Value), Notes: outcome.Notes,
			CausalityLevel: outcome.CausalityLevel,
		})
		if value := outcomeTrend(outcome.Value); value != "" {
			trend.CurrentTrend = value
		}
	}
	trends := make([]HealthWorkspaceTrend, 0, len(byKey))
	for _, trend := range byKey {
		sort.Slice(trend.Points, func(i, j int) bool {
			return trend.Points[i].OccurredAt.Before(trend.Points[j].OccurredAt)
		})
		trends = append(trends, *trend)
	}
	sort.Slice(trends, func(i, j int) bool { return trends[i].Key < trends[j].Key })
	return trends
}

func outcomeTrend(raw []byte) string {
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	trend, _ := value["trend"].(string)
	return strings.TrimSpace(trend)
}
