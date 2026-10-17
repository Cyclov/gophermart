package handler

import (
	"net/http/httptest"
	"testing"
)

func TestRouter(t *testing.T) {
	r := NewRouter()
	for _, tt := range []struct {
		method, path string
		status       int
	}{
		{"POST", "/api/user/register", 501},
		{"POST", "/api/user/login", 501},
		{"POST", "/api/user/orders", 501},
		{"GET", "/api/user/orders", 501},
		{"GET", "/api/user/balance", 501},
		{"POST", "/api/user/balance/withdraw", 501},
		{"GET", "/api/user/withdrawals", 501},
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
