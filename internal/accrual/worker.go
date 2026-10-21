package accrual

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/Cyclov/gophermart/internal/model"
	"golang.org/x/sync/errgroup"
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
	group, batchCtx := errgroup.WithContext(ctx)
	group.SetLimit(w.parallelism)
	pollErrors := make([]error, len(numbers))
	seen := make(map[string]struct{}, len(numbers))
	for i, number := range numbers {
		if batchCtx.Err() != nil {
			break
		}
		if _, duplicate := seen[number]; duplicate {
			continue
		}
		seen[number] = struct{}{}
		group.Go(func() error {
			// Go may have waited for a slot while another request cancelled the batch.
			if batchCtx.Err() != nil {
				return nil
			}
			result, err := w.client.Get(batchCtx, number)
			if err == nil {
				if err := w.queue.Apply(ctx, result, w.interval); err != nil {
					w.logger.Error("cannot apply accrual result")
				}
				return nil
			}
			pollErrors[i] = err
			var rate *RateLimitError
			if errors.As(err, &rate) {
				return err
			}
			return nil
		})
	}
	// Keep every failure, rather than only Wait's first error, to honor the longest Retry-After.
	_ = group.Wait()
	pause := w.interval
	for i, err := range pollErrors {
		if err == nil {
			continue
		}
		delay := w.interval
		var rate *RateLimitError
		if errors.As(err, &rate) {
			delay = max(delay, rate.Delay)
			pause = max(pause, delay)
		}
		if ctx.Err() == nil {
			if err := w.queue.Retry(ctx, numbers[i], delay); err != nil {
				w.logger.Error("cannot schedule accrual retry")
			}
		}
	}
	return pause
}
