package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AssetErasureRepository owns the per-asset erasure request, its target
// receipts and the database-resident controlled copies of one asset. Object
// storage and runtime stores are executed by the service through their ports.
type AssetErasureRepository struct {
	db  *gorm.DB
	now func() time.Time
}

func NewAssetErasureRepository(db *gorm.DB) *AssetErasureRepository {
	return &AssetErasureRepository{db: db, now: func() time.Time { return time.Now().UTC() }}
}

func (r *AssetErasureRepository) conn(ctx context.Context) *gorm.DB {
	return database.FromContext(ctx, r.db)
}

// CreateOrGet admits one erasure per owned asset. A repeated command for the
// same asset returns the existing request whatever its key; a key already
// used for another asset is rejected. Missing and foreign assets are absent.
func (r *AssetErasureRepository) CreateOrGet(ctx context.Context, userID, assetID uuid.UUID, idempotencyKey string) (*model.AssetErasureReceipt, error) {
	var requestID uuid.UUID
	err := r.conn(ctx).Transaction(func(tx *gorm.DB) error {
		var asset model.UserAsset
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND id = ?", userID, assetID).Take(&asset).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return model.ErrAssetErasureNotFound
			}
			return err
		}
		var byKey model.AssetErasureRequest
		err := tx.Where("user_id = ? AND idempotency_key = ?", userID, idempotencyKey).Take(&byKey).Error
		if err == nil && byKey.AssetID != assetID {
			return model.ErrAssetErasureIdempotencyUsed
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var existing model.AssetErasureRequest
		err = tx.Where("user_id = ? AND asset_id = ?", userID, assetID).Take(&existing).Error
		if err == nil {
			requestID = existing.ID
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		now := r.now()
		request := model.AssetErasureRequest{
			ID: uuid.New(), UserID: userID, AssetID: assetID, IdempotencyKey: idempotencyKey,
			InventoryRevision: model.AssetErasureInventoryRevision, Status: model.AssetErasurePending,
			RequestedAt: now, UpdatedAt: now,
		}
		if err := tx.Create(&request).Error; err != nil {
			return err
		}
		targets := make([]model.AssetErasureTarget, 0, len(model.AssetErasureTargets))
		for _, name := range model.AssetErasureTargets {
			targets = append(targets, model.AssetErasureTarget{
				UserID: userID, RequestID: request.ID, Target: name,
				State: model.AssetErasureTargetPending, UpdatedAt: now,
			})
		}
		if err := tx.Create(&targets).Error; err != nil {
			return err
		}
		requestID = request.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.Receipt(ctx, userID, requestID)
}

// GetByAsset returns the owner's receipt for one asset.
func (r *AssetErasureRepository) GetByAsset(ctx context.Context, userID, assetID uuid.UUID) (*model.AssetErasureReceipt, error) {
	var request model.AssetErasureRequest
	if err := r.conn(ctx).Where("user_id = ? AND asset_id = ?", userID, assetID).Take(&request).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.ErrAssetErasureNotFound
		}
		return nil, err
	}
	return r.Receipt(ctx, userID, request.ID)
}

// Receipt loads one request with its targets in inventory order.
func (r *AssetErasureRepository) Receipt(ctx context.Context, userID, requestID uuid.UUID) (*model.AssetErasureReceipt, error) {
	var request model.AssetErasureRequest
	if err := r.conn(ctx).Where("user_id = ? AND id = ?", userID, requestID).Take(&request).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.ErrAssetErasureNotFound
		}
		return nil, err
	}
	var rows []model.AssetErasureTarget
	if err := r.conn(ctx).Where("user_id = ? AND request_id = ?", userID, requestID).Find(&rows).Error; err != nil {
		return nil, err
	}
	byName := make(map[model.AssetErasureTargetName]model.AssetErasureTarget, len(rows))
	for _, row := range rows {
		byName[row.Target] = row
	}
	receipt := &model.AssetErasureReceipt{Request: request, Targets: make([]model.AssetErasureTarget, 0, len(rows))}
	for _, name := range model.AssetErasureTargets {
		if row, ok := byName[name]; ok {
			receipt.Targets = append(receipt.Targets, row)
		}
	}
	conditions, err := assetPinningConditions(r.conn(ctx), userID, request.AssetID)
	if err != nil {
		return nil, err
	}
	if receipt.Reviews, err = dependentReviews(r.conn(ctx), userID, conditions, erasureBodyRefs{}); err != nil {
		return nil, err
	}
	return receipt, nil
}

