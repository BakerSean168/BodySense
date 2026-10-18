package repository

import (
	"context"
	"errors"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrMultipleActiveTrainingPlans fails closed for legacy consumers expecting
// a single active plan. Never select a random plan when a user may have two.
var ErrMultipleActiveTrainingPlans = errors.New("multiple active training plans require explicit plan selection")

// ErrTrainingPlanOwnerLockRequiresTransaction prevents accidentally returning
// a released row lock to an independent-plan application transaction.
var ErrTrainingPlanOwnerLockRequiresTransaction = errors.New("training plan owner lock requires an active transaction")

// ErrTrainingPlanOwnerNotFound deliberately contains no user or plan identity.
var ErrTrainingPlanOwnerNotFound = errors.New("training plan owner not found")

// TrainingRepository handles database operations for training plans and logs.
type TrainingRepository struct {
	db *gorm.DB
}

func NewTrainingRepository(db *gorm.DB) *TrainingRepository {
	return &TrainingRepository{db: db}
}

// Plan operations

// LockOwnerForPlanActivation serializes accepted-plan projection operations
// for the same owner. It must run inside the outer acceptance transaction,
// before reading/accepting the proposal or testing projection idempotency.
// Locking the stable users row avoids a missing-plan race and hash collisions;
// no lock of another user's row is needed. FOR NO KEY UPDATE still conflicts
// with itself, so same-owner activations serialize, but not with the
// FOR KEY SHARE that PostgreSQL foreign-key checks take on users, so the
// owner's concurrent Chat/BodyFact/Observation inserts are not blocked.
// IndependentTrainingPlanService uses this before every proposal decision.
func (r *TrainingRepository) LockOwnerForPlanActivation(ctx context.Context, userID uuid.UUID) error {
	if userID == uuid.Nil {
		return ErrTrainingPlanOwnerNotFound
	}
	if !database.HasTransaction(ctx) {
		return ErrTrainingPlanOwnerLockRequiresTransaction
	}
	var row struct{ ID uuid.UUID }
	err := database.FromContext(ctx, r.db).Table("users").
		Select("id").
		Where("id = ?", userID).
		Clauses(clause.Locking{Strength: "NO KEY UPDATE"}).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrTrainingPlanOwnerNotFound
	}
	return err
}

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
	if userID == uuid.Nil {
		return nil, errors.New("valid training plan owner required")
	}
	// Keep the old one-plan Workspace API from silently selecting a wrong
	// prescription after V2 introduces multiple active plans. Limit(2)
	// bounds this compatibility lookup; the new V2 UI must instead list
	// plans and select explicitly by plan ID.
	var plans []model.TrainingPlan
	err := database.FromContext(ctx, r.db).
		Where("user_id = ? AND status = ?", userID, "active").
		Order("created_at DESC").
		Order("id DESC").
		Limit(2).
		Find(&plans).Error
	if err != nil {
		return nil, err
	}
	if len(plans) > 1 {
		return nil, ErrMultipleActiveTrainingPlans
	}
	if len(plans) == 0 {
		return nil, nil
	}
	return &plans[0], nil
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
			}
			return tx.WithContext(ctx).Create(&stored).Error
		}
		if err != nil {
			return err
		}
		stored.IsCheckedIn = true
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
		Exercises: []byte("[]"),
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
