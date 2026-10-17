package accrual

import (
	"errors"
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
