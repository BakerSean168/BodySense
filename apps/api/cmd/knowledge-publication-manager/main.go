package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: knowledge-publication-manager <publish|rollback> [flags]")
	}

	db, err := database.Connect(database.ConfigFromEnv())
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	publicationService := service.NewKnowledgePublicationService(
		repository.NewKnowledgePublicationRepository(db),
		database.NewTransactionManager(db),
	)

	ctx := context.Background()
	switch os.Args[1] {
	case "publish":
		publish(ctx, publicationService, os.Args[2:])
	case "rollback":
		rollback(ctx, publicationService, os.Args[2:])
	default:
		log.Fatalf("unknown command %q", os.Args[1])
	}
}

func publish(
	ctx context.Context,
	publicationService *service.KnowledgePublicationService,
	args []string,
) {
	flags := flag.NewFlagSet("publish", flag.ExitOnError)
	publicationKey := flags.String("publication-key", "", "Immutable publication key")
	batchKey := flags.String("batch-key", "", "Publication batch family key")
	unitKeysCSV := flags.String("unit-keys", "", "Comma-separated knowledge unit keys")
	publishedBy := flags.String("published-by", "", "Operator identity")
	summary := flags.String("summary", "", "Publication summary")
	_ = flags.Parse(args)

	publication, err := publicationService.PublishBatch(ctx, service.PublishKnowledgeBatchInput{
		PublicationKey: *publicationKey,
		BatchKey:       *batchKey,
		UnitKeys:       splitCSV(*unitKeysCSV),
		PublishedBy:    *publishedBy,
		Summary:        *summary,
	})
	if err != nil {
		log.Fatalf("publish knowledge batch: %v", err)
	}
	printJSON(publication)
}

func rollback(
	ctx context.Context,
	publicationService *service.KnowledgePublicationService,
	args []string,
) {
	flags := flag.NewFlagSet("rollback", flag.ExitOnError)
	publicationKey := flags.String("publication-key", "", "Published batch key to rollback")
	rollbackKey := flags.String("rollback-key", "", "Immutable rollback publication key")
	rolledBackBy := flags.String("rolled-back-by", "", "Operator identity")
	reason := flags.String("reason", "", "Rollback reason")
	_ = flags.Parse(args)

	publication, err := publicationService.RollbackBatch(ctx, service.RollbackKnowledgeBatchInput{
		PublicationKey:         *publicationKey,
		RollbackPublicationKey: *rollbackKey,
		RolledBackBy:           *rolledBackBy,
		Reason:                 *reason,
	})
	if err != nil {
		log.Fatalf("rollback knowledge batch: %v", err)
	}
	printJSON(publication)
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func printJSON(value any) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		log.Fatalf("encode result: %v", err)
	}
	fmt.Println(string(encoded))
}
