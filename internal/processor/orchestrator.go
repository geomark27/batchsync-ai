package processor

import (
	"context"
	"fmt"

	"batchsync-ai/internal/ai"
	"batchsync-ai/internal/model"
	"golang.org/x/time/rate"
	"google.golang.org/genai"
)

// Config holds the processor tuning parameters.
type Config struct {
	BatchSize  int // number of LogEntries per Gemini call
	MaxWorkers int // max concurrent goroutines
	RateLimit  int // max Gemini requests per second (0 = no limit)
}

// Run orchestrates the full pipeline: chunk → parallel AI analysis → collect results.
func Run(ctx context.Context, client *genai.Client, entries []model.LogEntry, cfg Config) ([]model.ResultadoIA, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	if cfg.BatchSize <= 0 {
		return nil, fmt.Errorf("processor: BatchSize must be > 0")
	}
	if cfg.MaxWorkers <= 0 {
		return nil, fmt.Errorf("processor: MaxWorkers must be > 0")
	}

	blocks := chunk(entries, cfg.BatchSize)

	rps := cfg.RateLimit
	if rps <= 0 {
		rps = cfg.MaxWorkers // default: one token per worker slot
	}
	limiter := rate.NewLimiter(rate.Limit(rps), rps)

	fn := func(ctx context.Context, block []model.LogEntry) ([]model.ResultadoIA, error) {
		return ai.Analyze(ctx, client, block)
	}

	return procesarConGoroutines(ctx, blocks, cfg.MaxWorkers, limiter, fn)
}
