package repository

import (
	"context"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
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

// Create creates a new user.
func (r *UserRepository) Create(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

// FindByEmail finds a user by email.
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
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

// UpdateLastLoginAt updates the user's last login timestamp.
func (r *UserRepository) UpdateLastLoginAt(ctx context.Context, userID uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Update("last_login_at", now).Error
}

// EmailExists checks if an email already exists.
func (r *UserRepository) EmailExists(ctx context.Context, email string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.User{}).Where("email = ?", email).Count(&count).Error
	return count > 0, err
}

// DeleteByID deletes a user by ID.
// WARNING: Callers must also invalidate the user's session cache.
func (r *UserRepository) DeleteByID(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.User{}).Error
}
