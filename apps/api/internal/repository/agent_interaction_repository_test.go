package repository

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupAgentInteractionRepo(t *testing.T) (*AgentInteractionRepository, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	return NewAgentInteractionRepository(gormDB), mock, func() { _ = sqlDB.Close() }
}

// TestAgentInteractionRepositoryAggregateScopesToOwner ensures the metrics query
// always joins through conversations and filters by the owning user, so a caller
// can never read another user's interaction data.
func TestAgentInteractionRepositoryAggregateScopesToOwner(t *testing.T) {
	repo, mock, cleanup := setupAgentInteractionRepo(t)
	defer cleanup()

	userID := uuid.New()
	conversationID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT agent_interactions.status, count(*) as count FROM "agent_interactions" JOIN conversations ON conversations.id = agent_interactions.conversation_id WHERE (conversations.user_id = $1 AND conversations.deleted_at IS NULL) AND agent_interactions.conversation_id = $2 `) + `GROUP BY "agent_interactions"\."status"`).
		WithArgs(userID, conversationID).
		WillReturnRows(sqlmock.NewRows([]string{"status", "count"}).
			AddRow("answered", 3).
			AddRow("expired", 1).
			AddRow("pending", 2))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COALESCE(AVG(EXTRACT(EPOCH FROM (agent_interactions.answered_at - agent_interactions.created_at))), 0) as avg FROM "agent_interactions" JOIN conversations ON conversations.id = agent_interactions.conversation_id WHERE (conversations.user_id = $1 AND conversations.deleted_at IS NULL) AND (agent_interactions.status = $2 AND agent_interactions.answered_at IS NOT NULL) AND agent_interactions.conversation_id = $3`)).
		WithArgs(userID, "answered", conversationID).
		WillReturnRows(sqlmock.NewRows([]string{"avg"}).AddRow(12.5))

	answered, expired, pending, avgWait, err := repo.AggregateInteractionMetrics(context.Background(), userID, &conversationID)
	if err != nil {
		t.Fatalf("AggregateInteractionMetrics: %v", err)
	}
	if answered != 3 || expired != 1 || pending != 2 {
		t.Fatalf("counts = %d/%d/%d, want 3/1/2", answered, expired, pending)
	}
	if avgWait != 12.5 {
		t.Fatalf("avgWait = %v, want 12.5", avgWait)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestAgentInteractionRepositoryAggregateWithoutConversationStillScopesToOwner
// ensures the global aggregate remains user-scoped (no conversation filter).
func TestAgentInteractionRepositoryAggregateWithoutConversationStillScopesToOwner(t *testing.T) {
	repo, mock, cleanup := setupAgentInteractionRepo(t)
	defer cleanup()

	userID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT agent_interactions.status, count(*) as count FROM "agent_interactions" JOIN conversations ON conversations.id = agent_interactions.conversation_id WHERE conversations.user_id = $1 AND conversations.deleted_at IS NULL `) + `GROUP BY "agent_interactions"\."status"`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"status", "count"}).AddRow("answered", 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COALESCE(AVG(EXTRACT(EPOCH FROM (agent_interactions.answered_at - agent_interactions.created_at))), 0) as avg FROM "agent_interactions" JOIN conversations ON conversations.id = agent_interactions.conversation_id WHERE (conversations.user_id = $1 AND conversations.deleted_at IS NULL) AND (agent_interactions.status = $2 AND agent_interactions.answered_at IS NOT NULL)`)).
		WithArgs(userID, "answered").
		WillReturnRows(sqlmock.NewRows([]string{"avg"}).AddRow(0))

	answered, _, _, _, err := repo.AggregateInteractionMetrics(context.Background(), userID, nil)
	if err != nil {
		t.Fatalf("AggregateInteractionMetrics: %v", err)
	}
	if answered != 1 {
		t.Fatalf("answered = %d, want 1", answered)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
