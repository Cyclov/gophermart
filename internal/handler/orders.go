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
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		uploadOrderResponse(orders, id, w, r)
	}
}
func uploadOrderResponse(orders OrderService, id int64, w http.ResponseWriter, r *http.Request) {
	kind, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || kind != "text/plain" {
		http.Error(w, "expected text/plain", http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	added, err := orders.Upload(ctx, id, strings.TrimSpace(string(body)))
	switch {
	case errors.Is(err, service.ErrOrderFormat):
		http.Error(w, "invalid order format", http.StatusBadRequest)
	case errors.Is(err, service.ErrOrderNumber):
		http.Error(w, "invalid order number", http.StatusUnprocessableEntity)
	case errors.Is(err, service.ErrOrderOwner):
		http.Error(w, "order already uploaded by another user", http.StatusConflict)
	case err != nil:
		http.Error(w, "internal server error", http.StatusInternalServerError)
	case added:
		w.WriteHeader(http.StatusAccepted)
	default:
		w.WriteHeader(http.StatusOK)
	}
}
func listOrders(orders OrderService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := auth.UserID(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		listOrdersResponse(orders, id, w, r)
	}
}
func listOrdersResponse(orders OrderService, id int64, w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	result, err := orders.List(ctx, id)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if len(result) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, result)
}
func writeJSON(w http.ResponseWriter, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
