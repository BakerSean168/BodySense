package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type assetErasureStore interface {
	CreateOrGet(ctx context.Context, userID, assetID uuid.UUID, idempotencyKey string) (*model.AssetErasureReceipt, error)
	GetByAsset(ctx context.Context, userID, assetID uuid.UUID) (*model.AssetErasureReceipt, error)
	Receipt(ctx context.Context, userID, requestID uuid.UUID) (*model.AssetErasureReceipt, error)
	Claim(ctx context.Context, requestID uuid.UUID, owner string, lease time.Duration) (*model.AssetErasureRequest, bool, error)
	Release(ctx context.Context, request *model.AssetErasureRequest) error
	RecordOutcome(ctx context.Context, request *model.AssetErasureRequest, target model.AssetErasureTargetName, outcome model.AssetErasureOutcome) error
	ExecuteDatabaseTarget(ctx context.Context, request *model.AssetErasureRequest, target model.AssetErasureTargetName) (model.AssetErasureOutcome, error)
	RuntimeScope(ctx context.Context, userID, assetID uuid.UUID) (model.AssetRuntimeScope, error)
	Finalize(ctx context.Context, request *model.AssetErasureRequest) (bool, error)
	ListRecoverable(ctx context.Context, retryAfter time.Duration, limit int) ([]uuid.UUID, error)
}

// assetSourceEraser deletes and verifies every object-storage replica of one
// asset's private original (the source_objects target).
type assetSourceEraser interface {
	EraseSourceCopies(ctx context.Context, userID, assetID uuid.UUID) (model.AssetErasureOutcome, error)
}

// AssetRuntimeEraser erases one asset from the owner's consultation runtime
// state (LangGraph checkpoints) and returns a receipt read back from storage.
type AssetRuntimeEraser interface {
	EraseAssetRuntimeCopies(ctx context.Context, userID, assetID uuid.UUID, scope model.AssetRuntimeScope) (*model.RuntimeErasureReceipt, error)
}

// assetRuntimeErasureTimeout bounds one runtime erasure call inside a pass.
const assetRuntimeErasureTimeout = 2 * time.Minute

// AssetErasureService runs the AS-002 per-asset erasure: unlike source
// removal it goes after every inventoried controlled copy, verifies each
// target on its own and reports the asset erased only after all verified.
// Targets that need a Diagnosis/assessment decision or a runtime handler stay
// outstanding on the receipt; nothing here changes a Diagnosis or a plan.
type AssetErasureService struct {
	store      assetErasureStore
	sources    assetSourceEraser
	runtime    AssetRuntimeEraser
	workerID   string
	lease      time.Duration
	retryAfter time.Duration
}

func NewAssetErasureService(store assetErasureStore, sources assetSourceEraser) *AssetErasureService {
	hostname, _ := os.Hostname()
	return &AssetErasureService{
		store:      store,
		sources:    sources,
		workerID:   fmt.Sprintf("%s-%s", hostname, uuid.NewString()[:8]),
		lease:      5 * time.Minute,
		retryAfter: time.Minute,
	}
}

// WithRuntimeEraser wires the ai-service runtime erasure. Without it the
// runtime target of an owner who has conversations stays outstanding.
func (s *AssetErasureService) WithRuntimeEraser(runtime AssetRuntimeEraser) *AssetErasureService {
	s.runtime = runtime
	return s
}

// Request admits the erasure command (or repeats it) and runs one pass now.
// A failed pass is not an error to the caller: the receipt says what is
// outstanding and the worker or a repeated command retries.
func (s *AssetErasureService) Request(ctx context.Context, userID, assetID uuid.UUID, idempotencyKey string) (*model.AssetErasureReceipt, error) {
	receipt, err := s.store.CreateOrGet(ctx, userID, assetID, idempotencyKey)
	if err != nil {
		return nil, err
	}
	if receipt.Request.Status != model.AssetErasureErased {
		if err := s.Process(ctx, receipt.Request.ID); err != nil {
			log.Printf("asset erasure %s left outstanding: %v", receipt.Request.ID, err)
		}
	}
	return s.store.Receipt(ctx, userID, receipt.Request.ID)
}

