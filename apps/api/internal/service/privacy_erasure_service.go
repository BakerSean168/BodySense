package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

const PrivacyErasureConfirmationPhrase = "DELETE ALL BODY DATA"

var ErrPrivacyErasureConfirmation = errors.New("privacy erasure confirmation phrase does not match")

type PrivacyErasurePlan struct {
	Destructive        bool                          `json:"destructive"`
	ConfirmationPhrase string                        `json:"confirmation_phrase"`
	Counts             []repository.PrivacyDataCount `json:"counts"`
	RetainedAudit      []string                      `json:"retained_audit"`
}

type privacyErasureStore interface {
	DeleteRestrictBoundAggregates(ctx context.Context, userID uuid.UUID) error
	CountUserData(ctx context.Context, userID uuid.UUID) ([]repository.PrivacyDataCount, error)
	CreateOrGet(ctx context.Context, userID uuid.UUID, subjectDigest string, report datatypes.JSON) (*model.PrivacyErasureRequest, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.PrivacyErasureRequest, error)
	TryClaim(ctx context.Context, id uuid.UUID, owner string, lease time.Duration) (*model.PrivacyErasureRequest, bool, error)
	MarkRetryable(ctx context.Context, id uuid.UUID, failure string) error
	MarkCompleted(ctx context.Context, id uuid.UUID) error
	ListRecoverable(ctx context.Context, limit int) ([]uuid.UUID, error)
}

type userPrivacyEraser interface {
	DeleteByID(ctx context.Context, id uuid.UUID) error
}

type authPrivacyRevoker interface {
	RevokeUser(ctx context.Context, userID uuid.UUID) error
}

// UserObjectCleaner is the privacy-facing storage port. BS-PROD-013 swaps the
// local implementation for durable object storage without changing erasure orchestration.
type UserObjectCleaner interface {
	EraseUserObjects(ctx context.Context, userID uuid.UUID) error
}

type privacyTransactionManager interface {
	WithinTransaction(ctx context.Context, fn func(context.Context) error) error
}

// LocalUserObjectCleaner removes the entire per-user upload prefix. Removing
// the prefix rather than only DB-listed files also cleans abandoned blobs from
// earlier partial upload failures.
type LocalUserObjectCleaner struct {
	root string
}

func NewLocalUserObjectCleaner(root string) *LocalUserObjectCleaner {
	return &LocalUserObjectCleaner{root: root}
}

func (c *LocalUserObjectCleaner) EraseUserObjects(_ context.Context, userID uuid.UUID) error {
	rootAbs, err := filepath.Abs(c.root)
	if err != nil {
		return fmt.Errorf("resolve upload root: %w", err)
	}
	userRoot := filepath.Join(rootAbs, userID.String())
	rel, err := filepath.Rel(rootAbs, userRoot)
	if err != nil || rel == "." || rel == ".." || filepath.IsAbs(rel) {
		return fmt.Errorf("refuse unsafe user upload prefix %q", userRoot)
	}
	if err := os.RemoveAll(userRoot); err != nil {
		return fmt.Errorf("erase user upload prefix: %w", err)
	}
	return nil
}

// PrivacyErasureService is the only service-level operation allowed to perform
// a full account/health-data erasure. Conversation deletion remains separate.
type PrivacyErasureService struct {
	requests      privacyErasureStore
	users         userPrivacyEraser
	auth          authPrivacyRevoker
	objects       UserObjectCleaner
	transactions  privacyTransactionManager
	workerID      string
	leaseDuration time.Duration
}

func NewPrivacyErasureService(
	requests *repository.PrivacyErasureRepository,
	users *repository.UserRepository,
	authService *AuthService,
	objects UserObjectCleaner,
	transactions *database.TransactionManager,
) *PrivacyErasureService {
	hostname, _ := os.Hostname()
	return &PrivacyErasureService{
		requests:      requests,
		users:         users,
		auth:          authService,
		objects:       objects,
		transactions:  transactions,
		workerID:      fmt.Sprintf("%s-%s", hostname, uuid.NewString()[:8]),
		leaseDuration: 5 * time.Minute,
	}
}

func privacySubjectDigest(userID uuid.UUID) string {
	sum := sha256.Sum256([]byte("bodysense/privacy-erasure/v1:" + userID.String()))
	return fmt.Sprintf("%x", sum[:])
}

