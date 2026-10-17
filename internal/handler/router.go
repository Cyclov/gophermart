package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func NewRouter() http.Handler {
	r := chi.NewRouter()
	r.Use(GzipMiddleware)
	r.Route("/api/user", func(r chi.Router) {
		r.Post("/register", notImplemented)
		r.Post("/login", notImplemented)
		r.Post("/orders", notImplemented)
		r.Get("/orders", notImplemented)
		r.Get("/balance", notImplemented)
		r.Post("/balance/withdraw", notImplemented)
		r.Get("/withdrawals", notImplemented)
	})
	return r
}

func notImplemented(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
