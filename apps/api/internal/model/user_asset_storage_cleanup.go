package model

import "github.com/google/uuid"

// UserAssetStorageCleanup records every controlled source replica that may
// require cleanup, including copy intent before Put and abandoned targets.
// Rows survive failed IO and are removed only after Exists confirms absence.
// This source-only inventory is not an AS-002 verified-erasure receipt.
type UserAssetStorageCleanup struct {
	AssetID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	StorageBackend string    `gorm:"primaryKey"`
	StorageKey     string    `gorm:"primaryKey"`
}

func (UserAssetStorageCleanup) TableName() string { return "user_asset_storage_cleanups" }
