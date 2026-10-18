package service

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

var (
	ErrDiagnosisReviewPromptInvalid = errors.New("invalid diagnosis review prompt dismissal")
	// ErrDiagnosisReviewPromptNotShown: the Condition has no dismissable prompt
	// (current, stale, or not adopted). A stale Condition is never hidden.
	ErrDiagnosisReviewPromptNotShown  = errors.New("diagnosis review prompt is not shown for this condition")
	ErrDiagnosisReviewPromptKeyReused = errors.New("review prompt dismissal key reused for a different prompt")
)

// NativeDiagnosisReviewPromptDismissal is set on a reviewed Condition only
// while the user's latest dismissal still covers every review reason shown.
type NativeDiagnosisReviewPromptDismissal struct {
	DismissalID                       uuid.UUID `json:"dismissalId"`
	DismissedThroughBodyStateRevision int64     `json:"dismissedThroughBodyStateRevision"`
	DismissedAt                       time.Time `json:"dismissedAt"`
}

// DiagnosisReviewPromptDismissCommand is the user's explicit "not now" on the
// review prompt they were shown: the Diagnosis revision and the BodyState
// revision the prompt was evaluated at must still be current.
type DiagnosisReviewPromptDismissCommand struct {
	ConditionID                uuid.UUID
	ExpectedRevision           int64
	EvaluatedBodyStateRevision int64
	IdempotencyKey             string
}

func (c DiagnosisReviewPromptDismissCommand) valid() bool {
	return c.ConditionID != uuid.Nil && c.ExpectedRevision >= 1 && c.EvaluatedBodyStateRevision >= 0 &&
		diagnosisTrimmedText(c.IdempotencyKey, 128)
}

type diagnosisReviewPromptStore interface {
	FindReviewPromptDismissalByKey(context.Context, uuid.UUID, string) (*model.DiagnosisReviewPromptDismissal, error)
	InsertReviewPromptDismissal(context.Context, *model.DiagnosisReviewPromptDismissal) error
	LatestReviewPromptDismissals(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]model.DiagnosisReviewPromptDismissal, error)
}

type diagnosisReviewedRead interface {
	Read(context.Context, uuid.UUID) (*NativeDiagnosisView, error)
}

// DiagnosisReviewPromptService records the minimal UI dismissal of §3.6. It
// never writes a Diagnosis revision, a Condition, or anything an Agent reads.
type DiagnosisReviewPromptService struct {
	store  diagnosisReviewPromptStore
	reader diagnosisReviewedRead
	now    func() time.Time
}

func NewDiagnosisReviewPromptService(db *gorm.DB, reader diagnosisReviewedRead) *DiagnosisReviewPromptService {
	return &DiagnosisReviewPromptService{store: repository.NewDiagnosisRepository(db), reader: reader,
		now: func() time.Time { return time.Now().UTC() }}
}

func (s *DiagnosisReviewPromptService) Dismiss(ctx context.Context, owner uuid.UUID, c DiagnosisReviewPromptDismissCommand) (*model.DiagnosisReviewPromptDismissal, error) {
	if owner == uuid.Nil || !c.valid() {
		return nil, ErrDiagnosisReviewPromptInvalid
	}
	if s == nil || s.store == nil || s.reader == nil {
		return nil, errors.New("diagnosis review prompt service is not configured")
	}
	if replay, err := s.replay(ctx, owner, c); replay != nil || err != nil {
		return replay, err
	}
	view, err := s.reader.Read(ctx, owner)
	if err != nil {
		return nil, err
	}
	if view == nil || view.UserID != owner {
		return nil, errors.New("diagnosis review prompt read owner mismatch")
	}
	var condition *NativeDiagnosisCondition
	for i := range view.ConditionEntries {
		if view.ConditionEntries[i].ConditionID == c.ConditionID {
			condition = &view.ConditionEntries[i]
		}
	}
	if condition == nil {
		return nil, model.ErrDiagnosisNotFound
	}
	freshness := condition.Freshness
	if freshness == nil {
		return nil, errors.New("diagnosis condition freshness missing")
	}
	if view.CurrentRevision != c.ExpectedRevision || freshness.EvaluatedBodyStateRevision != c.EvaluatedBodyStateRevision {
		return nil, ErrDiagnosisDecisionConflict
	}
	if condition.Disposition != model.ConditionAdopted || freshness.State != DiagnosisFreshnessReviewRecommended || len(freshness.Reasons) == 0 {
		return nil, ErrDiagnosisReviewPromptNotShown
	}
	row := &model.DiagnosisReviewPromptDismissal{
		UserID: owner, ID: uuid.New(), ConditionID: c.ConditionID,
		BasedOnBodyStateRevision: freshness.BasedOnBodyStateRevision, DismissedThroughBodyStateRevision: freshness.EvaluatedBodyStateRevision,
		DiagnosisRevision: view.CurrentRevision, ReasonCodes: diagnosisReviewReasonCodes(freshness.Reasons),
		IdempotencyKey: c.IdempotencyKey, DismissedAt: s.now().UTC().Truncate(time.Microsecond),
	}
	if err := s.store.InsertReviewPromptDismissal(ctx, row); err != nil {
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23505" {
			if replay, replayErr := s.replay(ctx, owner, c); replay != nil || replayErr != nil {
				return replay, replayErr
			}
		}
		return nil, err
	}
	return row, nil
}

func (s *DiagnosisReviewPromptService) replay(ctx context.Context, owner uuid.UUID, c DiagnosisReviewPromptDismissCommand) (*model.DiagnosisReviewPromptDismissal, error) {
	existing, err := s.store.FindReviewPromptDismissalByKey(ctx, owner, c.IdempotencyKey)
	if err != nil || existing == nil {
		return nil, err
	}
	if existing.UserID != owner || existing.ConditionID != c.ConditionID || existing.DiagnosisRevision != c.ExpectedRevision ||
		existing.DismissedThroughBodyStateRevision != c.EvaluatedBodyStateRevision {
		return nil, ErrDiagnosisReviewPromptKeyReused
	}
	return existing, nil
}

func diagnosisReviewReasonCodes(reasons []DiagnosisFreshnessReason) []string {
	seen := map[string]bool{}
	codes := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		if !seen[reason.Code] {
			seen[reason.Code] = true
			codes = append(codes, reason.Code)
		}
	}
	sort.Strings(codes)
	return codes
}

// diagnosisReviewPromptDismissed reports whether dismissal still covers the
// prompt freshness shows now: same analysis basis, no stale cause, every
// revision-attributed reason at or before the revision the user dismissed,
// and every unattributed reason one the user saw. New substantive evidence
// therefore raises the prompt again; a new formal analysis resets it.
func diagnosisReviewPromptDismissed(f *DiagnosisFreshness, d *model.DiagnosisReviewPromptDismissal) bool {
	if f == nil || d == nil || f.State != DiagnosisFreshnessReviewRecommended || d.BasedOnBodyStateRevision != f.BasedOnBodyStateRevision {
		return false
	}
	seen := map[string]bool{}
	for _, code := range d.ReasonCodes {
		seen[code] = true
	}
	for _, reason := range f.Reasons {
		if diagnosisFreshnessStaleCodes[reason.Code] {
			return false
		}
		if reason.BodyStateRevision != nil {
			if *reason.BodyStateRevision > d.DismissedThroughBodyStateRevision {
				return false
			}
			continue
		}
		if !seen[reason.Code] {
			return false
		}
	}
	return true
}
