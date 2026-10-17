package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Cyclov/gophermart/internal/model"
)

type orderStoreStub struct {
	called bool
	id     int64
	number string
	added  bool
	err    error
	orders []model.Order
}

func (s *orderStoreStub) AddOrder(ctx context.Context, id int64, number string) (bool, error) {
	s.called = true
	s.id = id
	s.number = number
	return s.added, s.err
}
func (s *orderStoreStub) ListOrders(ctx context.Context, id int64) ([]model.Order, error) {
	s.id = id
	return s.orders, s.err
}
func TestValidOrder(t *testing.T) {
	for _, tt := range []struct {
		number string
		valid  bool
	}{{"12345678903", true}, {"0012345678903", true}, {"79927398713", true}, {"123", false}, {"", false}, {"12a", false}, {"１２", false}} {
		if got := ValidOrder(tt.number); got != tt.valid {
			t.Errorf("%q: %v", tt.number, got)
		}
	}
}
func TestUploadOrder(t *testing.T) {
	for _, tt := range []struct {
		name, number      string
		storeErr, wantErr error
		added, called     bool
	}{
		{"new", "12345678903", nil, nil, true, true},
		{"repeat", "12345678903", nil, nil, false, true},
		{"foreign", "12345678903", ErrOrderOwner, ErrOrderOwner, false, true},
		{"format", "abc", nil, ErrOrderFormat, false, false},
		{"empty", "", nil, ErrOrderFormat, false, false},
		{"checksum", "123", nil, ErrOrderNumber, false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			store := &orderStoreStub{added: tt.added, err: tt.storeErr}
			added, err := NewOrders(store).Upload(context.Background(), 42, tt.number)
			if !errors.Is(err, tt.wantErr) || added != tt.added || store.called != tt.called {
				t.Fatalf("result %v, %v", added, err)
			}
			if store.called && (store.id != 42 || store.number != tt.number) {
				t.Fatal("incorrect storage arguments")
			}
		})
	}
}
func TestListOrders(t *testing.T) {
	store := &orderStoreStub{orders: []model.Order{{Number: "12345678903", Status: "NEW"}}}
	got, err := NewOrders(store).List(context.Background(), 42)
	if err != nil || len(got) != 1 || store.id != 42 {
		t.Fatal("list not delegated")
	}
}
