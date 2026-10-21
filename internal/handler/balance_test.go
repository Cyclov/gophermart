package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Cyclov/gophermart/internal/model"
	"github.com/Cyclov/gophermart/internal/service"
)

type balancesStub struct {
	err    error
	called bool
	id     int64
	amount json.Number
}

func (s *balancesStub) Get(ctx context.Context, id int64) (model.Balance, error) {
	s.called = true
	s.id = id
	return model.Balance{Current: "0", Withdrawn: "0"}, s.err
}
func (s *balancesStub) History(ctx context.Context, id int64) ([]model.Withdrawal, error) {
	s.called = true
	s.id = id
	return nil, s.err
}
func (s *balancesStub) Withdraw(ctx context.Context, id int64, order string, amount json.Number) error {
	s.called = true
	s.id = id
	s.amount = amount
	return s.err
}
func TestWithdrawalResponse(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		err        error
		status     int
		called     bool
	}{
		{"success", `{"order":"12345678903","sum":0.01}`, nil, 200, true},
		{"funds", `{"order":"12345678903","sum":0.01}`, service.ErrFunds, 402, true},
		{"order", `{"order":"bad","sum":0.01}`, service.ErrOrderNumber, 422, true},
		{"amount", `{"order":"12345678903","sum":0}`, service.ErrAmount, 400, true},
		{"database", `{"order":"12345678903","sum":0.01}`, errors.New("failed"), 500, true},
		{"json", `{`, nil, 400, false}, {"extra", `{} {}`, nil, 400, false}, {"missing", `{}`, nil, 400, false}, {"quoted", `{"sum":"1"}`, nil, 400, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := &balancesStub{err: tt.err}
			w := httptest.NewRecorder()
			withdrawalResponse(s, 42, w, httptest.NewRequest("POST", "/", strings.NewReader(tt.body)))
			if w.Code != tt.status || s.called != tt.called {
				t.Fatalf("code %d called %v", w.Code, s.called)
			}
			if s.called && s.id != 42 {
				t.Fatal("wrong identity")
			}
		})
	}
}

func TestBalanceReadResponses(t *testing.T) {
	for _, err := range []error{nil, errors.New("failed")} {
		s := &balancesStub{err: err}
		w := httptest.NewRecorder()
		balanceResponse(s, 42, w, httptest.NewRequest("GET", "/", nil))
		want := 200
		if err != nil {
			want = 500
		}
		if w.Code != want {
			t.Fatalf("balance: %d", w.Code)
		}
		w = httptest.NewRecorder()
		historyResponse(s, 42, w, httptest.NewRequest("GET", "/", nil))
		want = 204
		if err != nil {
			want = 500
		}
		if w.Code != want {
			t.Fatalf("history: %d", w.Code)
		}
	}
}

func TestBalanceHandlersRejectMissingIdentity(t *testing.T) {
	stub := &balancesStub{}
	for _, h := range []http.HandlerFunc{getBalance(stub), withdraw(stub), listWithdrawals(stub)} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
		if w.Code != http.StatusUnauthorized {
			t.Fatal("missing identity accepted")
		}
		if stub.called {
			t.Fatal("service called without identity")
		}
	}
}
