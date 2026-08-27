package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

var ErrInvalidLifestyleCandidate = errors.New("invalid lifestyle candidate")

type lifestyleBodyState interface {
	GetSnapshot(ctx context.Context, userID uuid.UUID, historyLimit int) (*BodyStateSnapshot, error)
	ListReviewableFacts(ctx context.Context, userID uuid.UUID, limit int) ([]model.BodyStateFact, error)
	ApplyCurrentContextPatch(ctx context.Context, userID uuid.UUID, expectedRevision *int64, patch model.BodyStateCurrentContextPatch, source string) (*model.BodyStateRevision, error)
	AcceptCurrentFactCandidate(ctx context.Context, userID uuid.UUID, expectedRevision *int64, candidateID uuid.UUID, effectiveAt time.Time) (*model.BodyStateFact, *model.BodyStateRevision, error)
	ReviewFact(ctx context.Context, userID uuid.UUID, expectedRevision *int64, factID uuid.UUID, reviewState string) (*model.BodyStateFact, *model.BodyStateRevision, error)
}

// LifestyleService is an application facade over BodyState. It owns taxonomy
// and projection semantics, but deliberately owns no persistence table.
type LifestyleService struct {
	bodyState lifestyleBodyState
}

func NewLifestyleService(bodyState lifestyleBodyState) *LifestyleService {
	return &LifestyleService{bodyState: bodyState}
}

func (s *LifestyleService) Get(ctx context.Context, userID uuid.UUID) (*dto.LifestyleSnapshot, error) {
	snapshot, err := s.bodyState.GetSnapshot(ctx, userID, 0)
	if err != nil {
		return nil, err
	}
	result := projectLifestyle(snapshot)
	pending, err := s.bodyState.ListReviewableFacts(ctx, userID, 100)
	if err != nil {
		return nil, err
	}
	result.PendingUpdates = projectLifestyleCandidates(pending)
	return result, nil
}

func (s *LifestyleService) Update(ctx context.Context, userID uuid.UUID, request dto.UpdateLifestyleRequest) (*dto.LifestyleSnapshot, error) {
	patch := buildLifestyleContextPatch(request, time.Now().UTC(), "user_edited", "lifestyle_editor")
	if len(patch.Facts) > 0 {
		if _, err := s.bodyState.ApplyCurrentContextPatch(ctx, userID, request.ExpectedRevision, patch, "lifestyle_editor"); err != nil {
			return nil, fmt.Errorf("update lifestyle: %w", err)
		}
	}
	return s.Get(ctx, userID)
}

func buildLifestyleContextPatch(request dto.UpdateLifestyleRequest, effectiveAt time.Time, origin, sourceType string) model.BodyStateCurrentContextPatch {
	type sectionUpdate struct {
		name  string
		kind  string
		input *dto.LifestyleSectionInput
	}
	updates := []sectionUpdate{
		{"activity", model.BodyStateFactKindLifestyleActivity, request.Activity},
		{"sleep", model.BodyStateFactKindLifestyleSleep, request.Sleep},
		{"exercise", model.BodyStateFactKindLifestyleExercise, request.Exercise},
		{"nutrition", model.BodyStateFactKindLifestyleNutrition, request.Nutrition},
		{"substances", model.BodyStateFactKindLifestyleSubstances, request.Substances},
		{"recovery", model.BodyStateFactKindLifestyleRecovery, request.Recovery},
	}
	patch := model.BodyStateCurrentContextPatch{}
	for _, update := range updates {
		if update.input == nil {
			continue
		}
		var replacement *model.BodyStateFact
		if summary := strings.TrimSpace(update.input.Summary); summary != "" {
			details := update.input.Details
			if len(details) == 0 || !json.Valid(details) {
				details = json.RawMessage(`{}`)
			}
			provenance, _ := json.Marshal(map[string]any{"source_type": sourceType, "section": update.name})
			replacement = &model.BodyStateFact{
				ConcernKey:     "lifestyle:" + update.name,
				Kind:           update.kind,
				Value:          summary,
				Details:        datatypes.JSON(details),
				Origin:         origin,
				ReviewState:    "confirmed",
				LifecycleState: "active",
				Trend:          "unknown",
				Provenance:     datatypes.JSON(provenance),
			}
		}
		patch.Facts = append(patch.Facts, model.BodyStateCurrentFactMutation{
			Kind: update.kind, Replacement: replacement, EffectiveAt: effectiveAt,
		})
	}
	return patch
}

func (s *LifestyleService) AcceptCandidate(
	ctx context.Context,
	userID uuid.UUID,
	expectedRevision *int64,
	candidateID uuid.UUID,
) (*dto.LifestyleSnapshot, error) {
	if err := s.validateCandidate(ctx, userID, candidateID); err != nil {
		return nil, err
	}
	if _, _, err := s.bodyState.AcceptCurrentFactCandidate(
		ctx, userID, expectedRevision, candidateID, time.Now().UTC(),
	); err != nil {
		return nil, err
	}
	return s.Get(ctx, userID)
}

