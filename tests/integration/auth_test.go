package integration

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Cyclov/gophermart/internal/auth"
	"github.com/Cyclov/gophermart/internal/handler"
	"github.com/Cyclov/gophermart/internal/service"
	"github.com/Cyclov/gophermart/internal/storage"
)

func TestAuthIntegration(t *testing.T) {
	ctx, pool, _ := testDatabase(t)
	t.Helper()
	tokens, err := auth.New(strings.Repeat("test-key", 8))
	if err != nil {
		t.Fatal(err)
	}
	router := handler.NewRouter(service.NewAuth(storage.NewUsers(pool), tokens), tokens, service.NewOrders(storage.NewOrders(pool)), service.NewBalances(storage.NewBalances(pool)))
	request := func(path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		if path == "/api/user/orders" {
			req.Header.Set("Content-Type", "text/plain")
		} else {
			req.Header.Set("Content-Type", "application/json")
		}
		if cookie != nil {
			req.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	registered := request("/api/user/register", `{"login":"bob","password":"secret"}`, nil)
	if registered.Code != 200 {
		t.Fatalf("registration: %d", registered.Code)
	}
	cookies := registered.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].MaxAge != 86400 {
		t.Fatal("missing session cookie")
	}
	cookie := cookies[0]
	if response := request("/api/user/orders", "12345678903", cookie); response.Code != 202 {
		t.Fatalf("automatic authentication: %d", response.Code)
	}
	for _, tt := range []struct {
		path, body string
		status     int
	}{
		{"register", `{"login":"bob","password":"secret"}`, 409},
		{"register", `{"login":"","password":"secret"}`, 400},
		{"register", `{"login":"other","password":"secret"} {}`, 400},
		{"login", `{"login":"bob","password":"bad"}`, 401},
		{"login", `{"login":"missing","password":"bad"}`, 401},
		{"login", `{"login":"bob","password":"secret"}`, 200},
		{"login", `invalid`, 400},
	} {
		if response := request("/api/user/"+tt.path, tt.body, nil); response.Code != tt.status {
			t.Fatalf("%s: status %d, want %d", tt.path, response.Code, tt.status)
		}
	}
	for _, value := range []string{"", "forged", cookie.Value + "x"} {
		if response := request("/api/user/orders", "123", &http.Cookie{Name: "session", Value: value}); response.Code != 401 {
			t.Fatalf("invalid token: %d", response.Code)
		}
	}
	var stored string
	if err := pool.QueryRow(ctx, "SELECT password_hash FROM users WHERE login='bob'").Scan(&stored); err != nil || stored == "secret" || !strings.HasPrefix(stored, "pbkdf2-sha256$") {
		t.Fatal("password not hashed")
	}
	var accountCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM accounts a JOIN users u ON u.id=a.user_id WHERE u.login='bob'").Scan(&accountCount); err != nil || accountCount != 1 {
		t.Fatal("account missing")
	}
	restartedTokens, _ := auth.New(strings.Repeat("test-key", 8))
	restarted := handler.NewRouter(service.NewAuth(storage.NewUsers(pool), restartedTokens), restartedTokens, service.NewOrders(storage.NewOrders(pool)), service.NewBalances(storage.NewBalances(pool)))
	req := httptest.NewRequest(http.MethodGet, "/api/user/balance", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	restarted.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatal("session did not survive service recreation")
	}

}
func TestRegistrationRollbackIntegration(t *testing.T) {
	ctx, pool, _ := testDatabase(t)
	var before, after int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "ALTER TABLE accounts ALTER COLUMN current SET DEFAULT -1"); err != nil {
		t.Fatal(err)
	}
	_, err := storage.NewUsers(pool).CreateUser(ctx, "rollback-user", "hash")
	if _, resetErr := pool.Exec(ctx, "ALTER TABLE accounts ALTER COLUMN current SET DEFAULT 0"); resetErr != nil {
		t.Fatal(resetErr)
	}
	if err == nil {
		t.Fatal("expected account constraint failure")
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&after); err != nil || before != after {
		t.Fatal("registration was not rolled back")
	}
}
