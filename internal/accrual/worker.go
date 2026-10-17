package accrual

import (
	"context"
	"errors"
	"log/slog"
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
	queue    Queue
	client   Fetcher
	logger   *slog.Logger
	interval time.Duration
}

func NewWorker(queue Queue, client Fetcher, logger *slog.Logger) *Worker {
	return &Worker{queue: queue, client: client, logger: logger, interval: time.Second}
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
	for _, number := range numbers {
		if ctx.Err() != nil {
			return w.interval
		}
		result, err := w.client.Get(ctx, number)
		if err != nil {
			delay := w.interval
			var rate *RateLimitError
			limited := errors.As(err, &rate)
			if limited && rate.Delay > delay {
				delay = rate.Delay
			}
			if retryErr := w.queue.Retry(ctx, number, delay); retryErr != nil {
				w.logger.Error("cannot schedule accrual retry")
			}
			if limited {
				return delay
			}
			continue
		}
		if err := w.queue.Apply(ctx, result, w.interval); err != nil {
			w.logger.Error("cannot apply accrual result")
		}
	}
	return w.interval
}
