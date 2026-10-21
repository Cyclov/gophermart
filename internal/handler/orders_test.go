package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Cyclov/gophermart/internal/model"
	"github.com/Cyclov/gophermart/internal/service"
)

type ordersStub struct {
	err    error
	added  bool
	list   []model.Order
	called bool
	id     int64
	number string
}

func (s *ordersStub) Upload(ctx context.Context, id int64, number string) (bool, error) {
	s.called = true
	s.id = id
	s.number = number
	return s.added, s.err
}
func (s *ordersStub) List(ctx context.Context, id int64) ([]model.Order, error) {
	s.called = true
	s.id = id
	return s.list, s.err
}
func TestUploadOrderResponse(t *testing.T) {
	for _, tt := range []struct {
		name, contentType, body string
		err                     error
		added                   bool
		status                  int
		called                  bool
	}{
		{"new", "text/plain", " 12345678903\n", nil, true, 202, true},
		{"repeat", "text/plain", "12345678903", nil, false, 200, true},
		{"format", "text/plain", "x", service.ErrOrderFormat, false, 400, true},
		{"checksum", "text/plain", "123", service.ErrOrderNumber, false, 422, true},
		{"owner", "text/plain", "12345678903", service.ErrOrderOwner, false, 409, true},
		{"database", "text/plain", "12345678903", errors.New("failed"), false, 500, true},
		{"content type", "application/json", "{}", nil, false, 400, false},
		{"too large", "text/plain", strings.Repeat("1", (64<<10)+1), nil, false, 400, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := &ordersStub{err: tt.err, added: tt.added}
			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/", strings.NewReader(tt.body))
			r.Header.Set("Content-Type", tt.contentType)
			uploadOrderResponse(s, 42, w, r)
			if w.Code != tt.status || s.called != tt.called {
				t.Fatalf("status %d, called %v", w.Code, s.called)
			}
			if s.called && s.id != 42 {
				t.Fatal("identity changed")
			}
			if tt.name == "new" && s.number != "12345678903" {
				t.Fatal("body not trimmed")
			}
		})
	}
}
func TestListOrdersResponse(t *testing.T) {
	for _, tt := range []struct {
		list   []model.Order
		err    error
		status int
	}{{nil, nil, 204}, {[]model.Order{{Number: "00123", Status: "NEW"}}, nil, 200}, {nil, errors.New("failed"), 500}} {
		s := &ordersStub{list: tt.list, err: tt.err}
		w := httptest.NewRecorder()
		listOrdersResponse(s, 42, w, httptest.NewRequest("GET", "/", nil))
		if w.Code != tt.status || s.id != 42 {
			t.Fatal("incorrect list response")
		}
		if w.Code == 204 && w.Body.Len() != 0 {
			t.Fatal("204 body")
		}
	}
}
func TestOrderHandlersRejectMissingIdentity(t *testing.T) {
	stub := &ordersStub{}
	for _, h := range []http.HandlerFunc{uploadOrder(stub), listOrders(stub)} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
		if w.Code != 401 {
			t.Fatal("missing identity accepted")
		}
		if stub.called {
			t.Fatal("service called without identity")
		}
	}
}
func TestWriteJSONError(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, make(chan int))
	if w.Code != 500 {
		t.Fatal("encoding error ignored")
	}
}
