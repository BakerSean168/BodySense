package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// LifestyleService is an application facade over BodyState. It owns taxonomy
// and projection semantics, but deliberately owns no persistence table.
type LifestyleService struct {
	bodyState bodyContextStore
}

func NewLifestyleService(bodyState bodyContextStore) *LifestyleService {
	return &LifestyleService{bodyState: bodyState}
}

func (s *LifestyleService) Get(ctx context.Context, userID uuid.UUID) (*LifestyleSnapshot, error) {
	current, err := s.bodyState.CurrentBodyContext(ctx, userID)
	if err != nil {
		return nil, err
	}
	return projectLifestyle(current), nil
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
	}
}

func emptyLifestyleSection(kind string) LifestyleSection {
	return LifestyleSection{Kind: kind, Summary: "", Details: json.RawMessage(`{}`)}
}
