package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/Cyclov/gophermart/internal/auth"
	"github.com/Cyclov/gophermart/internal/model"
	"github.com/Cyclov/gophermart/internal/service"
)

type OrderService interface {
	Upload(context.Context, int64, string) (bool, error)
	List(context.Context, int64) ([]model.Order, error)
}

func uploadOrder(orders OrderService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := auth.UserID(r.Context())
		if !ok {
			http.Error(w, "unauthorized", 401)
			return
		}
		kind, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || kind != "text/plain" {
			http.Error(w, "expected text/plain", 400)
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
		if err != nil {
			http.Error(w, "invalid request body", 400)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		added, err := orders.Upload(ctx, id, strings.TrimSpace(string(body)))
		switch {
		case errors.Is(err, service.ErrOrderFormat):
			http.Error(w, "invalid order format", 400)
		case errors.Is(err, service.ErrOrderNumber):
			http.Error(w, "invalid order number", 422)
		case errors.Is(err, service.ErrOrderOwner):
			http.Error(w, "order already uploaded by another user", 409)
		case err != nil:
			http.Error(w, "internal server error", 500)
		case added:
			w.WriteHeader(202)
		default:
			w.WriteHeader(200)
		}
	}
}

func listOrders(orders OrderService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := auth.UserID(r.Context())
		if !ok {
			http.Error(w, "unauthorized", 401)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		result, err := orders.List(ctx, id)
		if err != nil {
			http.Error(w, "internal server error", 500)
			return
		}
		if len(result) == 0 {
			w.WriteHeader(204)
			return
		}
		writeJSON(w, result)
	}
}

func writeJSON(w http.ResponseWriter, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, _ = w.Write(body)
}
