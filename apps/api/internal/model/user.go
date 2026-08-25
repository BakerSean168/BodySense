package model

import (
	"time"

	"github.com/google/uuid"
)

// User roles are deliberately small: member is the normal product identity; operator
// may administer global Knowledge but does not gain implicit access to user health data.
const (
	UserRoleMember   = "member"
	UserRoleOperator = "operator"
)

// User represents a user in the system.
type User struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	Email        string    `gorm:"type:varchar(255);uniqueIndex;not null" json:"email"`
	PasswordHash string    `gorm:"type:varchar(255);not null" json:"-"`
	Role         string    `gorm:"type:varchar(30);not null;default:'member'" json:"role"`
	CreatedAt    time.Time `gorm:"not null;default:now()" json:"created_at"`
	// 为什么 LastLoginAt 是指针类型？因为 LastLoginAt 是可选的字段，可能没有值。如果使用非指针类型（如 time.Time），则在没有值时会被初始化为零值（即 0001-01-01 00:00:00 UTC），这可能会导致误解或错误。使用指针类型可以明确表示该字段是否有值，如果为 nil，则表示用户从未登录过。
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

// gorm 和 json 标签的作用是分别用于 GORM ORM 框架和 JSON 序列化/反序列化。gorm 标签用于指定数据库字段的类型、约束条件等，而 json 标签用于指定在 JSON 编码/解码时的字段名称和行为。

// TableName specifies the table name for GORM.
func (User) TableName() string {
	return "users"
}
