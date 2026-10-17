package handler

import (
	"github.com/Cyclov/gophermart/internal/auth"
	"github.com/go-chi/chi/v5"
	"net/http"
)

func NewRouter(service AuthService, tokens *auth.Tokens, orders OrderService, balances BalanceService) http.Handler {
	r := chi.NewRouter()
	r.Use(GzipMiddleware)
	r.Route("/api/user", func(r chi.Router) {
		r.Post("/register", authEndpoint(service, true))
		r.Post("/login", authEndpoint(service, false))
		r.Group(func(r chi.Router) {
			r.Use(tokens.Middleware)
			r.Post("/orders", uploadOrder(orders))
			r.Get("/orders", listOrders(orders))
			r.Get("/balance", getBalance(balances))
			r.Post("/balance/withdraw", withdraw(balances))
			r.Get("/withdrawals", listWithdrawals(balances))
		})
	})
	return r
}
