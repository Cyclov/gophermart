package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Cyclov/gophermart/internal/auth"
	"github.com/Cyclov/gophermart/internal/handler"
	"github.com/Cyclov/gophermart/internal/model"
)

func TestRouterIntegration(t *testing.T) {
	for _, tt := range []struct {
		method, path string
		status       int
	}{
		{"POST", "/api/user/register", 400},
		{"POST", "/api/user/login", 400},
		{"POST", "/api/user/orders", 401},
		{"GET", "/api/user/orders", 401},
		{"GET", "/api/user/balance", 401},
		{"POST", "/api/user/balance/withdraw", 401},
		{"GET", "/api/user/withdrawals", 401},
		{"DELETE", "/api/user/orders", 405},
		{"GET", "/missing", 404},
	} {
		t.Run(tt.method+tt.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			newValidationTestRouter(t).ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, nil))
			if w.Code != tt.status {
				t.Fatalf("got %d, want %d", w.Code, tt.status)
			}
		})
	}
}

func TestRouterRejectsInvalidGzip(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/user/orders", strings.NewReader("invalid"))
	req.Header.Set("Content-Encoding", "gzip")
	w := httptest.NewRecorder()
	newValidationTestRouter(t).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", w.Code)
	}
}

// Every service call is unexpected in tests of request validation and authentication.
type unexpectedRouterServices struct{ t *testing.T }

func (s unexpectedRouterServices) Register(context.Context, string, string) (string, error) {
	s.t.Helper()
	s.t.Fatal("unexpected AuthService.Register call")
	return "", nil
}
func (s unexpectedRouterServices) Login(context.Context, string, string) (string, error) {
	s.t.Helper()
	s.t.Fatal("unexpected AuthService.Login call")
	return "", nil
}
func (s unexpectedRouterServices) Upload(context.Context, int64, string) (bool, error) {
	s.t.Helper()
	s.t.Fatal("unexpected OrderService.Upload call")
	return false, nil
}
func (s unexpectedRouterServices) List(context.Context, int64) ([]model.Order, error) {
	s.t.Helper()
	s.t.Fatal("unexpected OrderService.List call")
	return nil, nil
}
func (s unexpectedRouterServices) Get(context.Context, int64) (model.Balance, error) {
	s.t.Helper()
	s.t.Fatal("unexpected BalanceService.Get call")
	return model.Balance{}, nil
}
func (s unexpectedRouterServices) Withdraw(context.Context, int64, string, json.Number) error {
	s.t.Helper()
	s.t.Fatal("unexpected BalanceService.Withdraw call")
	return nil
}
func (s unexpectedRouterServices) History(context.Context, int64) ([]model.Withdrawal, error) {
	s.t.Helper()
	s.t.Fatal("unexpected BalanceService.History call")
	return nil, nil
}

func newValidationTestRouter(t *testing.T) http.Handler {
	t.Helper()
	tokens, err := auth.New(strings.Repeat("test", 8))
	if err != nil {
		t.Fatal(err)
	}
	services := unexpectedRouterServices{t: t}
	return handler.NewRouter(services, tokens, services, services)
}
