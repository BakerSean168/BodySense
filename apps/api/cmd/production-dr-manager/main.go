package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/bodysense/api/internal/dr"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: production-dr-manager <backup|status|restore-drill>")
	}
	cfg, err := dr.ConfigFromEnv()
	if err != nil {
		log.Fatalf("configure production DR: %v", err)
	}
	store, err := dr.NewStore(cfg)
	if err != nil {
		log.Fatalf("configure object store: %v", err)
	}
	manager := dr.NewManager(cfg, store)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()

	var result any
	switch os.Args[1] {
	case "backup":
		result, err = manager.Backup(ctx)
	case "status":
		result, err = manager.Status(ctx)
	case "restore-drill":
		result, err = manager.RestoreDrill(ctx)
	default:
		log.Fatalf("unknown production DR command %q", os.Args[1])
	}
	if err != nil {
		log.Fatalf("production DR %s: %v", os.Args[1], err)
	}
	payload, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatalf("encode DR result: %v", err)
	}
	fmt.Println(string(payload))
}
