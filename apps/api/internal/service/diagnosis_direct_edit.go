package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var ErrDiagnosisDirectEditInvalid = errors.New("invalid confirmed diagnosis direct edit")

// DiagnosisDirectEditCommand is the user's confirmed direct edit of exactly one
// committed ConditionEntry (diagnosis.md 3.5). Withdraw sets disposition to
// inactive with reason user_withdrawal; correct_metadata replaces only the
// display note. Anything substantive needs a formal analysis run instead.
// Owner comes from authentication; no Agent payload can address this command.
type DiagnosisDirectEditCommand struct {
	ConditionID       uuid.UUID
	EditKind          model.ConditionDirectEditKind
	RequestEvidenceID uuid.UUID
	ExpectedRevision  int64
	Reason            string
	Notes             *string
	ToolCallID        string
	IdempotencyKey    string
}

const diagnosisDirectEditNotesMax = 1000

func (c DiagnosisDirectEditCommand) valid() bool {
	if c.ConditionID == uuid.Nil || c.RequestEvidenceID == uuid.Nil || c.ExpectedRevision < 0 ||
		!diagnosisTrimmedText(c.IdempotencyKey, 128) || !diagnosisTrimmedText(c.ToolCallID, 256) || !diagnosisTrimmedText(c.Reason, 500) {
		return false
	}
	switch c.EditKind {
	case model.ConditionWithdraw:
		return c.Notes == nil
	case model.ConditionCorrectMetadata:
		return c.Notes != nil && utf8.ValidString(*c.Notes) && utf8.RuneCountInString(*c.Notes) <= diagnosisDirectEditNotesMax &&
			strings.TrimSpace(*c.Notes) == *c.Notes
	default:
		return false
	}
}

func diagnosisTrimmedText(s string, max int) bool {
	return s != "" && utf8.ValidString(s) && utf8.RuneCountInString(s) <= max && strings.TrimSpace(s) == s
}

type DiagnosisDirectEditService struct {
	repo    *repository.DiagnosisRepository
	sources *repository.DiagnosisSourceRepository
	body    *BodyStateService
	tx      *database.TransactionManager
	now     func() time.Time
	// Test-only seam; production always uses the Go-owned live pinned ledger.
	projectSafety func(*BodyStateReadModel) (SafetyEnvelopeV2, error)
}

func NewDiagnosisDirectEditService(db *gorm.DB) *DiagnosisDirectEditService {
	return &DiagnosisDirectEditService{
		repo: repository.NewDiagnosisRepository(db), sources: repository.NewDiagnosisSourceRepository(db),
		body: NewBodyStateService(repository.NewBodyStateRepository(db)), tx: database.NewTransactionManager(db),
		now: func() time.Time { return time.Now().UTC() }, projectSafety: ProjectSafetyEnvelopeV2,
	}
}

