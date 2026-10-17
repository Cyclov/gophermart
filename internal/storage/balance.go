package storage

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Cyclov/gophermart/internal/model"
	"github.com/Cyclov/gophermart/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Balances struct{ pool *pgxpool.Pool }

func NewBalances(pool *pgxpool.Pool) *Balances { return &Balances{pool: pool} }
func (s *Balances) GetBalance(ctx context.Context, id int64) (model.Balance, error) {
	var result model.Balance
	var current, withdrawn string
	if err := s.pool.QueryRow(ctx, "SELECT current::text,withdrawn::text FROM accounts WHERE user_id=$1", id).Scan(&current, &withdrawn); err != nil {
		return result, errors.New("cannot read balance")
	}
	result.Current = json.Number(current)
	result.Withdrawn = json.Number(withdrawn)
	return result, nil
}
func (s *Balances) Withdraw(ctx context.Context, id int64, order, amount string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return errors.New("cannot begin withdrawal")
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE accounts SET current=current-$2::numeric,withdrawn=withdrawn+$2::numeric
 WHERE user_id=$1 AND current>=$2::numeric`, id, amount)
	if err != nil {
		return errors.New("cannot debit account")
	}
	if tag.RowsAffected() != 1 {
		return service.ErrFunds
	}
	if _, err := tx.Exec(ctx, "INSERT INTO withdrawals(user_id,order_number,sum) VALUES ($1,$2,$3::numeric)", id, order, amount); err != nil {
		return errors.New("cannot record withdrawal")
	}
	if err := tx.Commit(ctx); err != nil {
		return errors.New("cannot commit withdrawal")
	}
	return nil
}
func (s *Balances) ListWithdrawals(ctx context.Context, id int64) ([]model.Withdrawal, error) {
	rows, err := s.pool.Query(ctx, `SELECT order_number,sum::text,processed_at FROM withdrawals WHERE user_id=$1 ORDER BY processed_at DESC,id DESC`, id)
	if err != nil {
		return nil, errors.New("cannot list withdrawals")
	}
	defer rows.Close()
	list := make([]model.Withdrawal, 0)
	for rows.Next() {
		var entry model.Withdrawal
		var sum string
		if err := rows.Scan(&entry.Order, &sum, &entry.ProcessedAt); err != nil {
			return nil, errors.New("cannot decode withdrawal")
		}
		entry.Sum = json.Number(sum)
		list = append(list, entry)
	}
	if rows.Err() != nil {
		return nil, errors.New("cannot read withdrawals")
	}
	return list, nil
}