// Receipt returns the owner's erasure receipt for one asset.
func (s *AssetErasureService) Receipt(ctx context.Context, userID, assetID uuid.UUID) (*model.AssetErasureReceipt, error) {
	return s.store.GetByAsset(ctx, userID, assetID)
}

// Process runs one leased pass over the whole inventory. Every target is
// attempted even when an earlier one fails, so the receipt always reflects
// the current state of each family.
func (s *AssetErasureService) Process(ctx context.Context, requestID uuid.UUID) error {
	request, claimed, err := s.store.Claim(ctx, requestID, s.workerID, s.lease)
	if err != nil || !claimed {
		return err
	}
	var passErr error
	for _, target := range model.AssetErasureTargets {
		if target == model.AssetErasureManifest {
			continue
		}
		var outcome model.AssetErasureOutcome
		var err error
		switch target {
		case model.AssetErasureSourceObjects:
			outcome, err = s.sources.EraseSourceCopies(ctx, request.UserID, request.AssetID)
		case model.AssetErasureRuntimeCheckpoints:
			outcome, err = s.eraseRuntimeCopies(ctx, request)
		default:
			outcome, err = s.store.ExecuteDatabaseTarget(ctx, request, target)
		}
		if err != nil {
			passErr = errors.Join(passErr, fmt.Errorf("%s: %w", target, err))
			outcome = model.AssetErasureOutcome{State: model.AssetErasureTargetFailed, FailureCode: model.AssetErasureCodeExecutionError}
		}
		if err := s.store.RecordOutcome(ctx, request, target, outcome); err != nil {
			passErr = errors.Join(passErr, err)
		}
	}
	erased, err := s.store.Finalize(ctx, request)
	if err != nil {
		passErr = errors.Join(passErr, fmt.Errorf("finalize: %w", err))
	}
	if !erased {
		if err := s.store.Release(context.WithoutCancel(ctx), request); err != nil {
			passErr = errors.Join(passErr, fmt.Errorf("release: %w", err))
		}
	}
	return passErr
}

// eraseRuntimeCopies is the runtime_checkpoints target: the ai-service
// erases the asset from every conversation of the owner and the target
// verifies only on a receipt that covers all of them.
func (s *AssetErasureService) eraseRuntimeCopies(ctx context.Context, request *model.AssetErasureRequest) (model.AssetErasureOutcome, error) {
	scope, err := s.store.RuntimeScope(ctx, request.UserID, request.AssetID)
	if err != nil {
		return model.AssetErasureOutcome{}, err
	}
	if len(scope.ThreadIDs) == 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetVerified}, nil
	}
	if len(scope.ContentSHA256) != 64 {
		return model.AssetErasureOutcome{}, errors.New("asset content hash is missing")
	}
	unavailable := model.AssetErasureOutcome{State: model.AssetErasureTargetFailed, Outstanding: len(scope.ThreadIDs), FailureCode: model.AssetErasureCodeRuntimeUnavailable}
	if s.runtime == nil {
		return unavailable, nil
	}
	callCtx, cancel := context.WithTimeout(ctx, assetRuntimeErasureTimeout)
	defer cancel()
	receipt, err := s.runtime.EraseAssetRuntimeCopies(callCtx, request.UserID, request.AssetID, scope)
	if err != nil {
		log.Printf("asset erasure %s runtime copies: %v", request.ID, err)
		return unavailable, nil
	}
	return scope.Outcome(request.AssetID, receipt), nil
}

// Recover retries recoverable requests once.
func (s *AssetErasureService) Recover(ctx context.Context, limit int) (int, error) {
	ids, err := s.store.ListRecoverable(ctx, s.retryAfter, limit)
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		if err := s.Process(ctx, id); err != nil {
			log.Printf("asset erasure recovery %s: %v", id, err)
		}
	}
	return len(ids), nil
}

// StartWorker periodically recovers interrupted and transiently failed passes.
func (s *AssetErasureService) StartWorker(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			if _, err := s.Recover(ctx, 10); err != nil {
				log.Printf("asset erasure recovery failed: %v", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}
