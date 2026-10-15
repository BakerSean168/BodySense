package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

var ErrInvalidLifestyleCandidate = errors.New("invalid lifestyle candidate")

type lifestyleBodyState interface {
	bodyContextStore
	ListReviewableFacts(ctx context.Context, userID uuid.UUID, limit int) ([]model.BodyStateFact, error)
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

func (s *LifestyleService) Get(ctx context.Context, userID uuid.UUID) (*LifestyleSnapshot, error) {
	current, err := s.bodyState.CurrentBodyContext(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := projectLifestyle(current)
	pending, err := s.bodyState.ListReviewableFacts(ctx, userID, 100)
	if err != nil {
		return nil, err
	}
	result.PendingUpdates = projectLifestyleCandidates(pending)
	return result, nil
}

func (s *LifestyleService) Update(ctx context.Context, userID uuid.UUID, request UpdateLifestyleRequest) (*LifestyleSnapshot, error) {
	sections := lifestyleRequestSections(request)
	if len(sections) > 0 {
		if _, err := s.bodyState.UpdateBodyContext(ctx, userID, request.ExpectedRevision, BodyContextUpdate{
			Lifestyle: sections, Context: "lifestyle_editor",
		}); err != nil {
			return nil, fmt.Errorf("update lifestyle: %w", err)
		}
	}
	return s.Get(ctx, userID)
}

func lifestyleRequestSections(request UpdateLifestyleRequest) map[string]*LifestyleSectionInput {
	sections := map[string]*LifestyleSectionInput{}
	for name, input := range map[string]*LifestyleSectionInput{
		"activity": request.Activity, "sleep": request.Sleep, "exercise": request.Exercise,
		"nutrition": request.Nutrition, "substances": request.Substances, "recovery": request.Recovery,
	} {
		if input != nil {
			sections[name] = input
		}
	}
	return sections
}

func (s *LifestyleService) AcceptCandidate(
	ctx context.Context,
	userID uuid.UUID,
	expectedRevision *int64,
	candidateID uuid.UUID,
) (*LifestyleSnapshot, error) {
	candidate, err := s.findCandidate(ctx, userID, candidateID)
	if err != nil {
		return nil, err
	}
	section := strings.TrimPrefix(candidate.Kind, "lifestyle.")
	if _, err := s.bodyState.UpdateBodyContext(ctx, userID, expectedRevision, BodyContextUpdate{
		Lifestyle: map[string]*LifestyleSectionInput{section: {Summary: candidate.Value, Details: json.RawMessage(candidate.Details)}},
		Context:   "lifestyle_candidate",
	}); err != nil {
		return nil, err
	}
	// The extraction record is closed so it is no longer offered for review.
	if _, _, err := s.bodyState.ReviewFact(ctx, userID, nil, candidateID, "confirmed"); err != nil {
		return nil, err
	}
	return s.Get(ctx, userID)
}

func (s *LifestyleService) RejectCandidate(
	ctx context.Context,
	userID uuid.UUID,
	expectedRevision *int64,
	candidateID uuid.UUID,
) (*LifestyleSnapshot, error) {
	if err := s.validateCandidate(ctx, userID, candidateID); err != nil {
		return nil, err
	}
	if _, _, err := s.bodyState.ReviewFact(ctx, userID, expectedRevision, candidateID, "rejected"); err != nil {
		return nil, err
	}
	return s.Get(ctx, userID)
}

func (s *LifestyleService) validateCandidate(ctx context.Context, userID, candidateID uuid.UUID) error {
	_, err := s.findCandidate(ctx, userID, candidateID)
	return err
}

func (s *LifestyleService) findCandidate(ctx context.Context, userID, candidateID uuid.UUID) (*model.BodyStateFact, error) {
	pending, err := s.bodyState.ListReviewableFacts(ctx, userID, 200)
	if err != nil {
		return nil, err
	}
	for index := range pending {
		fact := pending[index]
		if fact.ID != candidateID {
			continue
		}
		if isLifestyleFactKind(fact.Kind) && fact.Origin == "ai_extracted" &&
			fact.ReviewState == "unverified" && fact.LifecycleState == "active" && fact.ExcludedFromReasoning {
			return &fact, nil
		}
		break
	}
	return nil, fmt.Errorf("%w: %s", ErrInvalidLifestyleCandidate, candidateID)
}

func projectLifestyleCandidates(facts []model.BodyStateFact) []LifestyleCandidate {
	result := make([]LifestyleCandidate, 0)
	for _, fact := range facts {
		if !isLifestyleFactKind(fact.Kind) || fact.Origin != "ai_extracted" ||
			fact.ReviewState != "unverified" || fact.LifecycleState != "active" || !fact.ExcludedFromReasoning {
			continue
		}
		details := json.RawMessage(fact.Details)
		if len(details) == 0 {
			details = json.RawMessage(`{}`)
		}
		result = append(result, LifestyleCandidate{
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

func projectLifestyle(current *BodyContext) *LifestyleSnapshot {
	section := func(name string) LifestyleSection {
		kind := bodyContextLegacyLifestyleKind[name]
		fact := current.Lifestyle[name]
		if fact == nil {
			return emptyLifestyleSection(kind)
		}
		id := fact.ID
		recordedAt := fact.RecordedAt
		details := json.RawMessage(fact.Details)
		if len(details) == 0 {
			details = json.RawMessage(`{}`)
		}
		return LifestyleSection{
			Kind: kind, FactID: &id, Summary: fact.Statement, Details: details,
			ValidFrom: bodyContextFactTemporalStart(fact), UpdatedAt: &recordedAt, ReviewState: "confirmed",
		}
	}
	return &LifestyleSnapshot{
		CurrentRevision: current.Revision,
		Activity:        section("activity"),
		Sleep:           section("sleep"),
		Exercise:        section("exercise"),
		Nutrition:       section("nutrition"),
		Substances:      section("substances"),
		Recovery:        section("recovery"),
		PendingUpdates:  []LifestyleCandidate{},
	}
}

func emptyLifestyleSection(kind string) LifestyleSection {
	return LifestyleSection{Kind: kind, Summary: "", Details: json.RawMessage(`{}`)}
}
