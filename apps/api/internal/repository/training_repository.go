package repository

import (
	"context"
	"errors"

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

// TrainingRepository holds the training plan owner lock and the plan helpers
// still used by the accepted-plan projection.
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

// GetActivePlan is the HealthWorkspace's single-active-plan read; it fails
// closed with ErrMultipleActiveTrainingPlans like GetActivePlanByUserID.
func (r *TrainingRepository) GetActivePlan(ctx context.Context, userID uuid.UUID) (*model.TrainingPlan, error) {
	return r.GetActivePlanByUserID(ctx, userID)
}
