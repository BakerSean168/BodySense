package repository

import (
	"context"
	"time"

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
	return r.db.WithContext(ctx).Create(plan).Error
}

func (r *TrainingRepository) GetPlanByID(ctx context.Context, id, userID uuid.UUID) (*model.TrainingPlan, error) {
	var plan model.TrainingPlan
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		First(&plan).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &plan, err
}

func (r *TrainingRepository) ListPlansByUserID(ctx context.Context, userID uuid.UUID) ([]model.TrainingPlan, error) {
	var plans []model.TrainingPlan
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&plans).Error
	return plans, err
}

// Log operations

func (r *TrainingRepository) CreateOrUpdateLog(ctx context.Context, log *model.TrainingLog) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
	err := r.db.WithContext(ctx).
		Where("plan_id = ? AND date = ?", planID, date).
		First(&log).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &log, err
}

func (r *TrainingRepository) GetLogsByPlanID(ctx context.Context, planID uuid.UUID) ([]model.TrainingLog, error) {
	var logs []model.TrainingLog
	err := r.db.WithContext(ctx).
		Where("plan_id = ?", planID).
		Order("date DESC").
		Find(&logs).Error
	return logs, err
}

func (r *TrainingRepository) CheckIn(ctx context.Context, planID, userID uuid.UUID, date time.Time) error {
	var log model.TrainingLog
	err := r.db.WithContext(ctx).
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
		return r.db.WithContext(ctx).Create(&log).Error
	}
	if err != nil {
		return err
	}

	log.IsCheckedIn = true
	return r.db.WithContext(ctx).Save(&log).Error
}

// GetConsecutiveCheckInDays returns the number of consecutive check-in days ending today.
func (r *TrainingRepository) GetConsecutiveCheckInDays(ctx context.Context, planID uuid.UUID) (int, error) {
	var logs []model.TrainingLog
	err := r.db.WithContext(ctx).
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
