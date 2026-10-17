package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Cyclov/gophermart/internal/model"
)

var ErrFunds = errors.New("insufficient funds")
var ErrAmount = errors.New("invalid withdrawal amount")

type BalanceStore interface {
	GetBalance(context.Context, int64) (model.Balance, error)
	Withdraw(context.Context, int64, string, string) error
	ListWithdrawals(context.Context, int64) ([]model.Withdrawal, error)
}
type Balances struct{ store BalanceStore }

func NewBalances(store BalanceStore) *Balances { return &Balances{store: store} }
func (s *Balances) Get(ctx context.Context, id int64) (model.Balance, error) {
	return s.store.GetBalance(ctx, id)
}
func (s *Balances) Withdraw(ctx context.Context, id int64, number string, sum json.Number) error {
	if !ValidOrder(number) {
		return ErrOrderNumber
	}
	amount, err := model.Amount(sum, true)
	if err != nil {
		return ErrAmount
	}
	return s.store.Withdraw(ctx, id, number, amount)
}
func (s *Balances) History(ctx context.Context, id int64) ([]model.Withdrawal, error) {
	return s.store.ListWithdrawals(ctx, id)
}
