package integration

import (
	"context"
	"encoding/json"

	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Cyclov/gophermart/internal/accrual"
	"github.com/Cyclov/gophermart/internal/auth"
	"github.com/Cyclov/gophermart/internal/handler"
	"github.com/Cyclov/gophermart/internal/model"
	"github.com/Cyclov/gophermart/internal/service"
	"github.com/Cyclov/gophermart/internal/storage"
)

func TestFullLifecycleIntegration(t *testing.T) {
	ctx, pool, dsn := testDatabase(t)
	var calls atomic.Int32
	external := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch calls.Add(1) {
		case 1:
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
		case 2:
			w.WriteHeader(204)
		case 3:
			w.WriteHeader(500)
		case 4:
			_, _ = io.WriteString(w, `broken`)
		case 5:
			_, _ = io.WriteString(w, `{"order":"12345678903","status":"REGISTERED"}`)
		case 6:
			_, _ = io.WriteString(w, `{"order":"12345678903","status":"PROCESSING"}`)
		default:
			_, _ = io.WriteString(w, `{"order":"12345678903","status":"PROCESSED","accrual":5.555}`)
		}
	}))
	defer external.Close()
	tokens, _ := auth.New(strings.Repeat("k", 32))
	router := handler.NewRouter(service.NewAuth(storage.NewUsers(pool), tokens), tokens, service.NewOrders(storage.NewOrders(pool)), service.NewBalances(storage.NewBalances(pool)))
	api := httptest.NewServer(router)
	defer api.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 3 * time.Second}
	request := func(method, path, body, kind string) (int, []byte) {
		t.Helper()
		req, err := http.NewRequestWithContext(ctx, method, api.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if kind != "" {
			req.Header.Set("Content-Type", kind)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode, data
	}
	status, _ := request("POST", "/api/user/register", `{"login":"lifecycle","password":"secret"}`, "application/json")
	if status != 200 {
		t.Fatalf("register %d", status)
	}
	status, body := request("GET", "/api/user/orders", "", "")
	if status != 204 || len(body) != 0 {
		t.Fatal("empty orders response")
	}
	status, _ = request("POST", "/api/user/orders", "12345678903", "text/plain")
	if status != 202 {
		t.Fatalf("upload %d", status)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	workerCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		accrual.NewWorker(storage.NewOrders(pool), accrual.NewClient(external.URL), logger).Run(workerCtx)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for calls.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("initial worker did not stop")
	}
	if calls.Load() < 2 {
		t.Fatal("pending responses not retried")
	}
	restarted, err := storage.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	workerCtx, cancel = context.WithCancel(ctx)
	done = make(chan struct{})
	go func() {
		defer close(done)
		accrual.NewWorker(storage.NewOrders(restarted), accrual.NewClient(external.URL), logger).Run(workerCtx)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("worker did not stop")
		}
	})
	deadline = time.Now().Add(12 * time.Second)
	for {
		status, body = request("GET", "/api/user/orders", "", "")
		var orders []model.Order
		if status != 200 || json.Unmarshal(body, &orders) != nil {
			t.Fatal("invalid order list")
		}
		if len(orders) == 1 && orders[0].Status == "PROCESSED" {
			if orders[0].Accrual == nil || orders[0].Accrual.String() != "5.555" {
				t.Fatal("wrong accrual")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("order did not complete")
		}
		time.Sleep(30 * time.Millisecond)
	}
	status, _ = request("POST", "/api/user/balance/withdraw", `{"order":"2377225624","sum":0.555}`, "application/json")
	if status != 200 {
		t.Fatalf("withdraw %d", status)
	}
	status, body = request("GET", "/api/user/balance", "", "")
	var balance model.Balance
	if status != 200 || json.Unmarshal(body, &balance) != nil {
		t.Fatal("invalid balance")
	}
	current, ok := new(big.Rat).SetString(balance.Current.String())
	if !ok || current.Cmp(big.NewRat(5, 1)) != 0 {
		t.Fatalf("balance %s", balance.Current)
	}
	status, body = request("GET", "/api/user/withdrawals", "", "")
	var history []model.Withdrawal
	if status != 200 || json.Unmarshal(body, &history) != nil || len(history) != 1 || history[0].Sum != "0.555" {
		t.Fatal("invalid history")
	}
	for _, tt := range []struct {
		body   string
		status int
	}{{`{"order":"2377225624","sum":6}`, 402}, {`{"order":"123","sum":1}`, 422}, {`{"order":"2377225624","sum":-1}`, 400}, {`broken`, 400}} {
		got, _ := request("POST", "/api/user/balance/withdraw", tt.body, "application/json")
		if got != tt.status {
			t.Fatalf("withdraw %s: %d", tt.body, got)
		}
	}
	if calls.Load() < 7 {
		t.Fatalf("missing retry states: %d", calls.Load())
	}
}