func (s *LifestyleService) RejectCandidate(
	ctx context.Context,
	userID uuid.UUID,
	expectedRevision *int64,
	candidateID uuid.UUID,
) (*dto.LifestyleSnapshot, error) {
	if err := s.validateCandidate(ctx, userID, candidateID); err != nil {
		return nil, err
	}
	if _, _, err := s.bodyState.ReviewFact(ctx, userID, expectedRevision, candidateID, "rejected"); err != nil {
		return nil, err
	}
	return s.Get(ctx, userID)
}

func (s *LifestyleService) validateCandidate(ctx context.Context, userID, candidateID uuid.UUID) error {
	pending, err := s.bodyState.ListReviewableFacts(ctx, userID, 200)
	if err != nil {
		return err
	}
	for _, fact := range pending {
		if fact.ID != candidateID {
			continue
		}
		if isLifestyleFactKind(fact.Kind) && fact.Origin == "ai_extracted" &&
			fact.ReviewState == "unverified" && fact.LifecycleState == "active" && fact.ExcludedFromReasoning {
			return nil
		}
		break
	}
	return fmt.Errorf("%w: %s", ErrInvalidLifestyleCandidate, candidateID)
}

func projectLifestyleCandidates(facts []model.BodyStateFact) []dto.LifestyleCandidate {
	result := make([]dto.LifestyleCandidate, 0)
	for _, fact := range facts {
		if !isLifestyleFactKind(fact.Kind) || fact.Origin != "ai_extracted" ||
			fact.ReviewState != "unverified" || fact.LifecycleState != "active" || !fact.ExcludedFromReasoning {
			continue
		}
		details := json.RawMessage(fact.Details)
		if len(details) == 0 {
			details = json.RawMessage(`{}`)
		}
		result = append(result, dto.LifestyleCandidate{
			FactID: fact.ID, Kind: fact.Kind, Summary: fact.Value, Details: details, CreatedAt: fact.CreatedAt,
		})
	}
	return result
}

func isLifestyleFactKind(kind string) bool {
	switch kind {
	case model.BodyStateFactKindLifestyleActivity,
		model.BodyStateFactKindLifestyleSleep,
		model.BodyStateFactKindLifestyleExercise,
		model.BodyStateFactKindLifestyleNutrition,
		model.BodyStateFactKindLifestyleSubstances,
		model.BodyStateFactKindLifestyleRecovery:
		return true
	default:
		return false
	}
}

func projectLifestyle(snapshot *BodyStateSnapshot) *dto.LifestyleSnapshot {
	result := &dto.LifestyleSnapshot{
		CurrentRevision: snapshot.CurrentRevision,
		Activity:        emptyLifestyleSection(model.BodyStateFactKindLifestyleActivity),
		Sleep:           emptyLifestyleSection(model.BodyStateFactKindLifestyleSleep),
		Exercise:        emptyLifestyleSection(model.BodyStateFactKindLifestyleExercise),
		Nutrition:       emptyLifestyleSection(model.BodyStateFactKindLifestyleNutrition),
		Substances:      emptyLifestyleSection(model.BodyStateFactKindLifestyleSubstances),
		Recovery:        emptyLifestyleSection(model.BodyStateFactKindLifestyleRecovery),
		PendingUpdates:  []dto.LifestyleCandidate{},
	}
	for index := range snapshot.Facts {
		fact := snapshot.Facts[index]
		if fact.ReviewState != "confirmed" || fact.LifecycleState != "active" || fact.ExcludedFromReasoning {
			continue
		}
		section := lifestyleSectionFromFact(fact)
		switch fact.Kind {
		case model.BodyStateFactKindLifestyleActivity:
			result.Activity = section
		case model.BodyStateFactKindLifestyleSleep:
			result.Sleep = section
		case model.BodyStateFactKindLifestyleExercise:
			result.Exercise = section
		case model.BodyStateFactKindLifestyleNutrition:
			result.Nutrition = section
		case model.BodyStateFactKindLifestyleSubstances:
			result.Substances = section
		case model.BodyStateFactKindLifestyleRecovery:
			result.Recovery = section
		}
	}
	return result
}

func emptyLifestyleSection(kind string) dto.LifestyleSection {
	return dto.LifestyleSection{Kind: kind, Summary: "", Details: json.RawMessage(`{}`)}
}

func lifestyleSectionFromFact(fact model.BodyStateFact) dto.LifestyleSection {
	details := json.RawMessage(fact.Details)
	if len(details) == 0 {
		details = json.RawMessage(`{}`)
	}
	updatedAt := fact.UpdatedAt
	return dto.LifestyleSection{
		Kind: fact.Kind, FactID: &fact.ID, Summary: fact.Value, Details: details,
		ValidFrom: fact.ValidFrom, UpdatedAt: &updatedAt, ReviewState: fact.ReviewState,
	}
}
