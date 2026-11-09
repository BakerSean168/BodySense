package repository

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 把“数据库操作”集中封装起来，让上层业务代码不要直接到处调用 GORM。
// 保存一个数据库访问工具，然后提供一堆和 User 有关的方法。
// UserRepository handles user database operations.
type UserRepository struct {
	// 传入一个数据库操作对象 db *gorm.DB，UserRepository 将使用它来执行数据库操作。这样可以实现依赖注入，使得 UserRepository 更加灵活和可测试。
	// 这里的 * 是为了表示 db 是一个指针类型，指向 gorm.DB 的实例。gorm.DB 是 GORM 框架中用于与数据库交互的主要对象，它封装了数据库连接和操作方法。
	// 数据库操作入口 / database handle
	db *gorm.DB
}

// NewUserRepository creates a new UserRepository.
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// FindByID finds a user by ID.
func (r *UserRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// IsClosed checks the permanent anonymized erasure tombstone. A Supabase JWT
// issued before account deletion must never recreate the local health identity.
func (r *UserRepository) IsClosed(ctx context.Context, id uuid.UUID) (bool, error) {
	digest := sha256.Sum256([]byte("bodysense/privacy-erasure/v1:" + id.String()))
	var count int64
	err := r.db.WithContext(ctx).Table("privacy_erasure_requests").
		Where("subject_digest = ?", fmt.Sprintf("%x", digest[:])).
		Count(&count).Error
	return count > 0, err
}

// EnsureIdentity materializes a Supabase-authenticated user ID as the local
// business principal. The unique primary key makes concurrent first requests
// safe. A prior privacy erasure permanently prevents re-provisioning.
func (r *UserRepository) EnsureIdentity(ctx context.Context, id uuid.UUID, email string) (*model.User, error) {
	closed, err := r.IsClosed(ctx, id)
	if err != nil {
		return nil, err
	}
	if closed {
		return nil, fmt.Errorf("account has been closed")
	}

	record := &model.User{ID: id, Email: email, Role: model.UserRoleMember}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).Create(record).Error; err != nil {
		return nil, fmt.Errorf("provision user: %w", err)
	}
	user, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Protect against an erasure request racing with account provisioning.
	closed, err = r.IsClosed(ctx, id)
	if err != nil {
		return nil, err
	}
	if closed {
		return nil, fmt.Errorf("account has been closed")
	}
	return user, nil
}

// DeleteByID deletes a user by ID. Supabase identity deletion and the local
// erasure tombstone are managed by the durable privacy-erasure worker.
func (r *UserRepository) DeleteByID(ctx context.Context, id uuid.UUID) error {
	return database.FromContext(ctx, r.db).Where("id = ?", id).Delete(&model.User{}).Error
}
