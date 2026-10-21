package storage

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Cyclov/gophermart/internal/model"
	"github.com/Cyclov/gophermart/internal/service"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Orders struct{ pool *pgxpool.Pool }

func NewOrders(pool *pgxpool.Pool) *Orders { return &Orders{pool: pool} }

func (s *Orders) AddOrder(ctx context.Context, id int64, number string) (bool, error) {
	var owner int64
	err := s.pool.QueryRow(ctx, `INSERT INTO orders(number,user_id) VALUES ($1,$2)
 ON CONFLICT(number) DO NOTHING RETURNING user_id`, number, id).Scan(&owner)
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return false, errors.New("cannot save order")
	}
	if err := s.pool.QueryRow(ctx, "SELECT user_id FROM orders WHERE number=$1", number).Scan(&owner); err != nil {
		return false, errors.New("cannot read order owner")
	}
	if owner != id {
		return false, service.ErrOrderOwner
	}
	return false, nil
}

func (s *Orders) ListOrders(ctx context.Context, id int64) ([]model.Order, error) {
	rows, err := s.pool.Query(ctx, `SELECT number,status,accrual::text,uploaded_at FROM orders
 WHERE user_id=$1 ORDER BY uploaded_at DESC,number`, id)
	if err != nil {
		return nil, errors.New("cannot list orders")
	}
	defer rows.Close()
	orders := make([]model.Order, 0)
	for rows.Next() {
		var order model.Order
		var amount *string
		if err := rows.Scan(&order.Number, &order.Status, &amount, &order.UploadedAt); err != nil {
			return nil, errors.New("cannot decode order")
		}
		if amount != nil {
			n := json.Number(*amount)
			order.Accrual = &n
		}
		orders = append(orders, order)
	}
	if rows.Err() != nil {
		return nil, errors.New("cannot read orders")
	}
	return orders, nil
}
