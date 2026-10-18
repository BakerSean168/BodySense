package repository

import (
	"context"
	"errors"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrTrainerSessionNotFound = errors.New("training session not found")

// ErrTrainerSessionRace is a lost compare-and-set or a unique-key race. The
// application maps it to a conflict; nothing was written.
var ErrTrainerSessionRace = errors.New("training session changed concurrently")

// ErrTrainerSessionInProgress reports the per-user single in-progress index.
var ErrTrainerSessionInProgress = errors.New("another training session is in progress")

// TrainerSessionRepository participates in the application's transaction and
// always filters by owner, so a foreign id reads exactly like a missing one.
type TrainerSessionRepository struct{ db *gorm.DB }

func NewTrainerSessionRepository(db *gorm.DB) *TrainerSessionRepository {
	return &TrainerSessionRepository{db: db}
}

func (r *TrainerSessionRepository) dbFor(ctx context.Context) *gorm.DB {
	return database.FromContext(ctx, r.db)
}

// SafetyRevision share-locks the owner's BodyState head so no native Safety
// command commits between the TrainerGate read and the receipt insert. A
// missing head is revision 0, which the gate never allows.
func (r *TrainerSessionRepository) SafetyRevision(ctx context.Context, owner uuid.UUID) (int64, error) {
	var head model.BodyState
	err := r.dbFor(ctx).Clauses(clause.Locking{Strength: "SHARE"}).Where("user_id=?", owner).Take(&head).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	return head.CurrentRevision, err
}

func (r *TrainerSessionRepository) Session(ctx context.Context, owner, id uuid.UUID, forUpdate bool) (*model.TrainingSession, error) {
	var row model.TrainingSession
	q := r.dbFor(ctx)
	if forUpdate {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := q.Where("user_id=? AND id=?", owner, id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTrainerSessionNotFound
	}
	return &row, err
}

func (r *TrainerSessionRepository) SessionByStartKey(ctx context.Context, owner uuid.UUID, key string) (*model.TrainingSession, error) {
	return r.optionalSession(ctx, "user_id=? AND start_idempotency_key=?", owner, key)
}

func (r *TrainerSessionRepository) SessionByFinishKey(ctx context.Context, owner uuid.UUID, key string) (*model.TrainingSession, error) {
	return r.optionalSession(ctx, "user_id=? AND finish_idempotency_key=?", owner, key)
}

func (r *TrainerSessionRepository) optionalSession(ctx context.Context, where string, args ...any) (*model.TrainingSession, error) {
	var row model.TrainingSession
	err := r.dbFor(ctx).Where(where, args...).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &row, err
}

// List returns the owner's newest sessions, optionally only one state.
func (r *TrainerSessionRepository) List(ctx context.Context, owner uuid.UUID, state string, limit int) ([]model.TrainingSession, error) {
	rows := []model.TrainingSession{}
	q := r.dbFor(ctx).Where("user_id=?", owner)
	if state != "" {
		q = q.Where("state=?", state)
	}
	err := q.Order("started_at DESC, id").Limit(limit).Find(&rows).Error
	return rows, err
}

func (r *TrainerSessionRepository) Actions(ctx context.Context, owner uuid.UUID, sessions []uuid.UUID) ([]model.TrainingSetAction, error) {
	rows := []model.TrainingSetAction{}
	if len(sessions) == 0 {
		return rows, nil
	}
	err := r.dbFor(ctx).Where("user_id=? AND session_id IN ?", owner, sessions).Order("session_id, session_revision").Find(&rows).Error
	return rows, err
}

func (r *TrainerSessionRepository) ActionByKey(ctx context.Context, owner uuid.UUID, key string) (*model.TrainingSetAction, error) {
	var row model.TrainingSetAction
	err := r.dbFor(ctx).Where("user_id=? AND idempotency_key=?", owner, key).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &row, err
}

// SetRecorded reports whether the session already holds an action for set.
func (r *TrainerSessionRepository) SetRecorded(ctx context.Context, owner, session uuid.UUID, planExerciseID string, set int) (bool, error) {
	var n int64
	err := r.dbFor(ctx).Model(&model.TrainingSetAction{}).
		Where("user_id=? AND session_id=? AND plan_exercise_id=? AND set_number=?", owner, session, planExerciseID, set).
		Count(&n).Error
	return n > 0, err
}

func (r *TrainerSessionRepository) InsertSession(ctx context.Context, row *model.TrainingSession) error {
	err := r.dbFor(ctx).Create(row).Error
	if uniqueViolation(err, "training_sessions_one_in_progress") {
		return ErrTrainerSessionInProgress
	}
	if uniqueViolation(err, "") {
		return ErrTrainerSessionRace
	}
	return err
}

// AppendAction advances the in-progress session by exactly one revision and
// inserts the receipt at that revision, or writes nothing.
func (r *TrainerSessionRepository) AppendAction(ctx context.Context, action *model.TrainingSetAction) error {
	db := r.dbFor(ctx)
	result := db.Model(&model.TrainingSession{}).
		Where("user_id=? AND id=? AND state='in_progress' AND revision=?", action.UserID, action.SessionID, action.SessionRevision-1).
		Update("revision", action.SessionRevision)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrTrainerSessionRace
	}
	if err := db.Create(action).Error; err != nil {
		if uniqueViolation(err, "") {
			return ErrTrainerSessionRace
		}
		return err
	}
	return nil
}

// End applies the single in_progress -> ended transition at expected+1.
func (r *TrainerSessionRepository) End(ctx context.Context, row *model.TrainingSession, expected int64) error {
	result := r.dbFor(ctx).Model(&model.TrainingSession{}).
		Where("user_id=? AND id=? AND state='in_progress' AND revision=?", row.UserID, row.ID, expected).
		Updates(map[string]any{
			"state": row.State, "ended_reason": row.EndedReason, "ended_at": row.EndedAt, "revision": row.Revision,
			"finish_idempotency_key": row.FinishIdempotencyKey, "finish_request_fingerprint": row.FinishRequestFingerprint,
		})
	if result.Error != nil {
		if uniqueViolation(result.Error, "") {
			return ErrTrainerSessionRace
		}
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrTrainerSessionRace
	}
	return nil
}

// uniqueViolation matches SQLSTATE 23505, optionally for one constraint.
func uniqueViolation(err error, constraint string) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505" && (constraint == "" || pg.ConstraintName == constraint)
}
