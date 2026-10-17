package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Cyclov/gophermart/internal/auth"
	"github.com/Cyclov/gophermart/internal/model"
	"github.com/Cyclov/gophermart/internal/service"
)

type BalanceService interface {
	Get(context.Context, int64) (model.Balance, error)
	Withdraw(context.Context, int64, string, json.Number) error
	History(context.Context, int64) ([]model.Withdrawal, error)
}

func getBalance(balances BalanceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := auth.UserID(r.Context())
		if !ok {
			http.Error(w, "unauthorized", 401)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		balance, err := balances.Get(ctx, id)
		if err != nil {
			http.Error(w, "internal server error", 500)
			return
		}
		writeJSON(w, balance)
	}
}
func withdraw(balances BalanceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := auth.UserID(r.Context())
		if !ok {
			http.Error(w, "unauthorized", 401)
			return
		}
		withdrawalResponse(balances, id, w, r)
	}
}
func withdrawalResponse(balances BalanceService, id int64, w http.ResponseWriter, r *http.Request) {
	var input struct {
		Order string          `json:"order"`
		Sum   json.RawMessage `json:"sum"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	if err := decoder.Decode(&input); err != nil {
		http.Error(w, "invalid request", 400)
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		http.Error(w, "invalid request", 400)
		return
	}
	raw := strings.TrimSpace(string(input.Sum))
	if len(raw) == 0 || raw[0] == '"' {
		http.Error(w, "invalid amount", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	err := balances.Withdraw(ctx, id, input.Order, json.Number(raw))
	switch {
	case errors.Is(err, service.ErrOrderNumber):
		http.Error(w, "invalid order number", 422)
	case errors.Is(err, service.ErrAmount):
		http.Error(w, "invalid amount", 400)
	case errors.Is(err, service.ErrFunds):
		http.Error(w, "insufficient funds", 402)
	case err != nil:
		http.Error(w, "internal server error", 500)
	default:
		w.WriteHeader(200)
	}
}
func listWithdrawals(balances BalanceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := auth.UserID(r.Context())
		if !ok {
			http.Error(w, "unauthorized", 401)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		list, err := balances.History(ctx, id)
		if err != nil {
			http.Error(w, "internal server error", 500)
			return
		}
		if len(list) == 0 {
			w.WriteHeader(204)
			return
		}
		writeJSON(w, list)
	}
}
