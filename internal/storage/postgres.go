package storage

import (
	"context"
	"errors"
	"time"

	"github.com/Cyclov/gophermart/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, errors.New("invalid database connection settings")
	}
	cfg.ConnConfig.ConnectTimeout = 5 * time.Second
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, errors.New("cannot create database pool")
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, errors.New("cannot connect to database")
	}
	db := stdlib.OpenDBFromPool(pool)
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err == nil {
		_, err = provider.Up(ctx)
	}
	_ = db.Close()
	if err != nil {
		pool.Close()
		return nil, errors.New("cannot apply database migrations")
	}
	return pool, nil
}
