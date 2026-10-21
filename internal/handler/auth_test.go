package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Cyclov/gophermart/internal/service"
)

type authStub struct {
	token           string
	err             error
	called          bool
	register        bool
	login, password string
}

func (s *authStub) Register(ctx context.Context, login, password string) (string, error) {
	s.called = true
	s.register = true
	s.login = login
	s.password = password
	return s.token, s.err
}
func (s *authStub) Login(ctx context.Context, login, password string) (string, error) {
	s.called = true
	s.login = login
	s.password = password
	return s.token, s.err
}

func TestAuthEndpoint(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		register   bool
		err        error
		status     int
		called     bool
	}{
		{"register", `{"login":"alice","password":"secret"}`, true, nil, 200, true},
		{"login", `{"login":"alice","password":"secret"}`, false, nil, 200, true},
		{"duplicate login", `{"login":"alice","password":"secret"}`, true, service.ErrLoginTaken, 409, true},
		{"wrong credentials", `{"login":"alice","password":"secret"}`, false, service.ErrCredentials, 401, true},
		{"storage error", `{"login":"alice","password":"secret"}`, false, errors.New("failed"), 500, true},
		{"invalid json", `{`, true, nil, 400, false},
		{"empty login", `{"login":" ","password":"secret"}`, true, nil, 400, false},
		{"empty password", `{"login":"alice"}`, false, nil, 400, false},
		{"multiple objects", `{"login":"alice","password":"secret"} {}`, true, nil, 400, false},
		{"oversized", `{"login":"alice","password":"` + strings.Repeat("a", 64<<10) + `"}`, true, nil, 400, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stub := &authStub{token: "opaque-token", err: tt.err}
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
			w := httptest.NewRecorder()
			authEndpoint(stub, tt.register).ServeHTTP(w, req)
			if w.Code != tt.status {
				t.Fatalf("status %d, want %d", w.Code, tt.status)
			}
			if stub.called != tt.called {
				t.Fatal("unexpected service call")
			}
			if stub.called && (stub.login != "alice" || stub.password != "secret" || stub.register != tt.register) {
				t.Fatal("incorrect service arguments")
			}
			if tt.status == 200 {
				if w.Header().Get("Authorization") != "Bearer opaque-token" {
					t.Fatal("missing token header")
				}
				cookies := w.Result().Cookies()
				if len(cookies) != 1 || cookies[0].Value != "opaque-token" || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode {
					t.Fatal("invalid cookie")
				}
			} else if len(w.Result().Cookies()) != 0 || w.Header().Get("Authorization") != "" {
				t.Fatal("token issued on error")
			}
		})
	}
}
