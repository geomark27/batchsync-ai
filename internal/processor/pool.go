package processor

import (
	"context"
	"sync"

	"batchsync-ai/internal/model"
	"golang.org/x/time/rate"
)

// WorkFunc is the function executed by each worker for a single block.
type WorkFunc func(ctx context.Context, block []model.LogEntry) ([]model.ResultadoIA, error)

// procesarConGoroutines runs fn concurrently over each block using a semaphore
// to cap goroutines at maxWorkers. The rate limiter controls how many blocks
// are dispatched per second to respect Gemini API quotas.
// Fail-fast: the first worker error cancels all remaining work.
func procesarConGoroutines(
	ctx context.Context,
	blocks [][]model.LogEntry,
	maxWorkers int,
	limiter *rate.Limiter,
	fn WorkFunc,
) ([]model.ResultadoIA, error) {
	sem := make(chan struct{}, maxWorkers)
	resultCh := make(chan []model.ResultadoIA, len(blocks))
	errCh := make(chan error, 1)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup

	for _, block := range blocks {
		if err := limiter.Wait(ctx); err != nil {
			break
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(b []model.LogEntry) {
			defer wg.Done()
			defer func() { <-sem }()

			results, err := fn(ctx, b)
			if err != nil {
				select {
				case errCh <- err:
					cancel()
				default:
				}
				return
			}
			resultCh <- results
		}(block)
	}

	wg.Wait()
	close(resultCh)

	select {
	case err := <-errCh:
		return nil, err
	default:
	}

	var all []model.ResultadoIA
	for batch := range resultCh {
		all = append(all, batch...)
	}
	return all, nil
}
