package storage

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Cyclov/gophermart/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
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

func TestPostgresIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_DSN to run PostgreSQL integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal("cannot create test pool")
	}
	defer admin.Close()
	schema := "gophermart_test_" + time.Now().Format("20060102150405000000000")
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal("cannot create test schema")
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, err := admin.Exec(cleanupCtx, "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Error("cannot remove test schema")
		}
	}()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("cannot parse test DSN")
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := Connect(ctx, cfg.ConnString())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { pool.Close() }()
	var userID int64
	if err := pool.QueryRow(ctx, "INSERT INTO users(login,password_hash) VALUES ('alice','hash') RETURNING id").Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO accounts(user_id,current) VALUES ($1,500.5)", userID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO orders(number,user_id) VALUES ('00123',$1)", userID); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct{ name, query, code string }{
		{"unique login", "INSERT INTO users(login,password_hash) VALUES ('alice','hash')", "23505"},
		{"global order uniqueness", "INSERT INTO orders(number,user_id) SELECT '00123',id FROM users", "23505"},
		{"foreign key", "INSERT INTO accounts(user_id) VALUES (999999)", "23503"},
		{"negative balance", "UPDATE accounts SET current=-1", "23514"},
		{"invalid status", "UPDATE orders SET status='REGISTERED'", "23514"},
		{"non finite balance", "UPDATE accounts SET current='NaN'::numeric", "23514"},
		{"zero withdrawal", "INSERT INTO withdrawals(user_id,order_number,sum) SELECT id,'00123',0 FROM users", "23514"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := pool.Exec(ctx, tt.query)
			pgErr, ok := err.(*pgconn.PgError)
			if !ok || pgErr.Code != tt.code {
				t.Fatalf("expected PostgreSQL constraint error %s", tt.code)
			}
		})
	}
	pool.Close()
	pool, err = Connect(ctx, cfg.ConnString())
	if err != nil {
		t.Fatal(err)
	}
	var balance, number string
	if err := pool.QueryRow(ctx, "SELECT current::text FROM accounts WHERE user_id=$1", userID).Scan(&balance); err != nil || balance != "500.5" {
		t.Fatal("balance did not persist exactly")
	}
	if err := pool.QueryRow(ctx, "SELECT number FROM orders WHERE user_id=$1", userID).Scan(&number); err != nil || number != "00123" {
		t.Fatal("order number did not persist")
	}
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Down(ctx); err != nil {
		t.Fatal(err)
	}
	var exists bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('users') IS NOT NULL").Scan(&exists); err != nil || exists {
		t.Fatal("down migration did not remove tables")
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
}
