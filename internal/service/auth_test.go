package service

import (
	"context"
	"errors"
	"testing"
)

func TestPasswordHash(t *testing.T) {
	first, err := hashPassword("password")
	if err != nil {
		t.Fatal(err)
	}
	second, err := hashPassword("password")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("password salts must differ")
	}
	if !verifyPassword("password", first) {
		t.Fatal("correct password rejected")
	}
	if verifyPassword("wrong", first) || verifyPassword("password", "broken") {
		t.Fatal("invalid password accepted")
	}
}

type userStoreStub struct {
	id      int64
	hash    string
	err     error
	created bool
}

func (s *userStoreStub) CreateUser(ctx context.Context, login, hash string) (int64, error) {
	s.created = true
	s.hash = hash
	return s.id, s.err
}
func (s *userStoreStub) FindUser(ctx context.Context, login string) (int64, string, error) {
	return s.id, s.hash, s.err
}

type issuerStub struct {
	id  int64
	err error
}

func (s *issuerStub) Issue(id int64) (string, error) { s.id = id; return "issued-token", s.err }

func TestRegister(t *testing.T) {
	for _, storeErr := range []error{nil, ErrLoginTaken, errors.New("database failed")} {
		store := &userStoreStub{id: 42, err: storeErr}
		issuer := new(issuerStub)
		token, err := NewAuth(store, issuer).Register(context.Background(), "alice", "secret")
		if !errors.Is(err, storeErr) {
			t.Fatalf("unexpected error %v", err)
		}
		if !store.created || store.hash == "secret" || !verifyPassword("secret", store.hash) {
			t.Fatal("password not hashed")
		}
		if storeErr == nil {
			if token != "issued-token" || issuer.id != 42 {
				t.Fatal("token not issued for created user")
			}
		} else if issuer.id != 0 || token != "" {
			t.Fatal("token issued after storage failure")
		}
	}
}

func TestLogin(t *testing.T) {
	hash, err := hashPassword("secret")
	if err != nil {
		t.Fatal(err)
	}
	dbErr := errors.New("database failed")
	for _, tt := range []struct {
		name, password    string
		storeErr, wantErr error
	}{
		{"valid", "secret", nil, nil},
		{"wrong password", "wrong", nil, ErrCredentials},
		{"missing user", "secret", ErrCredentials, ErrCredentials},
		{"database failure", "secret", dbErr, dbErr},
	} {
		t.Run(tt.name, func(t *testing.T) {
			store := &userStoreStub{id: 42, hash: hash, err: tt.storeErr}
			issuer := new(issuerStub)
			token, err := NewAuth(store, issuer).Login(context.Background(), "alice", tt.password)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("unexpected error %v", err)
			}
			if tt.wantErr == nil {
				if token != "issued-token" || issuer.id != 42 {
					t.Fatal("wrong token identity")
				}
			} else if issuer.id != 0 || token != "" {
				t.Fatal("token issued after failed login")
			}
		})
	}
}
