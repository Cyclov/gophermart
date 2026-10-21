package accrual

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestDecode(t *testing.T) {
	for _, tt := range []struct {
		body  string
		valid bool
	}{
		{`{"order":"123","status":"REGISTERED"}`, true},
		{`{"order":"123","status":"PROCESSING"}`, true},
		{`{"order":"123","status":"INVALID"}`, true},
		{`{"order":"123","status":"PROCESSED","accrual":0.001}`, true},
		{`{"order":"123","status":"PROCESSED"}`, true},
		{`{"order":"456","status":"PROCESSED"}`, false},
		{`{"order":"123","status":"UNKNOWN"}`, false},
		{`{"order":"123","status":"PROCESSED","accrual":-1}`, false},
		{`not json`, false},
	} {
		_, err := decode([]byte(tt.body), "123")
		if (err == nil) != tt.valid {
			t.Errorf("%s: %v", tt.body, err)
		}
		if err != nil && !errors.Is(err, ErrResponse) {
			t.Fatal(err)
		}
	}
}
func TestRetryAfter(t *testing.T) {
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	for _, tt := range []struct {
		value string
		want  time.Duration
	}{{"60", time.Minute}, {"0", 0}, {"invalid", time.Second}, {"Sat, 17 Oct 2026 12:01:00 GMT", time.Minute}, {"Sat, 17 Oct 2026 11:00:00 GMT", 0}} {
		if got := retryAfter(tt.value, now); got != tt.want {
			t.Errorf("%s: %v", tt.value, got)
		}
	}
}

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error { b.closed = true; return nil }

type brokenBody struct{}

func (brokenBody) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestClientGet(t *testing.T) {
	for _, tt := range []struct {
		name            string
		status          int
		body            string
		network, broken bool
		want            error
	}{
		{"success", 200, `{"order":"123","status":"PROCESSED","accrual":1.25}`, false, false, nil},
		{"pending", 204, "", false, false, ErrPending},
		{"rate limit", 429, "", false, false, nil},
		{"server error", 500, "", false, false, ErrTemporary},
		{"invalid json", 200, `broken`, false, false, ErrResponse},
		{"oversized", 200, strings.Repeat("x", (1<<20)+1), false, false, ErrResponse},
		{"read failure", 200, "", false, true, ErrTemporary},
		{"network", 0, "", true, false, ErrTemporary},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient("http://accrual.example")
			body := &trackedBody{Reader: strings.NewReader(tt.body)}
			if tt.broken {
				body.Reader = brokenBody{}
			}
			client.http.SetTransport(transportFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method != "GET" || r.URL.Path != "/api/orders/123" {
					t.Fatal("incorrect upstream request")
				}
				if tt.network {
					return nil, errors.New("network failure")
				}
				header := make(http.Header)
				header.Set("Retry-After", "5")
				return &http.Response{StatusCode: tt.status, Header: header, Body: body, Request: r}, nil
			}))
			result, err := client.Get(context.Background(), "123")
			if tt.status == 429 {
				var rate *RateLimitError
				if !errors.As(err, &rate) || rate.Delay != 5*time.Second {
					t.Fatal("missing rate limit")
				}
			} else if !errors.Is(err, tt.want) {
				t.Fatalf("%v, want %v", err, tt.want)
			}
			if !tt.network && !body.closed {
				t.Fatal("response body was not closed")
			}
			if tt.name == "success" && (result.Amount == nil || result.Amount.String() != "1.25") {
				t.Fatal("incorrect response")
			}
		})
	}
}
