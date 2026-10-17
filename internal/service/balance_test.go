package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Cyclov/gophermart/internal/model"
)

type balanceStub struct {
	called bool
	id     int64
	amount string
	err    error
}

func (s *balanceStub) GetBalance(ctx context.Context, id int64) (model.Balance, error) {
	s.id = id
	return model.Balance{Current: "1", Withdrawn: "0"}, s.err
}
func (s *balanceStub) Withdraw(ctx context.Context, id int64, order, amount string) error {
	s.called = true
	s.id = id
	s.amount = amount
	return s.err
}
func (s *balanceStub) ListWithdrawals(ctx context.Context, id int64) ([]model.Withdrawal, error) {
	s.id = id
	return nil, s.err
}
func TestWithdraw(t *testing.T) {
	for _, tt := range []struct {
		name, order, sum  string
		storeErr, wantErr error
		called            bool
	}{
		{"valid", "12345678903", "0.001", nil, nil, true},
		{"no funds", "12345678903", "1", ErrFunds, ErrFunds, true},
		{"number", "123", "1", nil, ErrOrderNumber, false},
		{"zero", "12345678903", "0", nil, ErrAmount, false},
		{"negative", "12345678903", "-1", nil, ErrAmount, false},
		{"invalid", "12345678903", "NaN", nil, ErrAmount, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := &balanceStub{err: tt.storeErr}
			err := NewBalances(s).Withdraw(context.Background(), 42, tt.order, json.Number(tt.sum))
			if !errors.Is(err, tt.wantErr) || s.called != tt.called {
				t.Fatalf("%v", err)
			}
			if s.called && (s.id != 42 || s.amount != tt.sum) {
				t.Fatal("incorrect store arguments")
			}
		})
	}
}
func TestBalanceAndHistory(t *testing.T) {
	s := new(balanceStub)
	balances := NewBalances(s)
	balance, err := balances.Get(context.Background(), 42)
	if err != nil || balance.Current != "1" || s.id != 42 {
		t.Fatal("balance")
	}
	if _, err := balances.History(context.Background(), 43); err != nil || s.id != 43 {
		t.Fatal("history")
	}
}
