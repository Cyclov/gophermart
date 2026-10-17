package accrual

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/Cyclov/gophermart/internal/model"
)

type Queue interface {
	Pending(context.Context, int) ([]string, error)
	Apply(context.Context, model.Accrual, time.Duration) error
	Retry(context.Context, string, time.Duration) error
}
type Fetcher interface {
	Get(context.Context, string) (model.Accrual, error)
}

type Worker struct {
	queue       Queue
	client      Fetcher
	logger      *slog.Logger
	interval    time.Duration
	parallelism int
}

func NewWorker(queue Queue, client Fetcher, logger *slog.Logger, parallelism int) *Worker {
	return &Worker{queue: queue, client: client, logger: logger, interval: time.Second, parallelism: min(max(parallelism, 1), 32)}
}
func (w *Worker) Run(ctx context.Context) {
	for ctx.Err() == nil {
		delay := w.step(ctx)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
func (w *Worker) step(ctx context.Context) time.Duration {
	numbers, err := w.queue.Pending(ctx, 100)
	if err != nil {
		w.logger.Error("cannot read accrual queue")
		return w.interval
	}
	if len(numbers) == 0 || ctx.Err() != nil {
		return w.interval
	}
	batchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := make(chan string)
	var workers sync.WaitGroup
	var pauseMu sync.Mutex
	pause := w.interval
	for i := 0; i < w.parallelism; i++ {
		workers.Go(func() {
			for {
				select {
				case <-batchCtx.Done():
					return
				case number, ok := <-jobs:
					if !ok || batchCtx.Err() != nil {
						return
					}
					result, err := w.client.Get(batchCtx, number)
					if err == nil {
						if err := w.queue.Apply(ctx, result, w.interval); err != nil {
							w.logger.Error("cannot apply accrual result")
						}
						continue
					}
					delay := w.interval
					var rate *RateLimitError
					if errors.As(err, &rate) {
						delay = max(delay, rate.Delay)
						pauseMu.Lock()
						pause = max(pause, delay)
						pauseMu.Unlock()
						cancel()
					}
					if ctx.Err() == nil {
						if err := w.queue.Retry(ctx, number, delay); err != nil {
							w.logger.Error("cannot schedule accrual retry")
						}
					}
				}
			}
		})
	}
	seen := make(map[string]struct{}, len(numbers))
dispatch:
	for _, number := range numbers {
		if _, duplicate := seen[number]; duplicate {
			continue
		}
		seen[number] = struct{}{}
		select {
		case <-batchCtx.Done():
			break dispatch
		case jobs <- number:
		}
	}
	close(jobs)
	workers.Wait()
	return pause
}
