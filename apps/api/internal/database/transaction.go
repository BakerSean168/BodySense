package database

import (
	"context"
	"database/sql"

	"gorm.io/gorm"
)

type transactionContextKey struct{}

// TransactionManager owns synchronous database transaction boundaries shared
// across application services and multiple repositories.
type TransactionManager struct {
	db *gorm.DB
}

func NewTransactionManager(db *gorm.DB) *TransactionManager {
	return &TransactionManager{db: db}
}

// WithinTransaction executes fn against one GORM transaction. Repositories use
// FromContext to participate in this transaction instead of opening unrelated
// commits for each aggregate mutation.
func (m *TransactionManager) WithinTransaction(
	ctx context.Context,
	fn func(context.Context) error,
) error {
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, transactionContextKey{}, tx))
	})
}

// WithinReadSnapshot executes fn in one read-only REPEATABLE READ transaction,
// so reads across several repositories observe one committed database
// snapshot (one BodyState revision). Repository transactions opened inside fn
// become savepoints of this snapshot.
func (m *TransactionManager) WithinReadSnapshot(
	ctx context.Context,
	fn func(context.Context) error,
) error {
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, transactionContextKey{}, tx))
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
}

// FromContext returns the transaction bound by TransactionManager, or fallback
// for ordinary repository calls outside a coordinated application transaction.
func FromContext(ctx context.Context, fallback *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(transactionContextKey{}).(*gorm.DB); ok && tx != nil {
		return tx.WithContext(ctx)
	}
	return fallback.WithContext(ctx)
}

// HasTransaction reports whether application work is inside a coordinated transaction.
func HasTransaction(ctx context.Context) bool {
	tx, ok := ctx.Value(transactionContextKey{}).(*gorm.DB)
	return ok && tx != nil
}
