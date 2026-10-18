package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type statementErasureStore interface {
	CreateOrGet(ctx context.Context, userID, statementID uuid.UUID, idempotencyKey string) (*model.StatementErasureReceipt, error)
	GetByStatement(ctx context.Context, userID, statementID uuid.UUID) (*model.StatementErasureReceipt, error)
	Receipt(ctx context.Context, userID, requestID uuid.UUID) (*model.StatementErasureReceipt, error)
	Claim(ctx context.Context, requestID uuid.UUID, owner string, lease time.Duration) (*model.StatementErasureRequest, bool, error)
	Release(ctx context.Context, request *model.StatementErasureRequest) error
	RecordOutcome(ctx context.Context, request *model.StatementErasureRequest, target model.StatementErasureTargetName, outcome model.AssetErasureOutcome) error
	ExecuteDatabaseTarget(ctx context.Context, request *model.StatementErasureRequest, target model.StatementErasureTargetName) (model.AssetErasureOutcome, error)
	RuntimeScope(ctx context.Context, userID, statementID uuid.UUID) (model.StatementRuntimeScope, error)
	Finalize(ctx context.Context, request *model.StatementErasureRequest) (bool, error)
	ListRecoverable(ctx context.Context, retryAfter time.Duration, limit int) ([]uuid.UUID, error)
}

// StatementRuntimeEraser erases one statement and its derived record ids
// from the owner's consultation runtime state and returns a receipt read
// back from storage.
type StatementRuntimeEraser interface {
	EraseStatementRuntimeCopies(ctx context.Context, userID, statementID uuid.UUID, scope model.StatementRuntimeScope) (*model.RuntimeErasureReceipt, error)
}

// StatementErasureService runs the AS-002 per-statement erasure: the user's
// statement and every BodyState record derived from it, their assessment
// copies and runtime state, each target verified on its own. What still
// rests on the statement (asserted records, adopted conditions, active
// plans) blocks with a review signal and is never changed here. Frozen
// replay inputs are retained by policy: the request then ends
// erased_with_retention, never erased.
type StatementErasureService struct {
	store      statementErasureStore
	runtime    StatementRuntimeEraser
	workerID   string
	lease      time.Duration
	retryAfter time.Duration
}

func NewStatementErasureService(store statementErasureStore) *StatementErasureService {
	hostname, _ := os.Hostname()
	return &StatementErasureService{
		store:      store,
		workerID:   fmt.Sprintf("%s-%s", hostname, uuid.NewString()[:8]),
		lease:      5 * time.Minute,
		retryAfter: time.Minute,
	}
}

// WithRuntimeEraser wires the ai-service runtime erasure. Without it the
// runtime target of an owner who has conversations stays outstanding.
func (s *StatementErasureService) WithRuntimeEraser(runtime StatementRuntimeEraser) *StatementErasureService {
	s.runtime = runtime
	return s
}

// Request admits the erasure command (or repeats it) and runs one pass now.
// A repeated command after the user's review is what re-runs blocked targets.
func (s *StatementErasureService) Request(ctx context.Context, userID, statementID uuid.UUID, idempotencyKey string) (*model.StatementErasureReceipt, error) {
	receipt, err := s.store.CreateOrGet(ctx, userID, statementID, idempotencyKey)
	if err != nil {
		return nil, err
	}
	if !receipt.Request.Status.Terminal() {
		if err := s.Process(ctx, receipt.Request.ID); err != nil {
			log.Printf("statement erasure %s left outstanding: %v", receipt.Request.ID, err)
		}
	}
	return s.store.Receipt(ctx, userID, receipt.Request.ID)
}

// Receipt returns the owner's erasure receipt for one statement.
func (s *StatementErasureService) Receipt(ctx context.Context, userID, statementID uuid.UUID) (*model.StatementErasureReceipt, error) {
	return s.store.GetByStatement(ctx, userID, statementID)
}

