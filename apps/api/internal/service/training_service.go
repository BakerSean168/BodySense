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
	GetPlanByID(ctx context.Context, id, userID uuid.UUID) (*model.TrainingPlan, error)
	ListPlansByUserID(ctx context.Context, userID uuid.UUID) ([]model.TrainingPlan, error)
	GetActivePlanByUserID(ctx context.Context, userID uuid.UUID) (*model.TrainingPlan, error)
	GetOrCreateLog(ctx context.Context, plan *model.TrainingPlan, date time.Time) (*model.TrainingLog, error)
	SaveLog(ctx context.Context, log *model.TrainingLog) error
	CheckInAndGet(ctx context.Context, plan *model.TrainingPlan, date time.Time) (*model.TrainingLog, error)
	MarkOutcomeRecorded(ctx context.Context, logID uuid.UUID, at time.Time) error
	GetLogsByPlanID(ctx context.Context, planID uuid.UUID) ([]model.TrainingLog, error)
	GetConsecutiveCheckInDays(ctx context.Context, planID uuid.UUID) (int, error)
}

type trainingOutcomeRecorder interface {
	RecordOutcome(ctx context.Context, userID uuid.UUID, outcome model.Outcome) (*model.Outcome, bool, error)
}

// TrainingService serves the legacy daily-task execution surface (/training)
// over TrainingPlan rows. It never creates, changes or supersedes a plan:
// plans are written only by an accepted PlanProposal. Check-ins and feedback
// are recorded as Outcomes. TP-003 replaces this surface with Trainer
// sessions.
type TrainingService struct {
	trainingRepo trainingRepository
	outcomes     trainingOutcomeRecorder
}

func NewTrainingService(trainingRepo trainingRepository, outcomes trainingOutcomeRecorder) *TrainingService {
	return &TrainingService{trainingRepo: trainingRepo, outcomes: outcomes}
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
			tasks := make([]TrainingExerciseLogInput, 0, len(phases[0].Exercises))
			for _, exercise := range phases[0].Exercises {
				tasks = append(tasks, TrainingExerciseLogInput{
					Name:      stringValue(exercise["name"]),
					Completed: false,
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
	if s.outcomes == nil {
		return nil
	}
	_, _, err = s.outcomes.RecordOutcome(ctx, userID, model.Outcome{
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

type TrainingExerciseLogInput struct {
	Name      string `json:"name"`
	Completed bool   `json:"completed"`
}

type TrainingFeedbackInput struct {
	Notes           string
	Exercises       *[]TrainingExerciseLogInput
	SymptomChanges  string
	TrainingFeeling string
	Difficulties    string
	BodyRegion      string
	ConcernKey      string
	Trend           string
}

func (s *TrainingService) UpdateLog(ctx context.Context, planID, userID uuid.UUID, notes string, exercises []TrainingExerciseLogInput) (map[string]any, error) {
	return s.UpdateLogWithFeedback(ctx, planID, userID, TrainingFeedbackInput{Notes: notes, Exercises: &exercises})
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
	if feedback.Exercises != nil {
		log.Exercises = rawJSON(*feedback.Exercises, `[]`)
	}
	if strings.TrimSpace(feedback.Notes) != "" {
		notes := strings.TrimSpace(feedback.Notes)
		log.Notes = &notes
	}
	if err := s.trainingRepo.SaveLog(ctx, log); err != nil {
		return nil, err
	}
	if s.outcomes == nil {
		return map[string]any{}, nil
	}

	kind := "training_feedback"
	value := map[string]any{
		"training_feeling": feedback.TrainingFeeling,
		"difficulties":     feedback.Difficulties,
	}
	if feedback.Exercises != nil {
		value["exercises"] = *feedback.Exercises
	}
	if strings.TrimSpace(feedback.SymptomChanges) != "" {
		kind = "symptom_change"
		value["description"] = strings.TrimSpace(feedback.SymptomChanges)
		value["trend"] = feedback.Trend
	}
	payloadHash := bodyStateHash(string(rawJSON(map[string]any{"notes": feedback.Notes, "value": value}, `{}`)))
	outcome, _, err := s.outcomes.RecordOutcome(ctx, userID, model.Outcome{
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
	// Feedback is an Outcome only; it never changes or proposes a plan here.
	return map[string]any{"outcome": outcome}, nil
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
		"plan_status": plan.Status,
	}, nil
}

// Reassess records reassessment feedback as an Outcome. Review-driven plan
// changes arrive as PlanProposal(update) only (not in this slice).
func (s *TrainingService) Reassess(
	ctx context.Context,
	planID, userID uuid.UUID,
	feedback TrainingFeedbackInput,
) (map[string]any, error) {
	return s.UpdateLogWithFeedback(ctx, planID, userID, feedback)
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
