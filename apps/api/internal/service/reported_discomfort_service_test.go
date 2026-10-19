package service

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type fakeDiscomfortStore struct {
	tx     *fakeOnboardingTransactions
	writes [][]string
	inTx   []bool
	row    *model.ReportedDiscomfortAreas
}

func (f *fakeDiscomfortStore) Replace(_ context.Context, _ uuid.UUID, codes []string, reportedAt time.Time) error {
	f.writes = append(f.writes, codes)
	f.inTx = append(f.inTx, f.tx != nil && f.tx.calls == 1)
	encoded, _ := json.Marshal(codes)
	f.row = &model.ReportedDiscomfortAreas{AreaCodes: encoded, Source: "onboarding", ReportedAt: reportedAt}
	return nil
}

func (f *fakeDiscomfortStore) Get(context.Context, uuid.UUID) (*model.ReportedDiscomfortAreas, error) {
	return f.row, nil
}

func newDiscomfortOnboarding() (*OnboardingContextService, *fakeOnboardingTransactions, *fakeDiscomfortStore, *memoryBodyContextStore) {
	transactions := &fakeOnboardingTransactions{}
	store := &fakeDiscomfortStore{tx: transactions}
	bodyState := &memoryBodyContextStore{}
	svc := NewOnboardingContextService(&fakeOnboardingProfileWriter{}, bodyState, transactions).WithReportedDiscomfort(store)
	return svc, transactions, store, bodyState
}

func TestOnboardingContextStoresDiscomfortSelfReportOutsideBodyState(t *testing.T) {
	svc, transactions, store, bodyState := newDiscomfortOnboarding()
	request := validOnboardingContextRequest()
	request.DiscomfortAreas = []string{"lower_back", "neck"}

	if _, err := svc.Submit(context.Background(), uuid.New(), request); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if transactions.calls != 1 || len(store.writes) != 1 || !store.inTx[0] {
		t.Fatalf("self-report must be written once inside the onboarding transaction: calls=%d writes=%v", transactions.calls, store.writes)
	}
	if want := []string{"neck", "lower_back"}; !reflect.DeepEqual(store.writes[0], want) {
		t.Fatalf("stored %v, want canonical %v", store.writes[0], want)
	}
	// Not a symptom: the BodyState ledger only gets the six lifestyle facts + injury.
	for _, fact := range bodyState.ledger.facts {
		if fact.Kind == "symptom" {
			t.Fatalf("discomfort self-report leaked into the ledger as a symptom: %#v", fact)
		}
	}
	if len(bodyState.ledger.facts) != 7 {
		t.Fatalf("ledger facts=%d, want 7 (unchanged by the self-report)", len(bodyState.ledger.facts))
	}

	snapshot, err := NewReportedDiscomfortService(store).Get(context.Background(), uuid.New())
	if err != nil || snapshot.Report == nil || !reflect.DeepEqual(snapshot.Report.AreaCodes, []string{"neck", "lower_back"}) || snapshot.Report.Source != "onboarding" {
		t.Fatalf("snapshot=%#v err=%v", snapshot, err)
	}
}

func TestOnboardingContextLeavesDiscomfortUntouchedWhenSkipped(t *testing.T) {
	svc, _, store, _ := newDiscomfortOnboarding()
	if _, err := svc.Submit(context.Background(), uuid.New(), validOnboardingContextRequest()); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if len(store.writes) != 0 {
		t.Fatalf("skipped step 05 wrote %v", store.writes)
	}
	snapshot, err := NewReportedDiscomfortService(store).Get(context.Background(), uuid.New())
	if err != nil || snapshot.Report != nil {
		t.Fatalf("unanswered step must read as report=null: %#v err=%v", snapshot, err)
	}
}

func TestOnboardingContextRejectsInvalidDiscomfortBeforeTransaction(t *testing.T) {
	for name, codes := range map[string][]string{
		"none with area": {"none", "hip"},
		"empty answer":   {},
		"unknown":        {"knee"},
	} {
		svc, transactions, store, _ := newDiscomfortOnboarding()
		request := validOnboardingContextRequest()
		request.DiscomfortAreas = codes
		_, err := svc.Submit(context.Background(), uuid.New(), request)
		if !errors.Is(err, ErrInvalidOnboardingContext) {
			t.Fatalf("%s: err=%v, want ErrInvalidOnboardingContext", name, err)
		}
		if transactions.calls != 0 || len(store.writes) != 0 {
			t.Fatalf("%s: invalid answer reached persistence", name)
		}
	}
}

func TestOnboardingContextRefusesToDropDiscomfortWithoutStore(t *testing.T) {
	transactions := &fakeOnboardingTransactions{}
	svc := NewOnboardingContextService(&fakeOnboardingProfileWriter{}, &memoryBodyContextStore{}, transactions)
	request := validOnboardingContextRequest()
	request.DiscomfortAreas = []string{"none"}
	if _, err := svc.Submit(context.Background(), uuid.New(), request); err == nil || transactions.calls != 0 {
		t.Fatalf("missing store must fail before the transaction: err=%v calls=%d", err, transactions.calls)
	}
}
