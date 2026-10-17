package service

import (
	"math"
	"strings"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

func positivePlanNumber(n float64) bool { return n > 0 && !math.IsNaN(n) && !math.IsInf(n, 0) }
func validatePlanDraft(d model.TrainingPlanDraft) error {
	if strings.TrimSpace(d.Title) == "" || strings.TrimSpace(d.Goal) == "" || len(d.Title) > 500 || len(d.Content.Sessions) == 0 || len(d.Content.Sessions) > 100 {
		return ErrPlanInvalid
	}
	if len(d.RelatedConditionIDs) > 200 {
		return ErrPlanInvalid
	}
	seenConditions := map[uuid.UUID]bool{}
	for _, id := range d.RelatedConditionIDs {
		if id == uuid.Nil || seenConditions[id] {
			return ErrPlanInvalid
		}
		seenConditions[id] = true
	}
	if n := d.Content.Frequency.SessionsPerWeek; n != nil && *n <= 0 {
		return ErrPlanInvalid
	}
	templates := map[string]bool{}
	entries := map[string]bool{}
	count := 0
	for _, session := range d.Content.Sessions {
		if strings.TrimSpace(session.TemplateID) == "" || templates[session.TemplateID] || strings.TrimSpace(session.Title) == "" || len(session.Stages) == 0 {
			return ErrPlanInvalid
		}
		templates[session.TemplateID] = true
		sessionExercises := 0
		for _, stage := range session.Stages {
			if stage.Phase != "warmup" && stage.Phase != "main" && stage.Phase != "cooldown" {
				return ErrPlanInvalid
			}
			for _, e := range stage.Exercises {
				count++
				sessionExercises++
				if count > 1000 || strings.TrimSpace(e.PlanExerciseID) == "" || entries[e.PlanExerciseID] || strings.TrimSpace(e.ExerciseID) == "" || strings.TrimSpace(e.NameSnapshot) == "" || e.Sets <= 0 {
					return ErrPlanInvalid
				}
				entries[e.PlanExerciseID] = true
				if e.RestSeconds != nil && (*e.RestSeconds < 0 || math.IsNaN(*e.RestSeconds) || math.IsInf(*e.RestSeconds, 0)) {
					return ErrPlanInvalid
				}
				if e.Side != "" && e.Side != "left" && e.Side != "right" && e.Side != "both" && e.Side != "alternating" {
					return ErrPlanInvalid
				}
				switch e.Target.Kind {
				case "repetitions":
					if !positivePlanNumber(e.Target.Min) || e.Target.Seconds != 0 || e.Target.Meters != 0 || (e.Target.Max != nil && (!positivePlanNumber(*e.Target.Max) || *e.Target.Max < e.Target.Min)) {
						return ErrPlanInvalid
					}
				case "duration":
					if !positivePlanNumber(e.Target.Seconds) || e.Target.Min != 0 || e.Target.Max != nil || e.Target.Meters != 0 {
						return ErrPlanInvalid
					}
				case "distance":
					if !positivePlanNumber(e.Target.Meters) || e.Target.Min != 0 || e.Target.Max != nil || e.Target.Seconds != 0 {
						return ErrPlanInvalid
					}
				default:
					return ErrPlanInvalid
				}
			}
		}
		if sessionExercises == 0 {
			return ErrPlanInvalid
		}
	}
	schedule := d.Content.Schedule
	switch schedule.Mode {
	case "frequency_only":
		if schedule.TimeZone != "" || len(schedule.Occurrences) > 0 {
			return ErrPlanInvalid
		}
	case "calendar":
		if schedule.TimeZone == "" || schedule.TimeZone == "Local" {
			return ErrPlanInvalid
		}
		if _, err := time.LoadLocation(schedule.TimeZone); err != nil {
			return ErrPlanInvalid
		}
		seen := map[string]bool{}
		for _, occurrence := range schedule.Occurrences {
			if occurrence.OccurrenceID == "" || seen[occurrence.OccurrenceID] || !templates[occurrence.TemplateID] {
				return ErrPlanInvalid
			}
			seen[occurrence.OccurrenceID] = true
			if _, err := time.Parse("2006-01-02", occurrence.LocalDate); err != nil {
				return ErrPlanInvalid
			}
		}
	default:
		return ErrPlanInvalid
	}
	for _, ref := range d.RelatedBodyRefs {
		if ref.SourceVersion <= 0 {
			return ErrPlanInvalid
		}
	}
	return nil
}