// Claim leases a request for one execution pass. Erased requests and live
// leases are never claimed, so two workers never execute the same request.
func (r *AssetErasureRepository) Claim(ctx context.Context, requestID uuid.UUID, owner string, lease time.Duration) (*model.AssetErasureRequest, bool, error) {
	now := r.now()
	var request model.AssetErasureRequest
	result := r.conn(ctx).Raw(`
UPDATE asset_erasure_requests
SET status = 'running', attempt_count = attempt_count + 1,
    lease_owner = ?, lease_expires_at = ?, updated_at = ?
WHERE id = ?
  AND (status IN ('pending','outstanding') OR (status = 'running' AND lease_expires_at < ?))
RETURNING *`, owner, now.Add(lease), now, requestID, now).Scan(&request)
	if result.Error != nil {
		return nil, false, fmt.Errorf("claim asset erasure: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, false, nil
	}
	return &request, true, nil
}

// Release ends a pass that did not erase everything.
func (r *AssetErasureRepository) Release(ctx context.Context, request *model.AssetErasureRequest) error {
	return r.conn(ctx).Exec(`UPDATE asset_erasure_requests
SET status = 'outstanding', lease_owner = NULL, lease_expires_at = NULL, updated_at = ?
WHERE user_id = ? AND id = ? AND status = 'running' AND lease_owner = ?`,
		r.now(), request.UserID, request.ID, *request.LeaseOwner).Error
}

// RuntimeScope lists what the runtime_checkpoints target hands the
// ai-service: every Conversation of the owner (thread id = Conversation id),
// those a message attached the asset to, and the asset's content hash.
// Messages keep only the upload_id pointer; they are conversation records,
// not copies, so they never block erasure.
func (r *AssetErasureRepository) RuntimeScope(ctx context.Context, userID, assetID uuid.UUID) (model.AssetRuntimeScope, error) {
	var scope model.AssetRuntimeScope
	var rows []struct {
		ID       uuid.UUID
		Attached bool
	}
	if err := r.conn(ctx).Raw(`SELECT c.id, EXISTS (
    SELECT 1 FROM messages m
    WHERE m.conversation_id = c.id
      AND m.parts @> jsonb_build_array(jsonb_build_object('upload_id', ?::text))) AS attached
 FROM conversations c WHERE c.user_id = ? ORDER BY c.id`, assetID.String(), userID).Scan(&rows).Error; err != nil {
		return scope, fmt.Errorf("asset runtime scope: %w", err)
	}
	for _, row := range rows {
		scope.ThreadIDs = append(scope.ThreadIDs, row.ID)
		if row.Attached {
			scope.AttachedThreadIDs = append(scope.AttachedThreadIDs, row.ID)
		}
	}
	if err := r.conn(ctx).Raw(`SELECT coalesce(content_sha256, '') FROM user_assets WHERE user_id = ? AND id = ?`,
		userID, assetID).Scan(&scope.ContentSHA256).Error; err != nil {
		return scope, fmt.Errorf("asset runtime scope: %w", err)
	}
	return scope, nil
}

// ListRecoverable returns requests a worker should run: new ones, expired
// leases, and outstanding ones with a transient failure older than retryAfter.
// Blocked and unsupported targets wait for a decision, a handler or an
// explicit repeat of the command.
func (r *AssetErasureRepository) ListRecoverable(ctx context.Context, retryAfter time.Duration, limit int) ([]uuid.UUID, error) {
	if limit <= 0 {
		limit = 10
	}
	now := r.now()
	var ids []uuid.UUID
	err := r.conn(ctx).Raw(`SELECT q.id FROM asset_erasure_requests q
WHERE q.status = 'pending'
   OR (q.status = 'running' AND q.lease_expires_at < ?)
   OR (q.status = 'outstanding' AND q.updated_at < ? AND EXISTS (
        SELECT 1 FROM asset_erasure_targets t
        WHERE t.user_id = q.user_id AND t.request_id = q.id AND t.state IN ('pending','failed')))
ORDER BY q.updated_at ASC
LIMIT ?`, now, now.Add(-retryAfter), limit).Scan(&ids).Error
	if err != nil {
		return nil, fmt.Errorf("list recoverable asset erasures: %w", err)
	}
	return ids, nil
}

// GetRequest loads a request by id for the worker.
func (r *AssetErasureRepository) GetRequest(ctx context.Context, requestID uuid.UUID) (*model.AssetErasureRequest, error) {
	var request model.AssetErasureRequest
	if err := r.conn(ctx).Where("id = ?", requestID).Take(&request).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.ErrAssetErasureNotFound
		}
		return nil, err
	}
	return &request, nil
}
