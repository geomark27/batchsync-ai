package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"batchsync-ai/internal/ai"
	"batchsync-ai/internal/config"
	"batchsync-ai/internal/database"
	"batchsync-ai/internal/processor"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		log.Fatalf("batchsync: %v", err)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	db, err := database.InitDB(cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	client, err := ai.ConfigureGeminiClient(ctx, cfg.GeminiAPIKey)
	if err != nil {
		return err
	}

	fetchLimit := cfg.BatchSize * cfg.MaxWorkers
	if fetchLimit <= 0 {
		fetchLimit = cfg.BatchSize
	}

	entries, err := database.FetchPendingLogs(db, fetchLimit)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		log.Printf("batchsync: no pending logs found")
		return nil
	}

	results, err := processor.Run(ctx, client, entries, processor.Config{
		BatchSize:  cfg.BatchSize,
		MaxWorkers: cfg.MaxWorkers,
		RateLimit:  cfg.MaxWorkers,
	})
	if err != nil {
		return err
	}

	if err := database.SaveBatchResult(db, results, *cfg); err != nil {
		return err
	}

	ids := make([]int, len(entries))
	for i, entry := range entries {
		ids[i] = entry.ID
	}

	if err := database.MarkLogsProcessed(db, ids, cfg.MPPQ); err != nil {
		return err
	}

	log.Printf("batchsync: processed %d logs and stored %d results", len(entries), len(results))
	return nil
}