// Apply persists one direct edit, its DiagnosisRevision, the changed entry and
// (for withdraw) the disposition history in one owner-fenced transaction. The
// head revision fence makes every open formal run stale, so an older run's
// decisions can never overwrite the direct edit. Safety is fail-closed: an
// urgent restriction or incomplete coverage changes nothing.
func (s *DiagnosisDirectEditService) Apply(ctx context.Context, owner uuid.UUID, c DiagnosisDirectEditCommand) (*model.ConditionDirectEdit, error) {
	if owner == uuid.Nil {
		return nil, model.ErrDiagnosisNotFound
	}
	if !c.valid() {
		return nil, ErrDiagnosisDirectEditInvalid
	}
	encoded, err := json.Marshal(struct {
		Owner     uuid.UUID
		Operation string
		Command   DiagnosisDirectEditCommand
	}{owner, "direct_edit", c})
	if err != nil {
		return nil, ErrDiagnosisDirectEditInvalid
	}
	sum := sha256.Sum256(encoded)
	fingerprint := hex.EncodeToString(sum[:])
	var result *model.ConditionDirectEdit
	err = s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		body, head, err := s.repo.LockDecisionFrame(ctx, owner)
		if err != nil {
			return err
		}
		existing, err := s.repo.FindDirectEditByKey(ctx, owner, c.IdempotencyKey)
		if err != nil {
			return err
		}
		if existing != nil {
			if existing.RequestFingerprint != fingerprint {
				return ErrDiagnosisDecisionConflict
			}
			result = existing
			return nil // immutable receipt replay; never re-applied to live state
		}
		if taken, err := s.repo.RevisionKeyTaken(ctx, owner, c.IdempotencyKey); err != nil || taken {
			if err != nil {
				return err
			}
			return ErrDiagnosisDecisionConflict
		}
		entry, err := s.repo.GetCondition(ctx, owner, c.ConditionID)
		if err != nil {
			return err
		}
		if c.ExpectedRevision != head.CurrentRevision || entry.CommittedRevision <= 0 || entry.CommittedRevision > head.CurrentRevision {
			return ErrDiagnosisDecisionConflict
		}
		request, err := s.repo.ResolveAnalysisRequest(ctx, owner, c.RequestEvidenceID)
		if err != nil {
			return err
		}
		if request.SourceKind != "user_message" || request.SourceMessageID == nil {
			return model.ErrDiagnosisNotFound
		}
		if err := s.sources.UserRequest(ctx, owner, request.ID, *request.SourceMessageID, request.SourceVersion); err != nil {
			return diagnosisEvidenceError(err)
		}
		snapshot, err := s.body.GetBodyState(ctx, owner, 1)
		if err != nil || snapshot == nil || snapshot.CurrentRevision != body.CurrentRevision {
			return ErrDiagnosisSafetyUnavailable
		}
		envelope, err := s.projectSafety(snapshot)
		if err != nil {
			return ErrDiagnosisSafetyUnavailable
		}
		for _, blocker := range envelope.ActiveBlockers {
			if blocker.RiskLevel == "urgent" {
				return ErrDiagnosisSafetyBlocked
			}
		}
		if !envelope.Coverage.Complete {
			return ErrDiagnosisSafetyUnavailable
		}
		now := s.now().UTC().Truncate(time.Microsecond)
		previous := entry.CommittedRevision
		next := head.CurrentRevision + 1
		change, err := applyDiagnosisDirectEdit(entry, c)
		if err != nil {
			return err
		}
		entry.CommittedRevision = next
		entry.UpdatedAt = now
		edit := &model.ConditionDirectEdit{
			UserID: owner, ID: uuid.New(), ConditionID: entry.ID, EditKind: c.EditKind,
			Changes: []model.ConditionDirectEditChange{change}, Reason: c.Reason,
			RequestMessageID: *request.SourceMessageID, RequestEvidenceID: request.ID, RequestSourceVersion: request.SourceVersion,
			ConfirmationToolCallID: c.ToolCallID, BasedOnDiagnosisRevision: head.CurrentRevision, ResultingDiagnosisRevision: next,
			DiagnosisRevisionID: uuid.New(), BodyStateRevision: body.CurrentRevision,
			IdempotencyKey: c.IdempotencyKey, RequestFingerprint: fingerprint, ReceiptID: uuid.New(), ConfirmedAt: now,
		}
		patch, err := json.Marshal([]map[string]any{{"kind": "direct_edit", "directEditId": edit.ID, "conditionId": entry.ID, "editKind": c.EditKind, "changes": edit.Changes}})
		if err != nil {
			return ErrDiagnosisDirectEditInvalid
		}
		replay, err := json.Marshal(map[string]any{"kind": "condition_direct_edit", "conditionId": entry.ID, "editKind": c.EditKind,
			"requestEvidenceId": request.ID, "requestSourceVersion": request.SourceVersion, "basedOnConditionRevision": previous, "changes": edit.Changes})
		if err != nil {
			return ErrDiagnosisDirectEditInvalid
		}
		provenance, err := json.Marshal(map[string]any{"actor": "user", "channel": "direct_edit", "confirmationToolCallId": c.ToolCallID})
		if err != nil {
			return ErrDiagnosisDirectEditInvalid
		}
		revision := &model.DiagnosisRevision{UserID: owner, ID: edit.DiagnosisRevisionID, PreviousRevision: head.CurrentRevision, Revision: next,
			DirectEditID: &edit.ID, Patch: datatypes.JSON(patch), DecisionIDs: []uuid.UUID{}, BodyStateRevision: body.CurrentRevision,
			EvidenceRefs: []model.DiagnosisEvidenceRef{}, ReplayInput: datatypes.JSON(replay), ExecutionProvenance: datatypes.JSON(provenance),
			IdempotencyKey: c.IdempotencyKey, RequestFingerprint: fingerprint, ReceiptID: edit.ReceiptID, CommittedAt: now}
		if err := s.repo.InsertSettledRevision(ctx, revision); err != nil {
			return err
		}
		if err := s.repo.InsertDirectEdit(ctx, edit); err != nil {
			return err
		}
		if err := s.repo.WriteDirectEditCondition(ctx, entry, previous); err != nil {
			return err
		}
		if c.EditKind == model.ConditionWithdraw {
			if err := s.repo.InsertDispositionHistory(ctx, &model.ConditionDispositionChange{UserID: owner, ID: uuid.New(), ConditionID: entry.ID,
				FromDisposition: model.ConditionAdopted, ToDisposition: model.ConditionInactive, Reason: "user_withdrawal",
				EvidenceRefs: []model.DiagnosisEvidenceRef{}, Details: c.Reason, DirectEditID: &edit.ID, CommittedRevision: next, ChangedAt: now}); err != nil {
				return err
			}
		}
		if err := s.repo.AdvanceDiagnosisHead(ctx, owner, head.CurrentRevision, next); err != nil {
			return err
		}
		result = edit
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// applyDiagnosisDirectEdit mutates only the permitted entry field and returns
// the exact server-read change. A no-op is refused: no empty revision exists.
func applyDiagnosisDirectEdit(entry *model.ConditionEntry, c DiagnosisDirectEditCommand) (model.ConditionDirectEditChange, error) {
	switch c.EditKind {
	case model.ConditionWithdraw:
		if entry.Disposition != model.ConditionAdopted {
			return model.ConditionDirectEditChange{}, ErrDiagnosisDecisionConflict
		}
		entry.Disposition = model.ConditionInactive
		return model.ConditionDirectEditChange{Field: "disposition", Before: string(model.ConditionAdopted), After: string(model.ConditionInactive)}, nil
	case model.ConditionCorrectMetadata:
		qualifiers := model.ConditionQualifiers{}
		if entry.Qualifiers != nil {
			qualifiers = *entry.Qualifiers
		}
		before := qualifiers.Notes
		if before == *c.Notes {
			return model.ConditionDirectEditChange{}, ErrDiagnosisDirectEditInvalid
		}
		qualifiers.Notes = *c.Notes
		if len(qualifiers.RegionIDs) == 0 && qualifiers.Laterality == "" && qualifiers.Notes == "" {
			entry.Qualifiers = nil
		} else {
			entry.Qualifiers = &qualifiers
		}
		return model.ConditionDirectEditChange{Field: "qualifiers.notes", Before: before, After: *c.Notes}, nil
	default:
		return model.ConditionDirectEditChange{}, ErrDiagnosisDirectEditInvalid
	}
}
