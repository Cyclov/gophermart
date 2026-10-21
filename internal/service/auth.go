package service

import (
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
)

var (
	ErrLoginTaken  = errors.New("login already taken")
	ErrCredentials = errors.New("invalid credentials")
)

const iterations = 600000

type UserStore interface {
	CreateUser(context.Context, string, string) (int64, error)
	FindUser(context.Context, string) (int64, string, error)
}

type TokenIssuer interface{ Issue(int64) (string, error) }
type Auth struct {
	store  UserStore
	tokens TokenIssuer
}

func NewAuth(store UserStore, tokens TokenIssuer) *Auth { return &Auth{store: store, tokens: tokens} }

func (a *Auth) Register(ctx context.Context, login, password string) (string, error) {
	hash, err := hashPassword(password)
	if err != nil {
		return "", err
	}
	id, err := a.store.CreateUser(ctx, login, hash)
	if err != nil {
		return "", err
	}
	return a.tokens.Issue(id)
}

func (a *Auth) Login(ctx context.Context, login, password string) (string, error) {
	id, hash, err := a.store.FindUser(ctx, login)
	if err != nil && !errors.Is(err, ErrCredentials) {
		return "", err
	}
	if errors.Is(err, ErrCredentials) {
		_, _ = pbkdf2.Key(sha256.New, password, make([]byte, 16), iterations, 32)
		return "", ErrCredentials
	}
	if !verifyPassword(password, hash) {
		return "", ErrCredentials
	}
	return a.tokens.Issue(id)
}
func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, iterations, 32)
	if err != nil {
		return "", err
	}
	return "pbkdf2-sha256$" + strconv.Itoa(iterations) + "$" + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(key), nil
}

func verifyPassword(password, hash string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" || parts[1] != strconv.Itoa(iterations) {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil || len(salt) != 16 {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(expected) != 32 {
		return false
	}
	actual, err := pbkdf2.Key(sha256.New, password, salt, iterations, 32)
	return err == nil && subtle.ConstantTimeCompare(actual, expected) == 1
}
