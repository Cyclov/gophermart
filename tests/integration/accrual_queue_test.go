package integration

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/Cyclov/gophermart/internal/model"
	"github.com/Cyclov/gophermart/internal/storage"
)

func TestAccrualTransactionIntegration(t *testing.T) {
	ctx, pool, dsn := testDatabase(t)
	id, err := storage.NewUsers(pool).CreateUser(ctx, "accrual", "hash")
	if err != nil {
		t.Fatal(err)
	}
	orders := storage.NewOrders(pool)
	if _, err := orders.AddOrder(ctx, id, "12345678903"); err != nil {
		t.Fatal(err)
	}
	pending, err := orders.Pending(ctx, 100)
	if err != nil || len(pending) != 1 {
		t.Fatal("missing pending order")
	}
	if err := orders.Apply(ctx, model.Accrual{Number: pending[0], Status: "REGISTERED"}, 0); err != nil {
		t.Fatal(err)
	}
	list, err := orders.ListOrders(ctx, id)
	if err != nil || list[0].Status != "PROCESSING" {
		t.Fatal("REGISTERED not mapped")
	}
	restarted, err := storage.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	pending, err = storage.NewOrders(restarted).Pending(ctx, 100)
	if err != nil || len(pending) != 1 {
		t.Fatal("queue did not persist")
	}
	amount := json.Number("500.123456789")
	result := model.Accrual{Number: pending[0], Status: "PROCESSED", Amount: &amount}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- orders.Apply(ctx, result, time.Second) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var current string
	if err := pool.QueryRow(ctx, "SELECT current::text FROM accounts WHERE user_id=$1", id).Scan(&current); err != nil || current != "500.123456789" {
		t.Fatalf("double or imprecise credit: %s", current)
	}
	if err := orders.Apply(ctx, model.Accrual{Number: result.Number, Status: "INVALID"}, 0); err != nil {
		t.Fatal(err)
	}
	list, err = orders.ListOrders(ctx, id)
	if err != nil || list[0].Status != "PROCESSED" {
		t.Fatal("terminal order regressed")
	}
	pending, err = orders.Pending(ctx, 100)
	if err != nil || len(pending) != 0 {
		t.Fatal("terminal order still pending")
	}
}
