package config

import (
	"errors"
	"flag"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Setenv("JWT_SECRET", strings.Repeat("s", 32))
	tests := []struct {
		name    string
		vars    map[string]string
		args    []string
		want    Config
		wantErr string
	}{
		{name: "defaults", vars: map[string]string{"DATABASE_DSN": "db", "ACCRUAL_SYSTEM_ADDRESS": "http://localhost:8081"}, want: Config{strings.Repeat("s", 32), "localhost:8080", "db", "http://localhost:8081"}},
		{name: "environment", vars: map[string]string{"RUN_ADDRESS": ":9000", "DATABASE_DSN": "env-db", "ACCRUAL_SYSTEM_ADDRESS": "https://env.example"}, want: Config{strings.Repeat("s", 32), ":9000", "env-db", "https://env.example"}},
		{name: "flags only", args: []string{"-a", ":9001", "-d", "flag-db", "-r", "http://flag.example"}, want: Config{strings.Repeat("s", 32), ":9001", "flag-db", "http://flag.example"}},
		{name: "flags win", vars: map[string]string{"RUN_ADDRESS": "bad", "DATABASE_DSN": "env-db", "ACCRUAL_SYSTEM_ADDRESS": "bad"}, args: []string{"-a", ":9002", "-d", "flag-db", "-r", "https://flag.example"}, want: Config{strings.Repeat("s", 32), ":9002", "flag-db", "https://flag.example"}},
		{name: "partial override", vars: map[string]string{"RUN_ADDRESS": ":9000", "DATABASE_DSN": "env-db", "ACCRUAL_SYSTEM_ADDRESS": "http://env.example"}, args: []string{"-a", ":9003"}, want: Config{strings.Repeat("s", 32), ":9003", "env-db", "http://env.example"}},
		{name: "missing database", wantErr: "DATABASE_DSN"},
		{name: "missing accrual", args: []string{"-d", "db"}, wantErr: "ACCRUAL_SYSTEM_ADDRESS"},
		{name: "empty explicit flag", vars: map[string]string{"DATABASE_DSN": "env-db"}, args: []string{"-d="}, wantErr: "DATABASE_DSN"},
		{name: "invalid address", args: []string{"-a", "localhost"}, wantErr: "RUN_ADDRESS"},
		{name: "invalid port", args: []string{"-a", ":65536"}, wantErr: "port"},
		{name: "non numeric port", args: []string{"-a", ":http"}, wantErr: "port"},
		{name: "invalid URL", args: []string{"-d", "db", "-r", "ftp://example.org"}, wantErr: "ACCRUAL_SYSTEM_ADDRESS"},
		{name: "relative URL", args: []string{"-d", "db", "-r", "/accrual"}, wantErr: "ACCRUAL_SYSTEM_ADDRESS"},
		{name: "unknown flag", args: []string{"-unknown=private-password"}, wantErr: "invalid flags"},
		{name: "missing flag value", args: []string{"-d"}, wantErr: "invalid flags"},
		{name: "positional argument", args: []string{"private-password"}, wantErr: "positional"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, key := range []string{"RUN_ADDRESS", "DATABASE_DSN", "ACCRUAL_SYSTEM_ADDRESS"} {
				t.Setenv(key, tt.vars[key])
			}
			got, err := Load(tt.args)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}
				if strings.Contains(err.Error(), "private-password") {
					t.Fatal("error leaked a credential")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestLoadHelp(t *testing.T) {
	for _, arg := range []string{"-h", "-help"} {
		_, err := Load([]string{arg})
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("%s: got %v", arg, err)
		}
	}
}

func TestJWTSecret(t *testing.T) {
	t.Setenv("DATABASE_DSN", "db")
	t.Setenv("RUN_ADDRESS", ":8080")
	t.Setenv("ACCRUAL_SYSTEM_ADDRESS", "http://localhost:8081")
	for _, secret := range []string{"", "short"} {
		t.Setenv("JWT_SECRET", secret)
		_, err := Load(nil)
		if err == nil || !strings.Contains(err.Error(), "JWT_SECRET") {
			t.Fatal("invalid secret accepted")
		}
	}
}
