package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type fakePrivacyStore struct {
	request      *model.PrivacyErasureRequest
	counts       []repository.PrivacyDataCount
	markRetryErr error
}

func (f *fakePrivacyStore) DeleteRestrictBoundAggregates(_ context.Context, _ uuid.UUID) error {
	return nil
}
func (f *fakePrivacyStore) CountUserData(_ context.Context, _ uuid.UUID) ([]repository.PrivacyDataCount, error) {
	return f.counts, nil
}
func (f *fakePrivacyStore) CreateOrGet(_ context.Context, userID uuid.UUID, digest string, report datatypes.JSON) (*model.PrivacyErasureRequest, error) {
	if f.request == nil {
		f.request = &model.PrivacyErasureRequest{ID: uuid.New(), SubjectUserID: &userID, SubjectDigest: digest, Status: "pending", Report: report}
	}
	return f.request, nil
}
func (f *fakePrivacyStore) GetByID(_ context.Context, id uuid.UUID) (*model.PrivacyErasureRequest, error) {
	if f.request == nil || f.request.ID != id {
		return nil, nil
	}
	return f.request, nil
}
func (f *fakePrivacyStore) TryClaim(_ context.Context, id uuid.UUID, owner string, _ time.Duration) (*model.PrivacyErasureRequest, bool, error) {
	if f.request == nil || f.request.ID != id || f.request.Status == "completed" || f.request.Status == "running" {
		return nil, false, nil
	}
	f.request.Status = "running"
	f.request.AttemptCount++
	f.request.LeaseOwner = &owner
	return f.request, true, nil
}
func (f *fakePrivacyStore) MarkRetryable(_ context.Context, id uuid.UUID, failure string) error {
	if f.markRetryErr != nil {
		return f.markRetryErr
	}
	f.request.Status = "retryable"
	f.request.LastError = &failure
	f.request.LeaseOwner = nil
	return nil
}
func (f *fakePrivacyStore) MarkCompleted(_ context.Context, id uuid.UUID) error {
	f.request.Status = "completed"
	f.request.SubjectUserID = nil
	f.request.LastError = nil
	f.request.LeaseOwner = nil
	now := time.Now()
	f.request.CompletedAt = &now
	return nil
}
func (f *fakePrivacyStore) ListRecoverable(_ context.Context, _ int) ([]uuid.UUID, error) {
	if f.request != nil && (f.request.Status == "pending" || f.request.Status == "retryable") {
		return []uuid.UUID{f.request.ID}, nil
	}
	return nil, nil
}

type fakePrivacyUserEraser struct{ calls int }

func (f *fakePrivacyUserEraser) DeleteByID(_ context.Context, _ uuid.UUID) error {
	f.calls++
	return nil
}

type fakePrivacyAuth struct {
	calls int
	err   error
}

func (f *fakePrivacyAuth) RevokeUser(_ context.Context, _ uuid.UUID) error { f.calls++; return f.err }

type fakePrivacyObjects struct {
	calls    int
	failOnce bool
}

func (f *fakePrivacyObjects) EraseUserObjects(_ context.Context, _ uuid.UUID) error {
	f.calls++
	if f.failOnce {
		f.failOnce = false
		return errors.New("object store unavailable")
	}
	return nil
}

type fakePrivacyTx struct{ calls int }

func (f *fakePrivacyTx) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	f.calls++
	return fn(ctx)
}

func newFakePrivacyService(store *fakePrivacyStore, objects *fakePrivacyObjects) (*PrivacyErasureService, *fakePrivacyAuth, *fakePrivacyUserEraser, *fakePrivacyTx) {
	auth := &fakePrivacyAuth{}
	users := &fakePrivacyUserEraser{}
	tx := &fakePrivacyTx{}
	return &PrivacyErasureService{
		requests: store, users: users, auth: auth, objects: objects, transactions: tx,
		workerID: "test-worker", leaseDuration: time.Minute,
	}, auth, users, tx
}

func TestPrivacyErasureRequestRequiresExactConfirmation(t *testing.T) {
	service, _, _, _ := newFakePrivacyService(&fakePrivacyStore{}, &fakePrivacyObjects{})
	_, err := service.Request(context.Background(), uuid.New(), "delete")
	if !errors.Is(err, ErrPrivacyErasureConfirmation) {
		t.Fatalf("Request error=%v, want confirmation error", err)
	}
}

func TestPrivacyErasureRequestCompletesAndAnonymizesAudit(t *testing.T) {
	store := &fakePrivacyStore{counts: []repository.PrivacyDataCount{{Name: "account", Count: 1}, {Name: "uploads", Count: 2}}}
	objects := &fakePrivacyObjects{}
	service, auth, users, tx := newFakePrivacyService(store, objects)
	userID := uuid.New()

	request, err := service.Request(context.Background(), userID, PrivacyErasureConfirmationPhrase)
	if err != nil {
		t.Fatalf("Request: %v", err)
	}
	if request.Status != "completed" || request.SubjectUserID != nil || request.CompletedAt == nil {
		t.Fatalf("completed request=%+v", request)
	}
	if auth.calls != 1 || objects.calls != 1 || users.calls != 1 || tx.calls != 1 {
		t.Fatalf("calls auth=%d objects=%d users=%d tx=%d", auth.calls, objects.calls, users.calls, tx.calls)
	}
	if request.SubjectDigest == "" || request.SubjectDigest == userID.String() {
		t.Fatalf("unsafe subject digest %q", request.SubjectDigest)
	}
}

func TestPrivacyErasurePartialFailureBecomesRetryableAndRecovers(t *testing.T) {
	store := &fakePrivacyStore{counts: []repository.PrivacyDataCount{{Name: "account", Count: 1}}}
	objects := &fakePrivacyObjects{failOnce: true}
	service, auth, users, tx := newFakePrivacyService(store, objects)
	userID := uuid.New()

	request, err := service.Request(context.Background(), userID, PrivacyErasureConfirmationPhrase)
	if err != nil {
		t.Fatalf("Request: %v", err)
	}
	if request.Status != "retryable" || request.LastError == nil {
		t.Fatalf("first request=%+v, want retryable", request)
	}
	if users.calls != 0 || tx.calls != 0 {
		t.Fatal("database deletion ran after object cleanup failure")
	}

	processed, err := service.Recover(context.Background(), 10)
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if processed != 1 || store.request.Status != "completed" {
		t.Fatalf("processed=%d status=%s", processed, store.request.Status)
	}
	if auth.calls != 2 || objects.calls != 2 || users.calls != 1 || tx.calls != 1 {
		t.Fatalf("retry calls auth=%d objects=%d users=%d tx=%d", auth.calls, objects.calls, users.calls, tx.calls)
	}
}

func TestLocalUserObjectCleanerDeletesOnlyUserPrefix(t *testing.T) {
	root := t.TempDir()
	userA := uuid.New()
	userB := uuid.New()
	pathA := filepath.Join(root, userA.String(), "photo.jpg")
	pathB := filepath.Join(root, userB.String(), "photo.jpg")
	for _, path := range []string{pathA, pathB} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("private"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	cleaner := NewLocalUserObjectCleaner(root)
	if err := cleaner.EraseUserObjects(context.Background(), userA); err != nil {
		t.Fatalf("EraseUserObjects: %v", err)
	}
	if _, err := os.Stat(pathA); !os.IsNotExist(err) {
		t.Fatalf("user A object still exists or unexpected error: %v", err)
	}
	if _, err := os.Stat(pathB); err != nil {
		t.Fatalf("user B object was affected: %v", err)
	}
}
