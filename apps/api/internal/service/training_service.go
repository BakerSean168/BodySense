package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type trainingRepository interface {
	CreatePlan(ctx context.Context, plan *model.TrainingPlan) error
	GetPlanByID(ctx context.Context, id, userID uuid.UUID) (*model.TrainingPlan, error)
	ListPlansByUserID(ctx context.Context, userID uuid.UUID) ([]model.TrainingPlan, error)
	GetActivePlanByUserID(ctx context.Context, userID uuid.UUID) (*model.TrainingPlan, error)
	GetPlanByTreatmentRevision(ctx context.Context, userID, treatmentRevisionID uuid.UUID) (*model.TrainingPlan, error)
	SupersedePlansExcept(ctx context.Context, userID, treatmentRevisionID uuid.UUID) error
	GetOrCreateLog(ctx context.Context, plan *model.TrainingPlan, date time.Time) (*model.TrainingLog, error)
	SaveLog(ctx context.Context, log *model.TrainingLog) error
	CheckInAndGet(ctx context.Context, plan *model.TrainingPlan, date time.Time) (*model.TrainingLog, error)
	MarkOutcomeRecorded(ctx context.Context, logID uuid.UUID, at time.Time) error
	GetLogsByPlanID(ctx context.Context, planID uuid.UUID) ([]model.TrainingLog, error)
	GetConsecutiveCheckInDays(ctx context.Context, planID uuid.UUID) (int, error)
}

type trainingTreatmentService interface {
	AcceptProposal(ctx context.Context, userID, revisionID uuid.UUID) (*model.Treatment, error)
	RecordOutcome(ctx context.Context, userID uuid.UUID, outcome model.Outcome) (*model.Outcome, bool, error)
	GetCurrent(ctx context.Context, userID uuid.UUID) (*model.Treatment, error)
	GenerateProposal(ctx context.Context, userID uuid.UUID, input TreatmentProposalInput) (*model.TreatmentRevision, error)
}

var ErrTrainingProjectionFailed = errors.New("failed to create executable training projection")

// TrainingService adapts accepted Treatment interventions into the daily-task
// execution UI. TreatmentRevision is the source of truth; the training tables
// are a transactional execution projection plus activity log.
type TrainingService struct {
	trainingRepo trainingRepository
	treatment    trainingTreatmentService
	unitOfWork   treatmentUnitOfWork
}

func NewTrainingService(
	trainingRepo trainingRepository,
	treatment trainingTreatmentService,
	unitOfWork treatmentUnitOfWork,
) *TrainingService {
	return &TrainingService{trainingRepo: trainingRepo, treatment: treatment, unitOfWork: unitOfWork}
}