// Process runs one leased pass over the whole inventory; every target is
// attempted even when an earlier one fails.
func (s *StatementErasureService) Process(ctx context.Context, requestID uuid.UUID) error {
	request, claimed, err := s.store.Claim(ctx, requestID, s.workerID, s.lease)
	if err != nil || !claimed {
		return err
	}
	var passErr error
	for _, target := range model.StatementErasureTargets {
		if target == model.StatementErasureManifest {
			continue
		}
		var outcome model.AssetErasureOutcome
		var err error
		if target == model.StatementErasureRuntimeCheckpoints {
			outcome, err = s.eraseRuntimeCopies(ctx, request)
		} else {
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
	terminal, err := s.store.Finalize(ctx, request)
	if err != nil {
		passErr = errors.Join(passErr, fmt.Errorf("finalize: %w", err))
	}
	if !terminal {
		if err := s.store.Release(context.WithoutCancel(ctx), request); err != nil {
			passErr = errors.Join(passErr, fmt.Errorf("release: %w", err))
		}
	}
	return passErr
}

func (s *StatementErasureService) eraseRuntimeCopies(ctx context.Context, request *model.StatementErasureRequest) (model.AssetErasureOutcome, error) {
	scope, err := s.store.RuntimeScope(ctx, request.UserID, request.StatementID)
	if err != nil {
		return model.AssetErasureOutcome{}, err
	}
	if len(scope.ThreadIDs) == 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetVerified}, nil
	}
	unavailable := model.AssetErasureOutcome{State: model.AssetErasureTargetFailed, Outstanding: len(scope.ThreadIDs), FailureCode: model.AssetErasureCodeRuntimeUnavailable}
	if s.runtime == nil {
		return unavailable, nil
	}
	callCtx, cancel := context.WithTimeout(ctx, assetRuntimeErasureTimeout)
	defer cancel()
	receipt, err := s.runtime.EraseStatementRuntimeCopies(callCtx, request.UserID, request.StatementID, scope)
	if err != nil {
		log.Printf("statement erasure %s runtime copies: %v", request.ID, err)
		return unavailable, nil
	}
	return model.RuntimeOutcome(scope.ThreadIDs, request.StatementID, receipt), nil
}

// Recover retries recoverable requests once.
func (s *StatementErasureService) Recover(ctx context.Context, limit int) (int, error) {
	ids, err := s.store.ListRecoverable(ctx, s.retryAfter, limit)
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		if err := s.Process(ctx, id); err != nil {
			log.Printf("statement erasure recovery %s: %v", id, err)
		}
	}
	return len(ids), nil
}

// StartWorker periodically recovers interrupted and transiently failed passes.
func (s *StatementErasureService) StartWorker(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			if _, err := s.Recover(ctx, 10); err != nil {
				log.Printf("statement erasure recovery failed: %v", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

type statementRuntimeErasureRequest struct {
	UserID            string   `json:"user_id"`
	ThreadIDs         []string `json:"thread_ids"`
	AttachedThreadIDs []string `json:"attached_thread_ids"`
	RecordIDs         []string `json:"record_ids"`
}

// EraseStatementRuntimeCopies asks the ai-service to erase one statement and
// its derived record ids from the owner's consultation threads
// (POST /runtime/statements/{statement}/erasure).
func (c *AIClient) EraseStatementRuntimeCopies(ctx context.Context, userID, statementID uuid.UUID, scope model.StatementRuntimeScope) (*model.RuntimeErasureReceipt, error) {
	raw, err := c.callJSON(ctx, "/runtime/statements/"+statementID.String()+"/erasure", statementRuntimeErasureRequest{
		UserID:            userID.String(),
		ThreadIDs:         uuidStrings(scope.ThreadIDs),
		AttachedThreadIDs: uuidStrings(scope.AttachedThreadIDs),
		RecordIDs:         uuidStrings(scope.RecordIDs),
	})
	if err != nil {
		return nil, err
	}
	var receipt model.RuntimeErasureReceipt
	if err := json.Unmarshal(raw, &receipt); err != nil {
		return nil, fmt.Errorf("decode statement runtime erasure receipt: %w", err)
	}
	return &receipt, nil
}
