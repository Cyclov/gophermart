package storage

import (
	"context"
	"strings"
	"testing"
)

func TestConnectInvalidDSN(t *testing.T) {
	_, err := Connect(context.Background(), "postgres://user:private-password@%")
	if err == nil {
		t.Fatal("expected invalid DSN error")
	}
	if strings.Contains(err.Error(), "private-password") {
		t.Fatal("credential leaked")
	}
}
