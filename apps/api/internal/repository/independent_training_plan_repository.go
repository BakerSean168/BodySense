package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrIndependentPlanConflict = errors.New("training plan version or decision conflict")
var ErrIndependentPlanNotFound = errors.New("training plan or proposal not found")

// IndependentTrainingPlanRepository participates in the application's transaction.
// No method selects a plan by 'current active' or supersedes another plan.
type IndependentTrainingPlanRepository struct{ db *gorm.DB }

func NewIndependentTrainingPlanRepository(db *gorm.DB) *IndependentTrainingPlanRepository {
	return &IndependentTrainingPlanRepository{db: db}
}
func (r *IndependentTrainingPlanRepository) dbFor(ctx context.Context) *gorm.DB {
	return database.FromContext(ctx, r.db)
}

// LockContext locks the same BodyState head used by native fact/screen/lift
// commands. The owner lock must be held first. Missing BodyState is revision 0,
// never a clean safety screen. Diagnosis rows are read only as context here.
func (r *IndependentTrainingPlanRepository) LockContext(ctx context.Context, owner uuid.UUID) (int64, json.RawMessage, error) {
	if !database.HasTransaction(ctx) {
		return 0, nil, ErrTrainingPlanOwnerLockRequiresTransaction
	}
	db := r.dbFor(ctx)
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.BodyState{UserID: owner}).Error; err != nil {
		return 0, nil, err
	}
	var head model.BodyState
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id=?", owner).Take(&head).Error; err != nil {
		return 0, nil, err
	}
	// Conservative relevance in slice 1: any native BodyState revision or change
	// to the owner's Diagnosis context invalidates a pending proposal. No TTL or
	// clinical inference is introduced. Bound the read instead of omitting rows.
	var rows []struct{ Snapshot json.RawMessage }
	err := db.Raw("SELECT to_jsonb(d) AS snapshot FROM diagnosis_analyses d WHERE user_id=? ORDER BY id LIMIT 201", owner).Scan(&rows).Error
	if err != nil {
		return 0, nil, err
	}
	if len(rows) > 200 {
		return 0, nil, fmt.Errorf("diagnosis context exceeds bounded read")
	}
	encoded, err := json.Marshal(rows)
	return head.CurrentRevision, encoded, err
}
func (r *IndependentTrainingPlanRepository) ProposalByKey(ctx context.Context, owner uuid.UUID, key string) (*model.PlanProposal, error) {
	var row model.PlanProposal
	err := r.dbFor(ctx).Where("user_id=? AND idempotency_key=?", owner, key).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &row, err
}
func (r *IndependentTrainingPlanRepository) Proposal(ctx context.Context, owner, id uuid.UUID) (*model.PlanProposal, error) {
	var p model.PlanProposal
	err := r.dbFor(ctx).Where("user_id=? AND id=?", owner, id).Take(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrIndependentPlanNotFound
	}
	return &p, err
}
func (r *IndependentTrainingPlanRepository) Plan(ctx context.Context, owner, id uuid.UUID) (*model.TrainingPlan, error) {
	var p model.TrainingPlan
	err := r.dbFor(ctx).Where("user_id=? AND id=? AND current_accepted_revision_id IS NOT NULL", owner, id).Take(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrIndependentPlanNotFound
	}
	return &p, err
}
func (r *IndependentTrainingPlanRepository) List(ctx context.Context, owner uuid.UUID) ([]model.TrainingPlan, error) {
	p := []model.TrainingPlan{}
	err := r.dbFor(ctx).Where("user_id=? AND current_accepted_revision_id IS NOT NULL", owner).Order("created_at,id").Find(&p).Error
	return p, err
}
func (r *IndependentTrainingPlanRepository) Revision(ctx context.Context, owner, plan, id uuid.UUID) (*model.TrainingPlanRevision, error) {
	var row model.TrainingPlanRevision
	err := r.dbFor(ctx).Where("user_id=? AND plan_id=? AND id=?", owner, plan, id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrIndependentPlanNotFound
	}
	return &row, err
}
func (r *IndependentTrainingPlanRepository) Receipt(ctx context.Context, owner uuid.UUID, key string) (*model.PlanDecisionReceipt, error) {
	var row model.PlanDecisionReceipt
	err := r.dbFor(ctx).Where("user_id=? AND idempotency_key=?", owner, key).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &row, err
}
func (r *IndependentTrainingPlanRepository) InsertProposal(ctx context.Context, p *model.PlanProposal) error {
	return r.dbFor(ctx).Create(p).Error
}
func (r *IndependentTrainingPlanRepository) FinishProposal(ctx context.Context, p *model.PlanProposal) error {
	result := r.dbFor(ctx).Model(&model.PlanProposal{}).Where("user_id=? AND id=? AND state='pending'", p.UserID, p.ID).Updates(map[string]any{"state": p.State, "decision_id": p.DecisionID, "expired_reason": p.ExpiredReason})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrIndependentPlanConflict
	}
	return nil
}
func (r *IndependentTrainingPlanRepository) CommitRevision(ctx context.Context, plan *model.TrainingPlan, revision *model.TrainingPlanRevision, draft model.TrainingPlanDraft, create bool) error {
	db := r.dbFor(ctx)
	if create {
		if err := db.Create(plan).Error; err != nil {
			return err
		}
	} else {
		result := db.Model(&model.TrainingPlan{}).Where("user_id=? AND id=? AND current_accepted_revision_id=?", plan.UserID, plan.ID, revision.ParentRevisionID).Updates(map[string]any{"title": draft.Title, "goal": draft.Goal, "current_accepted_revision_id": revision.ID})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrIndependentPlanConflict
		}
	}
	identities := []struct{ Kind, ID string }{}
	for _, session := range draft.Content.Sessions {
		identities = append(identities, struct{ Kind, ID string }{"template", session.TemplateID})
		for _, stage := range session.Stages {
			for _, e := range stage.Exercises {
				identities = append(identities, struct{ Kind, ID string }{"exercise", e.PlanExerciseID})
			}
		}
	}
	for _, identity := range identities {
		if err := db.Exec("INSERT INTO training_plan_entry_identities(user_id,kind,id,plan_id) VALUES (?,?,?,?) ON CONFLICT DO NOTHING", plan.UserID, identity.Kind, identity.ID, plan.ID).Error; err != nil {
			return err
		}
		var count int64
		if err := db.Table("training_plan_entry_identities").Where("user_id=? AND kind=? AND id=? AND plan_id=?", plan.UserID, identity.Kind, identity.ID, plan.ID).Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return ErrIndependentPlanConflict
		}
	}
	if err := db.Create(revision).Error; err != nil {
		return err
	}
	for _, ref := range draft.RelatedBodyRefs {
		link := map[string]any{"user_id": plan.UserID, "revision_id": revision.ID}
		switch ref.Kind {
		case "body_fact":
			link["body_fact_id"] = ref.ID
		case "body_observation":
			link["body_observation_id"] = ref.ID
		case "symptom_episode":
			link["symptom_episode_id"] = ref.ID
		default:
			return ErrIndependentPlanNotFound
		}
		if err := db.Table("training_plan_revision_body_refs").Create(link).Error; err != nil {
			return err
		}
	}
	return nil
}
func (r *IndependentTrainingPlanRepository) InsertReceipt(ctx context.Context, receipt *model.PlanDecisionReceipt) error {
	err := r.dbFor(ctx).Create(receipt).Error
	var conflict *pgconn.PgError
	if errors.As(err, &conflict) && conflict.Code == "23505" {
		return ErrIndependentPlanConflict
	}
	return err
}

func (r *IndependentTrainingPlanRepository) ValidateBodyRefs(ctx context.Context, owner uuid.UUID, refs []model.PlanBodyRef) error {
	for _, ref := range refs {
		table := ""
		switch ref.Kind {
		case "body_fact":
			table = "body_facts"
		case "body_observation":
			table = "body_observations"
		case "symptom_episode":
			table = "symptom_episodes"
		default:
			return ErrIndependentPlanNotFound
		}
		var count int64
		err := r.dbFor(ctx).Table(table).Where("user_id=? AND id=? AND created_revision=?", owner, ref.ID, ref.SourceVersion).Count(&count).Error
		if err != nil {
			return err
		}
		if count != 1 {
			return ErrIndependentPlanNotFound
		}
	}
	return nil
}
