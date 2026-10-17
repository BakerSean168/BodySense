package repository

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type assetStorageConnectionKey struct{}

func (r *UserAssetRepository) assetDB(ctx context.Context) *gorm.DB {
	if conn, ok := ctx.Value(assetStorageConnectionKey{}).(*gorm.DB); ok {
		return conn.WithContext(ctx)
	}
	return database.FromContext(ctx, r.db)
}

// WithStorageLock serializes source copying/deletion across processes. A pinned
// connection holds a session advisory lock while individual ledger writes commit
// BEFORE external IO. A crashed process releases the lock but leaves its intent
// durable, so removal can find even a partially copied target. No transaction may
// enclose this boundary: rolling back intent after object IO would lose a replica.
func (r *UserAssetRepository) WithStorageLock(ctx context.Context, id uuid.UUID, fn func(context.Context) error) (err error) {
	if database.HasTransaction(ctx) {
		return errors.New("asset storage IO cannot run inside a database transaction")
	}
	pool, err := r.db.DB()
	if err != nil {
		return err
	}
	conn, err := pool.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	discard := func() { _ = conn.Raw(func(any) error { return driver.ErrBadConn }) }
	if _, err = conn.ExecContext(ctx, "SELECT pg_advisory_lock(hashtextextended($1, 0))", "user-asset-storage:"+id.String()); err != nil {
		discard()
		return err
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		var unlocked bool
		unlockErr := conn.QueryRowContext(unlockCtx, "SELECT pg_advisory_unlock(hashtextextended($1, 0))", "user-asset-storage:"+id.String()).Scan(&unlocked)
		if unlockErr != nil || !unlocked {
			discard()
			err = errors.Join(err, fmt.Errorf("release asset storage lock: unlocked=%v err=%v", unlocked, unlockErr))
		}
	}()
	scoped := r.db.Session(&gorm.Session{NewDB: true, Context: ctx})
	scoped.Statement.ConnPool = conn
	return fn(context.WithValue(ctx, assetStorageConnectionKey{}, scoped))
}

// TrackStorageReplica is a write-ahead record, not proof that the object exists.
// It must commit under WithStorageLock before any Put can create that replica.
func (r *UserAssetRepository) TrackStorageReplica(ctx context.Context, replica model.UserAssetStorageCleanup) error {
	return r.assetDB(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&replica).Error
}

func (r *UserAssetRepository) ListStorageReplicas(ctx context.Context, id, userID uuid.UUID) ([]model.UserAssetStorageCleanup, error) {
	var replicas []model.UserAssetStorageCleanup
	err := r.assetDB(ctx).Where("asset_id = ? AND user_id = ?", id, userID).Order("storage_backend, storage_key").Find(&replicas).Error
	return replicas, err
}

func (r *UserAssetRepository) ForgetStorageReplica(ctx context.Context, replica model.UserAssetStorageCleanup) error {
	return r.assetDB(ctx).Where("asset_id = ? AND user_id = ? AND storage_backend = ? AND storage_key = ?", replica.AssetID, replica.UserID, replica.StorageBackend, replica.StorageKey).Delete(&model.UserAssetStorageCleanup{}).Error
}
