package handler

import (
	"github.com/Cyclov/gophermart/internal/auth"
	"github.com/go-chi/chi/v5"
	"net/http"
)

func NewRouter(service AuthService, tokens *auth.Tokens) http.Handler {
	r := chi.NewRouter()
	r.Use(GzipMiddleware)
	r.Route("/api/user", func(r chi.Router) {
		r.Post("/register", authEndpoint(service, true))
		r.Post("/login", authEndpoint(service, false))
		r.Group(func(r chi.Router) {
			r.Use(tokens.Middleware)
			r.Post("/orders", notImplemented)
			r.Get("/orders", notImplemented)
			r.Get("/balance", notImplemented)
			r.Post("/balance/withdraw", notImplemented)
			r.Get("/withdrawals", notImplemented)
		})
	})
	return r
}

func notImplemented(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
