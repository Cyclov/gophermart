package auth

import (
	"context"
	"net/http"
	"strings"
)

type userIDKey struct{}

func UserID(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(userIDKey{}).(int64)
	return id, ok
}

func (t *Tokens) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw string
		if header := r.Header.Get("Authorization"); header != "" {
			parts := strings.Fields(header)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				http.Error(w, "unauthorized", 401)
				return
			}
			raw = parts[1]
		} else if cookie, err := r.Cookie("session"); err == nil {
			raw = cookie.Value
		}
		id, err := t.Parse(raw)
		if err != nil {
			http.Error(w, "unauthorized", 401)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey{}, id)))
	})
}
