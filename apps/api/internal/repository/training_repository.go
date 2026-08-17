package repository

import (
	"context"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TrainingRepository handles database operations for training plans and logs.
type TrainingRepository struct {
	db *gorm.DB
}

func NewTrainingRepository(db *gorm.DB) *TrainingRepository {
	return &TrainingRepository{db: db}
}

// Plan operations

func (r *TrainingRepository) CreatePlan(ctx context.Context, plan *model.TrainingPlan) error {
	return database.FromContext(ctx, r.db).Create(plan).Error
}

func (r *TrainingRepository) GetPlanByID(ctx context.Context, id, userID uuid.UUID) (*model.TrainingPlan, error) {
	var plan model.TrainingPlan
	err := database.FromContext(ctx, r.db).
		Where("id = ? AND user_id = ?", id, userID).
		First(&plan).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &plan, err
}

func (r *TrainingRepository) ListPlansByUserID(ctx context.Context, userID uuid.UUID) ([]model.TrainingPlan, error) {
	var plans []model.TrainingPlan
	err := database.FromContext(ctx, r.db).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&plans).Error
	return plans, err
}

func (r *TrainingRepository) GetActivePlanByUserID(ctx context.Context, userID uuid.UUID) (*model.TrainingPlan, error) {
	var plan model.TrainingPlan
	err := database.FromContext(ctx, r.db).
		Where("user_id = ? AND status = ?", userID, "active").
		Order("created_at DESC").
		First(&plan).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &plan, err
}

// Log operations

func (r *TrainingRepository) CreateOrUpdateLog(ctx context.Context, log *model.TrainingLog) error {
	return database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		// Try to find existing log for this plan+date with row-level lock
		var existing model.TrainingLog
		err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("plan_id = ? AND date = ?", log.PlanID, log.Date).
			First(&existing).Error

		if err == gorm.ErrRecordNotFound {
			return tx.Create(log).Error
		}
		if err != nil {
			return err
		}

		// Update existing log
		existing.Exercises = log.Exercises
		existing.Notes = log.Notes
		existing.IsCheckedIn = log.IsCheckedIn
		return tx.Save(&existing).Error
	})
}

func (r *TrainingRepository) GetLogByDate(ctx context.Context, planID uuid.UUID, date time.Time) (*model.TrainingLog, error) {
	var log model.TrainingLog
	err := database.FromContext(ctx, r.db).
		Where("plan_id = ? AND date = ?", planID, date).
		First(&log).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &log, err
}

func (r *TrainingRepository) GetLogsByPlanID(ctx context.Context, planID uuid.UUID) ([]model.TrainingLog, error) {
	var logs []model.TrainingLog
	err := database.FromContext(ctx, r.db).
		Where("plan_id = ?", planID).
		Order("date DESC").
		Find(&logs).Error
	return logs, err
}

func (r *TrainingRepository) CheckIn(ctx context.Context, planID, userID uuid.UUID, date time.Time) error {
	var log model.TrainingLog
	err := database.FromContext(ctx, r.db).
		Where("plan_id = ? AND date = ?", planID, date).
		First(&log).Error

	if err == gorm.ErrRecordNotFound {
		// Create new log with check-in
		log = model.TrainingLog{
			ID:          uuid.New(),
			UserID:      userID,
			PlanID:      planID,
			Date:        date,
			Exercises:   []byte("[]"),
			IsCheckedIn: true,
		}
		return database.FromContext(ctx, r.db).Create(&log).Error
	}
	if err != nil {
		return err
	}

	log.IsCheckedIn = true
	return database.FromContext(ctx, r.db).Save(&log).Error
}

