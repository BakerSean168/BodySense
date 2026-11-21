package main

import (
	"context"
	"log"
	"time"

	"github.com/bodysense/api/internal/service"
)

// startRunLeaseReconciler reclaims runs whose lease expired and lets retention
// catch up past each lost Run.
func startRunLeaseReconciler(
	runService *service.RunService,
	conversationService *service.ConversationService,
	runtimeEventService *service.RuntimeEventService,
	chatRetention *service.ChatRetentionService,
) {
	reconcile := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		runs, err := runService.ReconcileExpiredRuns(ctx, 100)
		if err != nil {
			log.Printf("run lease reconciliation failed: %v", err)
			return
		}
		for i := range runs {
			run := &runs[i]
			failedMessage, err := conversationService.FinalizeExecutionLostProjection(ctx, run)
			if err != nil {
				log.Printf("finalize execution-lost projection for run %s: %v", run.ID, err)
			}
			if err := runtimeEventService.RecordMessageExecutionLost(ctx, run, failedMessage); err != nil {
				log.Printf("record execution-lost message event for run %s: %v", run.ID, err)
			}
			// A lost Run is settled too; let retention catch up past it.
			chatRetention.OnRunSettled(ctx, run.UserID)
		}
	}

	go func() {
		reconcile()
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			reconcile()
		}
	}()
}