// AcceptTreatmentAndEnsurePlan is the only activation boundary. Treatment
// acceptance and its executable TrainingPlan projection commit or roll back as
// one synchronous database transaction.
func (s *TrainingService) AcceptTreatmentAndEnsurePlan(
	ctx context.Context,
	userID, revisionID uuid.UUID,
	consultationID *uuid.UUID,
) (*model.Treatment, *model.TrainingPlan, error) {
	if s.unitOfWork == nil {
		return nil, nil, errors.New("training unit of work is not configured")
	}
	var treatment *model.Treatment
	var plan *model.TrainingPlan
	err := s.unitOfWork.WithinTransaction(ctx, func(txCtx context.Context) error {
		var acceptErr error
		treatment, acceptErr = s.treatment.AcceptProposal(txCtx, userID, revisionID)
		if acceptErr != nil {
			return acceptErr
		}
		var projectionErr error
		plan, projectionErr = s.EnsurePlanForTreatment(txCtx, userID, consultationID, treatment)
		if projectionErr != nil {
			return fmt.Errorf("%w: %v", ErrTrainingProjectionFailed, projectionErr)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return treatment, plan, nil
}

// EnsurePlanForTreatment idempotently projects one accepted TreatmentRevision
// into the execution-oriented TrainingPlan used by daily task/check-in UI.
func (s *TrainingService) EnsurePlanForTreatment(
	ctx context.Context,
	userID uuid.UUID,
	consultationID *uuid.UUID,
	treatment *model.Treatment,
) (*model.TrainingPlan, error) {
	if treatment == nil || treatment.UserID != userID || treatment.Current == nil {
		return nil, errors.New("accepted current treatment is required")
	}
	revision := treatment.Current
	if revision.AcceptanceState != model.TreatmentAcceptanceAccepted {
		return nil, errors.New("only accepted treatment revisions can be executed")
	}
	if existing, err := s.trainingRepo.GetPlanByTreatmentRevision(ctx, userID, revision.ID); err != nil {
		return nil, err
	} else if existing != nil {
		// A previous request may have created the projection and failed before
		// superseding older plans. Re-run the idempotent cleanup on every retry.
		if err := s.trainingRepo.SupersedePlansExcept(ctx, userID, revision.ID); err != nil {
			return nil, err
		}
		return existing, nil
	}

	var content model.TreatmentPlanContent
	if err := json.Unmarshal(revision.Plan, &content); err != nil {
		return nil, fmt.Errorf("decode accepted treatment plan: %w", err)
	}
	exercises := make([]map[string]any, 0)
	for _, intervention := range revision.Interventions {
		if intervention.Kind != "exercise" && intervention.Kind != "mobility" && intervention.Kind != "self_test" {
			continue
		}
		var prescription map[string]any
		_ = json.Unmarshal(intervention.Prescription, &prescription)
		exercises = append(exercises, map[string]any{
			"intervention_id": intervention.ID,
			"name":            intervention.Title,
			"description":     intervention.Description,
			"sets":            stringValue(prescription["sets"]),
			"reps":            firstNonEmpty(stringValue(prescription["reps"]), stringValue(prescription["duration"])),
			"notes":           stringValue(prescription["notes"]),
		})
	}
	phase := map[string]any{
		"week":                         1,
		"focus":                        firstNonEmpty(content.Summary, content.Goal, revision.Goal),
		"exercises":                    exercises,
		"source_treatment_revision_id": revision.ID,
	}
	phases, _ := json.Marshal([]map[string]any{phase})
	plan := &model.TrainingPlan{
		ID: uuid.New(), UserID: userID, ConsultationID: consultationID,
		TreatmentID: &treatment.ID, TreatmentRevisionID: &revision.ID,
		Status: "active", Goal: revision.Goal, DurationWeeks: revision.DurationWeeks,
		CurrentWeek: 1, Phases: phases,
	}
	if err := s.trainingRepo.SupersedePlansExcept(ctx, userID, revision.ID); err != nil {
		return nil, err
	}
	if err := s.trainingRepo.CreatePlan(ctx, plan); err != nil {
		return nil, err
	}
	return plan, nil
}

func (s *TrainingService) GetPlanByTreatmentRevision(
	ctx context.Context,
	userID, treatmentRevisionID uuid.UUID,
) (*model.TrainingPlan, error) {
	return s.trainingRepo.GetPlanByTreatmentRevision(ctx, userID, treatmentRevisionID)
}

// EnsureCurrentPlan is the explicit recovery path for the accepted-treatment
// execution projection. It is safe to call after a response loss or partial
// projection failure because EnsurePlanForTreatment is idempotent.
func (s *TrainingService) EnsureCurrentPlan(
	ctx context.Context,
	userID uuid.UUID,
	consultationID *uuid.UUID,
) (*model.TrainingPlan, error) {
	if s.treatment == nil {
		return nil, errors.New("treatment service is not configured")
	}
	current, err := s.treatment.GetCurrent(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.EnsurePlanForTreatment(ctx, userID, consultationID, current)
}

func (s *TrainingService) GetPlan(ctx context.Context, id, userID uuid.UUID) (*model.TrainingPlan, error) {
	return s.trainingRepo.GetPlanByID(ctx, id, userID)
}

func (s *TrainingService) ListPlans(ctx context.Context, userID uuid.UUID) ([]model.TrainingPlan, error) {
	return s.trainingRepo.ListPlansByUserID(ctx, userID)
}

func (s *TrainingService) GetActivePlan(ctx context.Context, userID uuid.UUID) (*model.TrainingPlan, error) {
	return s.trainingRepo.GetActivePlanByUserID(ctx, userID)
}

func (s *TrainingService) GetTodayTask(ctx context.Context, planID, userID uuid.UUID) (*model.TrainingLog, error) {
	plan, err := s.trainingRepo.GetPlanByID(ctx, planID, userID)
	if err != nil || plan == nil {
		return nil, errors.New("plan not found")
	}
	if plan.Status != "active" {
		return nil, fmt.Errorf("training plan is %s", plan.Status)
	}
	today := localDate(time.Now())
	log, err := s.trainingRepo.GetOrCreateLog(ctx, plan, today)
	if err != nil {
		return nil, err
	}
	if string(log.Exercises) == "[]" || len(log.Exercises) == 0 {
		var phases []struct {
			Exercises []map[string]any `json:"exercises"`
		}
		_ = json.Unmarshal(plan.Phases, &phases)
		if len(phases) > 0 {
			tasks := make([]map[string]any, 0, len(phases[0].Exercises))
			for _, exercise := range phases[0].Exercises {
				tasks = append(tasks, map[string]any{
					"intervention_id": exercise["intervention_id"],
					"name":            exercise["name"],
					"completed":       false,
				})
			}
			log.Exercises = rawJSON(tasks, `[]`)
			if err := s.trainingRepo.SaveLog(ctx, log); err != nil {
				return nil, err
			}
		}
	}
	return log, nil
}

func (s *TrainingService) CheckIn(ctx context.Context, planID, userID uuid.UUID) error {
	plan, err := s.trainingRepo.GetPlanByID(ctx, planID, userID)
	if err != nil || plan == nil {
		return errors.New("plan not found")
	}
	if plan.Status != "active" {
		return fmt.Errorf("training plan is %s", plan.Status)
	}
	log, err := s.trainingRepo.CheckInAndGet(ctx, plan, localDate(time.Now()))
	if err != nil {
		return err
	}
	if s.treatment == nil || plan.TreatmentRevisionID == nil {
		return nil
	}
	_, _, err = s.treatment.RecordOutcome(ctx, userID, model.Outcome{
		TreatmentID: plan.TreatmentID, TreatmentRevisionID: plan.TreatmentRevisionID,
		SourceType: "training_checkin", SourceKey: "training-log:" + log.ID.String() + ":checkin",
		Kind: "training_adherence", ConcernKey: "general",
		Value:                datatypes.JSON(rawJSON(map[string]any{"checked_in": true, "date": log.Date}, `{}`)),
		AssociationStatement: "本次打卡记录了干预执行时间，仅表示执行与后续变化之间可用于观察的时间关系。",
		CausalityLevel:       "association_only", OccurredAt: time.Now().UTC(),
		Provenance: datatypes.JSON(rawJSON(map[string]any{"training_plan_id": plan.ID, "training_log_id": log.ID}, `{}`)),
	})
	if err == nil {
		_ = s.trainingRepo.MarkOutcomeRecorded(ctx, log.ID, time.Now().UTC())
	}
	return err
}

type TrainingFeedbackInput struct {
	Notes           string
	Exercises       any
	SymptomChanges  string
	TrainingFeeling string
	Difficulties    string
	BodyRegion      string
	ConcernKey      string
	Trend           string
	FactID          *uuid.UUID
}

func (s *TrainingService) UpdateLog(ctx context.Context, planID, userID uuid.UUID, notes string, exercises any) (map[string]any, error) {
	return s.UpdateLogWithFeedback(ctx, planID, userID, TrainingFeedbackInput{Notes: notes, Exercises: exercises})
}

func (s *TrainingService) UpdateLogWithFeedback(
	ctx context.Context,
	planID, userID uuid.UUID,
	feedback TrainingFeedbackInput,
) (map[string]any, error) {
	plan, err := s.trainingRepo.GetPlanByID(ctx, planID, userID)
	if err != nil || plan == nil {
		return nil, errors.New("plan not found")
	}
	log, err := s.trainingRepo.GetOrCreateLog(ctx, plan, localDate(time.Now()))
	if err != nil {
		return nil, err
	}
	log.Exercises = rawJSON(feedback.Exercises, `[]`)
	if strings.TrimSpace(feedback.Notes) != "" {
		notes := strings.TrimSpace(feedback.Notes)
		log.Notes = &notes
	}
	if err := s.trainingRepo.SaveLog(ctx, log); err != nil {
		return nil, err
	}
	if s.treatment == nil || plan.TreatmentRevisionID == nil {
		return map[string]any{"review_recommended": false}, nil
	}

	kind := "training_feedback"
	value := map[string]any{
		"training_feeling": feedback.TrainingFeeling,
		"difficulties":     feedback.Difficulties,
		"exercises":        feedback.Exercises,
	}
	if strings.TrimSpace(feedback.SymptomChanges) != "" {
		kind = "symptom_change"
		value["description"] = strings.TrimSpace(feedback.SymptomChanges)
		value["trend"] = feedback.Trend
		if feedback.FactID != nil {
			value["fact_id"] = feedback.FactID.String()
		}
	}
	payloadHash := bodyStateHash(string(rawJSON(map[string]any{"notes": feedback.Notes, "value": value}, `{}`)))
	outcome, _, err := s.treatment.RecordOutcome(ctx, userID, model.Outcome{
		TreatmentID: plan.TreatmentID, TreatmentRevisionID: plan.TreatmentRevisionID,
		SourceType: "training_feedback", SourceKey: "training-log:" + log.ID.String() + ":feedback:" + payloadHash,
		Kind: kind, ConcernKey: firstNonEmpty(feedback.ConcernKey, "general"), BodyRegion: feedback.BodyRegion,
		Value: datatypes.JSON(rawJSON(value, `{}`)), Notes: feedback.Notes,
		AssociationStatement: "该反馈发生在训练干预期间，可用于观察时间关联，但不单独证明训练导致了该变化。",
		CausalityLevel:       "association_only", OccurredAt: time.Now().UTC(),
		Provenance: datatypes.JSON(rawJSON(map[string]any{"training_plan_id": plan.ID, "training_log_id": log.ID}, `{}`)),
	})
	if err != nil {
		return nil, err
	}
	current, err := s.treatment.GetCurrent(ctx, userID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"outcome":            outcome,
		"treatment_status":   valueOrEmpty(current, func(t *model.Treatment) string { return t.Status }),
		"review_recommended": current != nil && current.Status == model.TreatmentStatusReviewRecommended,
		"paused":             current != nil && current.Status == model.TreatmentStatusPaused,
	}, nil
}

func (s *TrainingService) GetLogsByPlanID(ctx context.Context, planID uuid.UUID) ([]model.TrainingLog, error) {
	return s.trainingRepo.GetLogsByPlanID(ctx, planID)
}

func (s *TrainingService) GetProgress(ctx context.Context, planID, userID uuid.UUID) (map[string]any, error) {
	plan, err := s.trainingRepo.GetPlanByID(ctx, planID, userID)
	if err != nil || plan == nil {
		return nil, errors.New("plan not found")
	}
	logs, err := s.trainingRepo.GetLogsByPlanID(ctx, planID)
	if err != nil {
		return nil, err
	}
	consecutive, _ := s.trainingRepo.GetConsecutiveCheckInDays(ctx, planID)
	totalCheckIns := 0
	for _, log := range logs {
		if log.IsCheckedIn {
			totalCheckIns++
		}
	}
	return map[string]any{
		"consecutive_days": consecutive, "total_checkins": totalCheckIns,
		"current_week": plan.CurrentWeek, "total_weeks": plan.DurationWeeks,
		"treatment_revision_id": plan.TreatmentRevisionID, "plan_status": plan.Status,
	}, nil
}

func (s *TrainingService) Reassess(
	ctx context.Context,
	planID, userID uuid.UUID,
	feedback TrainingFeedbackInput,
) (map[string]any, error) {
	result, err := s.UpdateLogWithFeedback(ctx, planID, userID, feedback)
	if err != nil {
		return nil, err
	}
	if s.treatment == nil {
		return result, nil
	}
	current, err := s.treatment.GetCurrent(ctx, userID)
	if err != nil || current == nil || current.Current == nil {
		return result, err
	}
	if current.Status != model.TreatmentStatusReviewRecommended {
		return result, nil
	}
	proposal, err := s.treatment.GenerateProposal(ctx, userID, TreatmentProposalInput{
		DiagnosisAnalysisID: current.Current.SourceDiagnosisAnalysisID,
		UserConstraints: map[string]any{
			"training_feedback": map[string]any{
				"symptom_changes":  feedback.SymptomChanges,
				"training_feeling": feedback.TrainingFeeling,
				"difficulties":     feedback.Difficulties,
			},
		},
		ChangeReason: "training outcome review",
	})
	if err != nil {
		// The Outcome is already durable at this point. A stale Diagnosis or a
		// safety block means the next action is to re-analyze/review, not to turn
		// successful feedback persistence into a failed request.
		switch {
		case errors.Is(err, ErrTreatmentAnalysisStale):
			result["requires_new_diagnosis"] = true
			result["has_proposal"] = false
			return result, nil
		case errors.Is(err, ErrTreatmentSafetyBlocked):
			result["paused"] = true
			result["has_proposal"] = false
			return result, nil
		default:
			return nil, err
		}
	}
	result["proposal"] = proposal
	result["has_proposal"] = true
	return result, nil
}

func localDate(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, value.Location())
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func valueOrEmpty[T any](value *T, getter func(*T) string) string {
	if value == nil {
		return ""
	}
	return getter(value)
}