// GetConsecutiveCheckInDays returns the number of consecutive check-in days ending today.
func (r *TrainingRepository) GetConsecutiveCheckInDays(ctx context.Context, planID uuid.UUID) (int, error) {
	var logs []model.TrainingLog
	err := database.FromContext(ctx, r.db).
		Where("plan_id = ? AND is_checked_in = true", planID).
		Order("date DESC").
		Find(&logs).Error
	if err != nil {
		return 0, err
	}

	if len(logs) == 0 {
		return 0, nil
	}

	count := 0
	expected := time.Now().Truncate(24 * time.Hour)
	for i, log := range logs {
		logDate := log.Date.Truncate(24 * time.Hour)
		if i == 0 && logDate.Equal(expected.Add(-24*time.Hour)) {
			// Allow first entry to be yesterday (timezone edge case)
			count++
			expected = logDate.Add(-24 * time.Hour)
		} else if logDate.Equal(expected) {
			count++
			expected = logDate.Add(-24 * time.Hour)
		} else {
			break
		}
	}
	return count, nil
}

// GetPlanByTreatmentRevision returns the execution projection for one accepted
// TreatmentRevision. The treatment revision remains the source of truth.
func (r *TrainingRepository) GetPlanByTreatmentRevision(
	ctx context.Context,
	userID, treatmentRevisionID uuid.UUID,
) (*model.TrainingPlan, error) {
	var plan model.TrainingPlan
	err := database.FromContext(ctx, r.db).
		Where("user_id = ? AND treatment_revision_id = ?", userID, treatmentRevisionID).
		First(&plan).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &plan, err
}

func (r *TrainingRepository) SupersedePlansExcept(
	ctx context.Context,
	userID, treatmentRevisionID uuid.UUID,
) error {
	return database.FromContext(ctx, r.db).Model(&model.TrainingPlan{}).
		Where("user_id = ? AND (treatment_revision_id IS NULL OR treatment_revision_id <> ?) AND status = ?", userID, treatmentRevisionID, "active").
		Update("status", "superseded").Error
}

func (r *TrainingRepository) CheckInAndGet(
	ctx context.Context,
	plan *model.TrainingPlan,
	date time.Time,
) (*model.TrainingLog, error) {
	var stored model.TrainingLog
	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		err := tx.WithContext(ctx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("plan_id = ? AND date = ?", plan.ID, date).
			First(&stored).Error
		if err == gorm.ErrRecordNotFound {
			stored = model.TrainingLog{
				ID: uuid.New(), UserID: plan.UserID, PlanID: plan.ID, Date: date,
				Exercises: []byte("[]"), IsCheckedIn: true,
				TreatmentRevisionID: plan.TreatmentRevisionID,
			}
			return tx.WithContext(ctx).Create(&stored).Error
		}
		if err != nil {
			return err
		}
		stored.IsCheckedIn = true
		if stored.TreatmentRevisionID == nil {
			stored.TreatmentRevisionID = plan.TreatmentRevisionID
		}
		return tx.WithContext(ctx).Save(&stored).Error
	})
	if err != nil {
		return nil, err
	}
	return &stored, nil
}

func (r *TrainingRepository) GetOrCreateLog(
	ctx context.Context,
	plan *model.TrainingPlan,
	date time.Time,
) (*model.TrainingLog, error) {
	var log model.TrainingLog
	err := database.FromContext(ctx, r.db).
		Where("plan_id = ? AND date = ?", plan.ID, date).
		First(&log).Error
	if err == nil {
		return &log, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	log = model.TrainingLog{
		ID: uuid.New(), UserID: plan.UserID, PlanID: plan.ID, Date: date,
		Exercises: []byte("[]"), TreatmentRevisionID: plan.TreatmentRevisionID,
	}
	if err := database.FromContext(ctx, r.db).Create(&log).Error; err != nil {
		return nil, err
	}
	return &log, nil
}

func (r *TrainingRepository) SaveLog(
	ctx context.Context,
	log *model.TrainingLog,
) error {
	return database.FromContext(ctx, r.db).Save(log).Error
}

func (r *TrainingRepository) MarkOutcomeRecorded(
	ctx context.Context,
	logID uuid.UUID,
	at time.Time,
) error {
	return database.FromContext(ctx, r.db).Model(&model.TrainingLog{}).
		Where("id = ?", logID).
		Update("outcome_recorded_at", at).Error
}
