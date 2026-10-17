package integration

import (
	"context"
	"crypto/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Cyclov/gophermart/internal/storage"
	"github.com/Cyclov/gophermart/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func testDatabase(t *testing.T) (context.Context, *pgxpool.Pool, string) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_DSN to run database integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal("cannot create test pool")
	}
	t.Cleanup(admin.Close)
	schema := pgx.Identifier{"gophermart_test_" + strings.ToLower(rand.Text())}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal("cannot create test schema")
	}
	t.Cleanup(func() {
		cleanupCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if _, err := admin.Exec(cleanupCtx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error("cannot remove test schema")
		}
	})
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid test DSN")
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := storage.Connect(ctx, cfg.ConnString())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return ctx, pool, cfg.ConnString()
}

func TestPostgresIntegration(t *testing.T) {
	ctx, pool, dsn := testDatabase(t)
	var err error
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
	pool, err = storage.Connect(ctx, dsn)
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
	if _, err := provider.DownTo(ctx, 0); err != nil {
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
