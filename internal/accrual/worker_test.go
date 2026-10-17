package accrual

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Cyclov/gophermart/internal/model"
)

type queueStub struct {
	numbers          []string
	err              error
	applied, retries int
	delay            time.Duration
}

func (q *queueStub) Pending(ctx context.Context, limit int) ([]string, error) {
	return q.numbers, q.err
}
func (q *queueStub) Apply(ctx context.Context, r model.Accrual, d time.Duration) error {
	q.applied++
	return q.err
}
func (q *queueStub) Retry(ctx context.Context, n string, d time.Duration) error {
	q.retries++
	q.delay = d
	return q.err
}

type fetchStub struct {
	result model.Accrual
	err    error
	calls  int
}

func (f *fetchStub) Get(ctx context.Context, n string) (model.Accrual, error) {
	f.calls++
	return f.result, f.err
}
func TestWorkerStep(t *testing.T) {
	for _, tt := range []struct {
		name                    string
		fetchErr, queueErr      error
		applied, retries, calls int
		delay                   time.Duration
	}{
		{"success", nil, nil, 2, 0, 2, time.Second},
		{"pending", ErrPending, nil, 0, 2, 2, time.Second},
		{"temporary", ErrTemporary, nil, 0, 2, 2, time.Second},
		{"invalid response", ErrResponse, nil, 0, 2, 2, time.Second},
		{"rate limit", &RateLimitError{Delay: 3 * time.Second}, nil, 0, 1, 1, 3 * time.Second},
		{"database error", nil, errors.New("failed"), 0, 0, 0, time.Second},
	} {
		t.Run(tt.name, func(t *testing.T) {
			q := &queueStub{numbers: []string{"1", "2"}, err: tt.queueErr}
			f := &fetchStub{err: tt.fetchErr}
			w := NewWorker(q, f, slog.New(slog.NewTextHandler(io.Discard, nil)))
			delay := w.step(context.Background())
			if delay != tt.delay || q.applied != tt.applied || q.retries != tt.retries || f.calls != tt.calls {
				t.Fatalf("delay=%v applied=%d retries=%d calls=%d", delay, q.applied, q.retries, f.calls)
			}
		})
	}
}
func TestWorkerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	q := &queueStub{numbers: []string{"1"}}
	f := new(fetchStub)
	w := NewWorker(q, f, slog.New(slog.NewTextHandler(io.Discard, nil)))
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop")
	}
	if f.calls != 0 {
		t.Fatal("cancelled worker fetched orders")
	}
}

type cancelFetcher struct {
	cancel context.CancelFunc
	calls  int
}

func (f *cancelFetcher) Get(ctx context.Context, n string) (model.Accrual, error) {
	f.calls++
	if f.calls == 2 {
		f.cancel()
	}
	return model.Accrual{}, ErrPending
}
func TestWorkerRunStopsBetweenPolls(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q := &queueStub{numbers: []string{"123"}}
	f := &cancelFetcher{cancel: cancel}
	w := NewWorker(q, f, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w.interval = time.Millisecond
	done := make(chan struct{})
	go func() { defer close(done); w.Run(ctx) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop")
	}
	if f.calls != 2 || q.retries != 2 {
		t.Fatal("unexpected polling lifecycle")
	}
}
