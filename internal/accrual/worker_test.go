package accrual

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/Cyclov/gophermart/internal/model"
)

type queueStub struct {
	mu               sync.Mutex
	numbers          []string
	err              error
	applied, retries int
	delay            time.Duration
}

func (q *queueStub) Pending(ctx context.Context, limit int) ([]string, error) {
	return q.numbers, q.err
}
func (q *queueStub) Apply(ctx context.Context, r model.Accrual, d time.Duration) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.applied++
	return q.err
}
func (q *queueStub) Retry(ctx context.Context, n string, d time.Duration) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.retries++
	q.delay = d
	return q.err
}

type fetchStub struct {
	mu     sync.Mutex
	result model.Accrual
	err    error
	calls  int
}

func (f *fetchStub) Get(ctx context.Context, n string) (model.Accrual, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
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
			w := NewWorker(q, f, slog.New(slog.NewTextHandler(io.Discard, nil)), 1)
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
	w := NewWorker(q, f, slog.New(slog.NewTextHandler(io.Discard, nil)), 1)
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
	w := NewWorker(q, f, slog.New(slog.NewTextHandler(io.Discard, nil)), 1)
	w.interval = time.Millisecond
	done := make(chan struct{})
	go func() { defer close(done); w.Run(ctx) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop")
	}
	if f.calls != 2 || q.retries != 1 {
		t.Fatal("unexpected polling lifecycle")
	}
}

type poolFetcher struct {
	started chan string
	release chan struct{}
	mu      sync.Mutex
	seen    map[string]int
}

func (f *poolFetcher) Get(ctx context.Context, number string) (model.Accrual, error) {
	f.mu.Lock()
	f.seen[number]++
	f.mu.Unlock()
	f.started <- number
	select {
	case <-ctx.Done():
		return model.Accrual{}, ctx.Err()
	case <-f.release:
		return model.Accrual{Number: number, Status: "PROCESSED"}, nil
	}
}
func TestWorkerPoolBoundAndDeduplication(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := &poolFetcher{started: make(chan string, 5), release: make(chan struct{}), seen: make(map[string]int)}
	var once sync.Once
	release := func() { once.Do(func() { close(f.release) }) }
	defer release()
	q := &queueStub{numbers: []string{"1", "2", "3", "4", "1"}}
	w := NewWorker(q, f, slog.New(slog.NewTextHandler(io.Discard, nil)), 3)
	done := make(chan struct{})
	go func() { defer close(done); w.step(ctx) }()
	defer func() {
		cancel()
		release()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("pool did not stop")
		}
	}()
	for i := 0; i < 3; i++ {
		select {
		case <-f.started:
		case <-time.After(time.Second):
			t.Fatal("requests are not parallel")
		}
	}
	select {
	case <-f.started:
		t.Fatal("parallelism limit exceeded")
	default:
	}
	release()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("batch did not finish")
	}
	if len(f.seen) != 4 || q.applied != 4 {
		t.Fatal("incorrect job count")
	}
	for number, count := range f.seen {
		if count != 1 {
			t.Fatalf("duplicate processing %s: %d", number, count)
		}
	}
}

type rateFetcher struct {
	blocked chan struct{}
	queued  bool
	mu      sync.Mutex
}

func (f *rateFetcher) Get(ctx context.Context, number string) (model.Accrual, error) {
	switch number {
	case "rate":
		select {
		case <-f.blocked:
			return model.Accrual{}, &RateLimitError{Delay: time.Minute}
		case <-ctx.Done():
			return model.Accrual{}, ctx.Err()
		}
	case "blocked":
		close(f.blocked)
		<-ctx.Done()
		return model.Accrual{}, ctx.Err()
	default:
		f.mu.Lock()
		f.queued = true
		f.mu.Unlock()
		return model.Accrual{}, nil
	}
}
func TestWorkerPoolSharedRateLimit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := &rateFetcher{blocked: make(chan struct{})}
	q := &queueStub{numbers: []string{"rate", "blocked", "queued"}}
	w := NewWorker(q, f, slog.New(slog.NewTextHandler(io.Discard, nil)), 2)
	done := make(chan time.Duration, 1)
	go func() { done <- w.step(ctx) }()
	select {
	case delay := <-done:
		if delay != time.Minute {
			t.Fatalf("global pause %v", delay)
		}
	case <-time.After(time.Second):
		cancel()
		<-done
		t.Fatal("rate limit did not cancel in-flight request")
	}
	if f.queued {
		t.Fatal("queued request started after rate limit")
	}
	if q.retries != 2 {
		t.Fatal("in-flight jobs not rescheduled")
	}
}
func TestWorkerPoolCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := &poolFetcher{started: make(chan string, 4), release: make(chan struct{}), seen: make(map[string]int)}
	q := &queueStub{numbers: []string{"1", "2", "3", "4"}}
	w := NewWorker(q, f, slog.New(slog.NewTextHandler(io.Discard, nil)), 2)
	done := make(chan struct{})
	go func() { defer close(done); w.step(ctx) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("pool leaked")
		}
	}()
	for i := 0; i < 2; i++ {
		select {
		case <-f.started:
		case <-time.After(time.Second):
			t.Fatal("worker did not start")
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("in-flight requests not cancelled")
	}
	if q.retries != 0 || q.applied != 0 || len(f.seen) != 2 {
		t.Fatal("work continued after cancellation")
	}
}
