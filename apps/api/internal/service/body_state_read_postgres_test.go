package service

import (
	"context"
	"testing"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
)

type concurrentNativeReadRepository struct {
	*repository.BodyStateRepository
	beforeFacts func()
}

func (r *concurrentNativeReadRepository) ListFacts(ctx context.Context, owner uuid.UUID, before int64) (*model.BodyFactPage, error) {
	if r.beforeFacts != nil {
		fn := r.beforeFacts
		r.beforeFacts = nil
		fn()
	}
	return r.BodyStateRepository.ListFacts(ctx, owner, before)
}

func TestNativeBodyStateFrozenV2ReadPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	owner := seedOnboardingContextUser(t, db)
	foreign := seedOnboardingContextUser(t, db)
	ctx := context.Background()
	repo := repository.NewBodyStateRepository(db)
	record := func(user uuid.UUID, revision int64, text string) *model.BodyFactCommit {
		t.Helper()
		commit, err := repo.RecordDirectFacts(ctx, user, model.RecordBodyFactsCommand{IdempotencyKey: uuid.NewString(), ExpectedRevision: revision, Facts: []model.BodyFactInput{{Kind: "symptom", Statement: text, BodyRegionIDs: []string{}, Temporal: model.FactTemporal{Precision: "unknown"}, Details: []byte(`{"presence":"present"}`)}}})
		if err != nil {
			t.Fatal(err)
		}
		return commit
	}
	record(owner, 0, "我的膝盖痛")
	record(foreign, 0, "foreign statement")
	wrapped := &concurrentNativeReadRepository{BodyStateRepository: repo}
	wrapped.beforeFacts = func() { record(owner, 1, "稍后新报告肩痛") }
	svc := NewBodyStateService(wrapped).WithAgentContextSources(database.NewTransactionManager(db), nil)
	frozen, err := svc.GetBodyState(ctx, owner, 20)
	if err != nil {
		t.Fatal(err)
	}
	if frozen.CurrentRevision != 1 || len(frozen.Facts) != 1 || frozen.Facts[0].Statement != "我的膝盖痛" || frozen.Facts[0].Kind != "symptom" {
		t.Fatalf("snapshot mixed revisions or legacy/foreign rows: %+v", frozen)
	}
	for _, revision := range frozen.RecentRevisions {
		if revision.UserID != owner || revision.Revision > 1 {
			t.Fatalf("unfrozen history: %+v", revision)
		}
	}
	current, err := svc.GetBodyState(ctx, owner, 20)
	if err != nil {
		t.Fatal(err)
	}
	if current.CurrentRevision != 2 || len(current.Facts) != 2 {
		t.Fatalf("later read did not see committed facts: %+v", current)
	}
	envelope, err := ProjectSafetyEnvelopeV2(current)
	if err != nil || envelope.Coverage.Complete || len(envelope.Coverage.IncompleteSourceRefs) != 2 {
		t.Fatalf("unscreened symptoms must fail closed: %+v %v", envelope, err)
	}
}
