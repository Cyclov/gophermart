package service

import (
	"context"
	"errors"

	"github.com/Cyclov/gophermart/internal/model"
)

var (
	ErrOrderFormat = errors.New("invalid order format")
	ErrOrderNumber = errors.New("invalid order number")
	ErrOrderOwner  = errors.New("order belongs to another user")
)

type OrderStore interface {
	AddOrder(context.Context, int64, string) (bool, error)
	ListOrders(context.Context, int64) ([]model.Order, error)
}

type Orders struct{ store OrderStore }

func NewOrders(store OrderStore) *Orders { return &Orders{store: store} }

func ValidOrder(number string) bool {
	if number == "" {
		return false
	}
	sum := 0
	double := false
	for i := len(number) - 1; i >= 0; i-- {
		if number[i] < '0' || number[i] > '9' {
			return false
		}
		digit := int(number[i] - '0')
		if double {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
		double = !double
	}
	return sum%10 == 0
}

func (s *Orders) Upload(ctx context.Context, id int64, number string) (bool, error) {
	if number == "" {
		return false, ErrOrderFormat
	}
	for i := 0; i < len(number); i++ {
		if number[i] < '0' || number[i] > '9' {
			return false, ErrOrderFormat
		}
	}
	if !ValidOrder(number) {
		return false, ErrOrderNumber
	}
	return s.store.AddOrder(ctx, id, number)
}
func (s *Orders) List(ctx context.Context, id int64) ([]model.Order, error) {
	return s.store.ListOrders(ctx, id)
}
