package integration

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Cyclov/gophermart/internal/accrual"
	"github.com/Cyclov/gophermart/internal/model"
	"github.com/Cyclov/gophermart/internal/storage"
)

func TestWorkerPoolIntegration(t *testing.T) {
	ctx, pool, _ := testDatabase(t)
	id, err := storage.NewUsers(pool).CreateUser(ctx, "pool", "hash")
	if err != nil {
		t.Fatal(err)
	}
	orders := storage.NewOrders(pool)
	for _, number := range []string{"12345678903", "79927398713", "2377225624"} {
		if _, err := orders.AddOrder(ctx, id, number); err != nil {
			t.Fatal(err)
		}
	}
	started := make(chan struct{}, 3)
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	external := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		select {
		case <-r.Context().Done():
			return
		case <-release:
		}
		amount := json.Number("1")
		_ = json.NewEncoder(w).Encode(model.Accrual{Number: strings.TrimPrefix(r.URL.Path, "/api/orders/"), Status: "PROCESSED", Amount: &amount})
	}))
	defer external.Close()
	defer unblock()
	workerCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	worker := accrual.NewWorker(orders, accrual.NewClient(external.URL), slog.New(slog.NewTextHandler(io.Discard, nil)), 3)
	go func() { defer close(done); worker.Run(workerCtx) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("worker pool did not terminate")
		}
	}()
	for i := 0; i < 3; i++ {
		select {
		case <-started:
		case <-time.After(3 * time.Second):
			t.Fatal("HTTP requests did not run in parallel")
		}
	}
	unblock()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var credited bool
		if err := pool.QueryRow(ctx, "SELECT current=3 FROM accounts WHERE user_id=$1", id).Scan(&credited); err != nil {
			t.Fatal(err)
		}
		if credited {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("parallel orders not credited")
		}
		time.Sleep(10 * time.Millisecond)
	}
	list, err := orders.ListOrders(ctx, id)
	if err != nil || len(list) != 3 {
		t.Fatal("orders missing")
	}
	for _, order := range list {
		if order.Status != "PROCESSED" {
			t.Fatal("unfinished order")
		}
	}
}
