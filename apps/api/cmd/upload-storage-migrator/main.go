package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
	"github.com/bodysense/api/internal/uploadstorage"
)

func main() {
	from := flag.String("from", "local", "source upload storage backend")
	to := flag.String("to", "oss", "target upload storage backend")
	dryRun := flag.Bool("dry-run", false, "list migration scope without copying objects or updating manifests")
	flag.Parse()

	db, err := database.Connect(database.ConfigFromEnv())
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	registry, err := uploadstorage.NewRegistryFromEnv()
	if err != nil {
		log.Fatalf("configure upload storage: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	result, err := service.NewUploadStorageMigrator(repository.NewUploadRepository(db), registry).
		Migrate(ctx, *from, *to, *dryRun)
	if err != nil {
		log.Fatalf("migrate upload storage: %v", err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		log.Fatalf("encode migration result: %v", err)
	}
	fmt.Println(string(encoded))
}
