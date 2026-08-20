package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
)

func main() {
	champion := flag.String("champion", "treat-config-85718f8e90ac9d80", "Champion configuration ID")
	challenger := flag.String("challenger", "treat-config-f68eec9846664596", "Challenger configuration ID")
	stage := flag.String("stage", service.TreatmentRolloutShadow, "Rollout stage to summarize")
	canaryBPS := flag.Int("canary-bps", 500, "Canary basis-point step to summarize")
	limit := flag.Int("limit", 1000, "Maximum recent observations")
	flag.Parse()

	db, err := database.Connect(database.ConfigFromEnv())
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	rollout := service.NewTreatmentRolloutService(repository.NewTreatmentRolloutRepository(db), nil)
	summary, err := rollout.Summary(context.Background(), *champion, *challenger, *stage, *canaryBPS, *limit)
	if err != nil {
		log.Fatalf("summarize Treatment rollout: %v", err)
	}
	gate := service.EvaluateTreatmentRolloutGate(summary)
	progression := service.EvaluateTreatmentRolloutProgression(*stage, *canaryBPS, summary)
	encoded, _ := json.MarshalIndent(map[string]any{
		"summary":     summary,
		"gate":        gate,
		"progression": progression,
	}, "", "  ")
	fmt.Println(string(encoded))
	if gate.Action != "continue" {
		log.Fatalf("TREATMENT_ROLLOUT_GATE=%s", gate.Action)
	}
	fmt.Println("TREATMENT_ROLLOUT_GATE=continue")
}
