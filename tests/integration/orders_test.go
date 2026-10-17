package integration

import (
	"errors"
	"sync"
	"testing"

	"github.com/Cyclov/gophermart/internal/service"
	"github.com/Cyclov/gophermart/internal/storage"
)

func TestOrdersIntegration(t *testing.T) {
	ctx, pool, _ := testDatabase(t)
	users := storage.NewUsers(pool)
	first, err := users.CreateUser(ctx, "first", "hash")
	if err != nil {
		t.Fatal(err)
	}
	second, err := users.CreateUser(ctx, "second", "hash")
	if err != nil {
		t.Fatal(err)
	}
	orders := service.NewOrders(storage.NewOrders(pool))
	added, err := orders.Upload(ctx, first, "0012345678903")
	if err != nil || !added {
		t.Fatalf("upload %v", err)
	}
	added, err = orders.Upload(ctx, first, "0012345678903")
	if err != nil || added {
		t.Fatal("repeat upload")
	}
	if _, err = orders.Upload(ctx, second, "0012345678903"); !errors.Is(err, service.ErrOrderOwner) {
		t.Fatal("foreign order accepted")
	}
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := orders.Upload(ctx, first, "12345678903"); results <- err }()
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	got, err := orders.List(ctx, first)
	if err != nil || len(got) != 2 {
		t.Fatal("incorrect list")
	}
	if got[1].Number != "0012345678903" || got[1].Status != "NEW" || got[1].Accrual != nil {
		t.Fatal("order fields lost")
	}
	other, err := orders.List(ctx, second)
	if err != nil || len(other) != 0 {
		t.Fatal("foreign orders leaked")
	}
}
