package database

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestDatabaseLoggerFiltersBoundParametersAtEveryLevel(t *testing.T) {
	for _, level := range []string{"info", "warn", "error", "silent"} {
		t.Run(level, func(t *testing.T) {
			t.Setenv("DB_LOG_LEVEL", level)
			var output bytes.Buffer
			logger := newDatabaseLogger(log.New(&output, "", 0))
			filter, ok := logger.(interface {
				ParamsFilter(context.Context, string, ...interface{}) (string, []interface{})
			})
			if !ok {
				t.Fatal("database logger does not implement parameter filtering")
			}
			query, parameters := filter.ParamsFilter(context.Background(), "INSERT INTO body_facts(statement) VALUES ($1)", "synthetic-private-health-value")
			if len(parameters) != 0 || strings.Contains(query, "synthetic-private-health-value") {
				t.Fatal("health parameters are exposed to SQL interpolation")
			}
		})
	}
}

func TestDatabaseLoggerDryRunDoesNotWriteHealthText(t *testing.T) {
	t.Setenv("DB_LOG_LEVEL", "info")
	var output bytes.Buffer
	db, err := gorm.Open(postgres.Open("host=127.0.0.1 port=1 user=unused dbname=unused sslmode=disable"), &gorm.Config{
		DryRun: true, DisableAutomaticPing: true, SkipDefaultTransaction: true,
		Logger: newDatabaseLogger(log.New(&output, "", 0)),
	})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	const privateStatement = "synthetic-private-health-value"
	if err := db.Exec("INSERT INTO body_facts(statement) VALUES (?)", privateStatement).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "INSERT INTO body_facts") {
		t.Fatal("test did not exercise a SQL trace")
	}
	if strings.Contains(output.String(), privateStatement) {
		t.Fatalf("SQL trace leaked bound health text: %s", output.String())
	}
}
