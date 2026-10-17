package integration

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Cyclov/gophermart/internal/handler"
)

func TestRouterIntegration(t *testing.T) {
	r := handler.NewRouter(nil, nil)
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
			r.ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, nil))
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
	handler.NewRouter(nil, nil).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", w.Code)
	}
}