func (s *PrivacyErasureService) Plan(ctx context.Context, userID uuid.UUID) (*PrivacyErasurePlan, error) {
	counts, err := s.requests.CountUserData(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &PrivacyErasurePlan{
		Destructive:        true,
		ConfirmationPhrase: PrivacyErasureConfirmationPhrase,
		Counts:             counts,
		RetainedAudit: []string{
			"anonymous erasure request status/timestamps",
			"global knowledge publications not derived from this user",
		},
	}, nil
}

// Request accepts the irreversible user intent, persists the request first,
// attempts execution immediately, and leaves any partial failure recoverable by
// the background worker. A persisted request is therefore never lost because
// an HTTP connection disappears.
func (s *PrivacyErasureService) Request(ctx context.Context, userID uuid.UUID, confirmation string) (*model.PrivacyErasureRequest, error) {
	if confirmation != PrivacyErasureConfirmationPhrase {
		return nil, ErrPrivacyErasureConfirmation
	}
	plan, err := s.Plan(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("build privacy erasure plan: %w", err)
	}
	report, err := json.Marshal(plan)
	if err != nil {
		return nil, fmt.Errorf("encode privacy erasure report: %w", err)
	}
	request, err := s.requests.CreateOrGet(ctx, userID, privacySubjectDigest(userID), datatypes.JSON(report))
	if err != nil {
		return nil, err
	}
	if request.Status != "completed" {
		if err := s.ProcessRequest(ctx, request.ID); err != nil {
			log.Printf("privacy erasure request %s queued for retry: %v", request.ID, err)
		}
	}
	return s.requests.GetByID(ctx, request.ID)
}

func (s *PrivacyErasureService) ProcessRequest(ctx context.Context, id uuid.UUID) error {
	request, claimed, err := s.requests.TryClaim(ctx, id, s.workerID, s.leaseDuration)
	if err != nil {
		return err
	}
	if !claimed || request == nil {
		return nil
	}
	if request.SubjectUserID == nil {
		return s.requests.MarkCompleted(ctx, id)
	}
	userID := *request.SubjectUserID

	fail := func(stage string, err error) error {
		wrapped := fmt.Errorf("%s: %w", stage, err)
		if markErr := s.requests.MarkRetryable(ctx, id, wrapped.Error()); markErr != nil {
			return fmt.Errorf("%v; mark retryable: %w", wrapped, markErr)
		}
		return wrapped
	}

	// Revoke first. The Redis user tombstone prevents any concurrent login or
	// refresh from re-arming a session after this point.
	if err := s.auth.RevokeUser(ctx, userID); err != nil {
		return fail("revoke authentication", err)
	}
	// Physical blobs are removed before their DB manifest rows cascade. A retry
	// treats an already-absent prefix as success.
	if err := s.objects.EraseUserObjects(ctx, userID); err != nil {
		return fail("erase upload objects", err)
	}
	if s.transactions == nil {
		return fail("delete database subject", errors.New("transaction manager is not configured"))
	}
	if err := s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		// Preserve the normal TreatmentRevision -> Diagnosis RESTRICT contract.
		// Only this privileged erasure transaction may remove the Treatment
		// aggregate first so the subsequent user cascade can erase Diagnosis.
		if err := s.requests.DeleteRestrictBoundAggregates(txCtx, userID); err != nil {
			return err
		}
		return s.users.DeleteByID(txCtx, userID)
	}); err != nil {
		return fail("delete database subject", err)
	}
	if err := s.requests.MarkCompleted(ctx, id); err != nil {
		return fmt.Errorf("mark privacy erasure completed: %w", err)
	}
	return nil
}

func (s *PrivacyErasureService) Recover(ctx context.Context, limit int) (int, error) {
	ids, err := s.requests.ListRecoverable(ctx, limit)
	if err != nil {
		return 0, err
	}
	processed := 0
	for _, id := range ids {
		if err := s.ProcessRequest(ctx, id); err != nil {
			log.Printf("privacy erasure recovery %s: %v", id, err)
		}
		processed++
	}
	return processed, nil
}

func (s *PrivacyErasureService) StartWorker(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			if _, err := s.Recover(ctx, 10); err != nil {
				log.Printf("privacy erasure recovery failed: %v", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}
