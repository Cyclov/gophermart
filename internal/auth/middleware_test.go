package auth

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestMiddleware(t *testing.T) {
	tokens, _ := New(strings.Repeat("s", 32))
	raw, _ := tokens.Issue(42)
	h := tokens.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := UserID(r.Context())
		if !ok || id != 42 {
			t.Fatal("missing identity")
		}
		w.Header().Set("X-User", strconv.FormatInt(id, 10))
		w.WriteHeader(204)
	}))
	for _, tt := range []struct {
		name, header, cookie string
		status               int
	}{
		{"bearer", "Bearer " + raw, "", 204},
		{"cookie", "", raw, 204},
		{"missing", "", "", 401},
		{"invalid", "Bearer broken", "", 401},
		{"header takes precedence", "Bearer broken", raw, 401},
		{"wrong scheme", "Basic " + raw, "", 401},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			if tt.header != "" {
				r.Header.Set("Authorization", tt.header)
			}
			if tt.cookie != "" {
				r.AddCookie(&http.Cookie{Name: "session", Value: tt.cookie})
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tt.status {
				t.Fatalf("status %d, want %d", w.Code, tt.status)
			}
		})
	}
}
