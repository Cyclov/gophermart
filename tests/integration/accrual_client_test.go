package integration

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Cyclov/gophermart/internal/accrual"
)

func TestAccrualHTTPIntegration(t *testing.T) {
	for _, tt := range []struct {
		status int
		body   string
		want   error
	}{
		{200, `{"order":"123","status":"PROCESSED","accrual":500.25}`, nil},
		{204, "", accrual.ErrPending}, {429, "", nil}, {500, "", accrual.ErrTemporary}, {200, `broken`, accrual.ErrResponse},
	} {
		t.Run(fmt.Sprint(tt.status, tt.body), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/orders/123" {
					t.Errorf("path: %s", r.URL.Path)
				}
				if tt.status == 429 {
					w.Header().Set("Retry-After", "3")
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			got, err := accrual.NewClient(server.URL).Get(context.Background(), "123")
			if tt.status == 429 {
				var rate *accrual.RateLimitError
				if !errors.As(err, &rate) || rate.Delay != 3*time.Second {
					t.Fatal("rate limit not preserved")
				}
				return
			}
			if !errors.Is(err, tt.want) {
				t.Fatalf("error %v, want %v", err, tt.want)
			}
			if err == nil && (got.Amount == nil || got.Amount.String() != "500.25") {
				t.Fatal("amount lost")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := accrual.NewClient("http://127.0.0.1:1").Get(ctx, "123"); !errors.Is(err, accrual.ErrTemporary) {
		t.Fatal("cancellation not handled")
	}
}
