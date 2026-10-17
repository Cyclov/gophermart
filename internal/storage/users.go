package storage

import (
	"context"
	"errors"

	"github.com/Cyclov/gophermart/internal/service"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Users struct{ pool *pgxpool.Pool }

func NewUsers(pool *pgxpool.Pool) *Users { return &Users{pool: pool} }

func (s *Users) CreateUser(ctx context.Context, login, hash string) (int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, errors.New("cannot begin registration")
	}
	defer tx.Rollback(ctx)
	var id int64
	err = tx.QueryRow(ctx, "INSERT INTO users(login,password_hash) VALUES ($1,$2) RETURNING id", login, hash).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return 0, service.ErrLoginTaken
		}
		return 0, errors.New("cannot create user")
	}
	if _, err := tx.Exec(ctx, "INSERT INTO accounts(user_id) VALUES ($1)", id); err != nil {
		return 0, errors.New("cannot create account")
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, errors.New("cannot commit registration")
	}
	return id, nil
}

func (s *Users) FindUser(ctx context.Context, login string) (int64, string, error) {
	var id int64
	var hash string
	err := s.pool.QueryRow(ctx, "SELECT id,password_hash FROM users WHERE login=$1", login).Scan(&id, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", service.ErrCredentials
	}
	if err != nil {
		return 0, "", errors.New("cannot find user")
	}
	return id, hash, nil
}
