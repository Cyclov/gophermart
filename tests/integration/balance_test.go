package integration

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/Cyclov/gophermart/internal/model"
	"github.com/Cyclov/gophermart/internal/service"
	"github.com/Cyclov/gophermart/internal/storage"
)

func TestBalanceConcurrencyIntegration(t *testing.T) {
	ctx, pool, _ := testDatabase(t)
	id, err := storage.NewUsers(pool).CreateUser(ctx, "balance", "hash")
	if err != nil {
		t.Fatal(err)
	}
	other, err := storage.NewUsers(pool).CreateUser(ctx, "other", "hash")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE accounts SET current=1 WHERE user_id=$1", id); err != nil {
		t.Fatal(err)
	}
	balances := service.NewBalances(storage.NewBalances(pool))
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- balances.Withdraw(ctx, id, "12345678903", json.Number("0.1")) }()
	}
	wg.Wait()
	close(errs)
	successful := 0
	for err := range errs {
		if err == nil {
			successful++
		} else if !errors.Is(err, service.ErrFunds) {
			t.Fatal(err)
		}
	}
	if successful != 10 {
		t.Fatalf("successes %d, want 10", successful)
	}
	balance, err := balances.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := model.Amount(balance.Current, false); err != nil {
		t.Fatal("negative balance")
	}
	var zero bool
	if err := pool.QueryRow(ctx, "SELECT current=0 AND withdrawn=1 FROM accounts WHERE user_id=$1", id).Scan(&zero); err != nil || !zero {
		t.Fatal("incorrect totals")
	}
	history, err := balances.History(ctx, id)
	if err != nil || len(history) != 10 {
		t.Fatal("history is not atomic")
	}
	hidden, err := balances.History(ctx, other)
	if err != nil || len(hidden) != 0 {
		t.Fatal("history leaked")
	}
	for i := 1; i < len(history); i++ {
		if history[i].ProcessedAt.After(history[i-1].ProcessedAt) {
			t.Fatal("history not sorted")
		}
	}
	if _, err := pool.Exec(ctx, "UPDATE accounts SET current=1 WHERE user_id=$1", id); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "ALTER TABLE withdrawals ADD CONSTRAINT test_reject CHECK (sum>10) NOT VALID"); err != nil {
		t.Fatal(err)
	}
	if err := balances.Withdraw(ctx, id, "12345678903", json.Number("0.1")); err == nil {
		t.Fatal("expected history insert failure")
	}
	var unchanged bool
	if err := pool.QueryRow(ctx, "SELECT current=1 AND withdrawn=1 FROM accounts WHERE user_id=$1", id).Scan(&unchanged); err != nil || !unchanged {
		t.Fatal("debit not rolled back")
	}
}
