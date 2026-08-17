package database

import (
	"context"

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

// FromContext returns the transaction bound by TransactionManager, or fallback
// for ordinary repository calls outside a coordinated application transaction.
func FromContext(ctx context.Context, fallback *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(transactionContextKey{}).(*gorm.DB); ok && tx != nil {
		return tx.WithContext(ctx)
	}
	return fallback.WithContext(ctx)
}
