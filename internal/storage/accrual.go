package storage

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Cyclov/gophermart/internal/model"
)

func (s *Orders) Pending(ctx context.Context, limit int) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT number FROM orders WHERE status IN ('NEW','PROCESSING') AND next_poll_at<=now()
 ORDER BY next_poll_at,number LIMIT $1`, limit)
	if err != nil {
		return nil, errors.New("cannot query pending orders")
	}
	defer rows.Close()
	var numbers []string
	for rows.Next() {
		var number string
		if err := rows.Scan(&number); err != nil {
			return nil, errors.New("cannot read pending order")
		}
		numbers = append(numbers, number)
	}
	if rows.Err() != nil {
		return nil, errors.New("cannot read pending orders")
	}
	return numbers, nil
}
func (s *Orders) Retry(ctx context.Context, number string, delay time.Duration) error {
	_, err := s.pool.Exec(ctx, `UPDATE orders SET next_poll_at=now()+$2::double precision*interval '1 second'
 WHERE number=$1 AND status IN ('NEW','PROCESSING')`, number, delay.Seconds())
	if err != nil {
		return errors.New("cannot schedule order")
	}
	return nil
}
func (s *Orders) Apply(ctx context.Context, result model.Accrual, delay time.Duration) error {
	status := result.Status
	switch status {
	case "REGISTERED":
		status = "PROCESSING"
	case "PROCESSING", "INVALID", "PROCESSED":
	default:
		return errors.New("invalid accrual status")
	}
	amount := "0"
	if result.Amount != nil {
		var err error
		amount, err = model.Amount(json.Number(*result.Amount), false)
		if err != nil {
			return err
		}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return errors.New("cannot begin accrual transaction")
	}
	defer tx.Rollback(ctx)
	var current string
	var userID int64
	if err := tx.QueryRow(ctx, "SELECT status,user_id FROM orders WHERE number=$1 FOR UPDATE", result.Number).Scan(&current, &userID); err != nil {
		return errors.New("cannot lock order")
	}
	if current == "INVALID" || current == "PROCESSED" {
		return nil
	}
	var accrual any
	if status == "PROCESSED" {
		accrual = amount
	}
	if _, err := tx.Exec(ctx, `UPDATE orders SET status=$2,accrual=$3::numeric,
 next_poll_at=now()+$4::double precision*interval '1 second' WHERE number=$1`, result.Number, status, accrual, delay.Seconds()); err != nil {
		return errors.New("cannot update order")
	}
	if status == "PROCESSED" {
		tag, err := tx.Exec(ctx, "UPDATE accounts SET current=current+$2::numeric WHERE user_id=$1", userID, amount)
		if err != nil || tag.RowsAffected() != 1 {
			return errors.New("cannot credit account")
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return errors.New("cannot commit accrual")
	}
	return nil
}
