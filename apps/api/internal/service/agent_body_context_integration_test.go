package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
)

// The agent view reads every V2 section through the real repositories inside
// one read-only REPEATABLE READ snapshot (repository transactions become
// savepoints) and never mixes owners.
func TestAgentBodyContextOwnerIsolationPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	owner := seedOnboardingContextUser(t, db)
	foreign := seedOnboardingContextUser(t, db)
	empty := seedOnboardingContextUser(t, db)
	repo := repository.NewBodyStateRepository(db)
	equipment := repository.NewEquipmentAssetRepository(db)
	svc := NewBodyStateService(repo).WithAgentContextSources(database.NewTransactionManager(db), equipment)
	ctx := context.Background()

	seed := func(user uuid.UUID, statement, guess, gear string) uuid.UUID {
		t.Helper()
		commit, err := repo.RecordDirectFacts(ctx, user, model.RecordBodyFactsCommand{
			IdempotencyKey: "agent-view-" + user.String(), ExpectedRevision: 0,
			Facts: []model.BodyFactInput{{
				Kind: "symptom", Statement: statement, BodyRegionIDs: []string{},
				Temporal: model.FactTemporal{Precision: "unknown"}, Details: json.RawMessage(`{"presence":"present"}`),
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
		var ids []uuid.UUID
		if err := json.Unmarshal(commit.FactIDs, &ids); err != nil || len(ids) != 1 {
			t.Fatalf("fact ids: %s %v", commit.FactIDs, err)
		}
		if _, err := repo.RecordBodyHypothesis(ctx, user, model.RecordBodyHypothesisCommand{
			IdempotencyKey: "agent-view-guess-" + user.String(), ExpectedRevision: commit.Revision, Statement: guess, RelatedFactIDs: []uuid.UUID{ids[0]},
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := equipment.Create(ctx, user, model.CreateEquipmentAssetCommand{
			IdempotencyKey: "agent-view-gear-" + user.String(),
			EquipmentAssetFields: model.EquipmentAssetFields{
				DisplayName: gear, AccessKind: model.EquipmentAccessOwned, Availability: model.EquipmentAvailable,
			},
		}); err != nil {
			t.Fatal(err)
		}
		return ids[0]
	}
	ownerFact := seed(owner, "synthetic owner knee statement", "synthetic owner guess", "synthetic owner band")
	seed(foreign, "synthetic foreign statement", "synthetic foreign guess", "synthetic foreign bar")

	view, err := svc.GetAgentBodyContext(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if view.UserID != owner || view.CurrentRevision != 2 || !view.RecordsAvailable {
		t.Fatalf("owner view head: %#v", view)
	}
	if len(view.Facts) != 1 || view.Facts[0].ID != ownerFact || view.Facts[0].SafetyScreen != AgentSymptomUnscreened {
		t.Fatalf("owner facts: %#v", view.Facts)
	}
	if view.Safety.Status != AgentSafetyScreeningIncomplete || len(view.Safety.UnscreenedFactIDs) != 1 || view.Safety.UnscreenedFactIDs[0] != ownerFact {
		t.Fatalf("unscreened symptom must stay visible: %#v", view.Safety)
	}
	if len(view.UserHypotheses) != 1 || view.UserHypotheses[0].Statement != "synthetic owner guess" ||
		view.UserHypotheses[0].Authorship != AgentAuthorshipUserAuthored || len(view.UserHypotheses[0].RelatedFactIDs) != 1 {
		t.Fatalf("owner hypotheses: %#v", view.UserHypotheses)
	}
	if len(view.Equipment) != 1 || view.Equipment[0].DisplayName != "synthetic owner band" {
		t.Fatalf("owner equipment: %#v", view.Equipment)
	}
	raw, _ := json.Marshal(view)
	for _, leak := range []string{"synthetic foreign statement", "synthetic foreign guess", "synthetic foreign bar"} {
		if strings.Contains(string(raw), leak) {
			t.Fatalf("foreign data leaked into owner view: %q", leak)
		}
	}

	none, err := svc.GetAgentBodyContext(ctx, empty)
	if err != nil {
		t.Fatal(err)
	}
	if none.CurrentRevision != 0 || len(none.Facts) != 0 || len(none.Equipment) != 0 || len(none.UserHypotheses) != 0 ||
		none.Safety.Status != AgentSafetyNoCurrentSymptoms {
		t.Fatalf("empty owner view: %#v", none)
	}
}
