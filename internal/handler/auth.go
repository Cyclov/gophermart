package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	authjwt "github.com/Cyclov/gophermart/internal/auth"
	"github.com/Cyclov/gophermart/internal/service"
)

type AuthService interface {
	Register(context.Context, string, string) (string, error)
	Login(context.Context, string, string) (string, error)
}

func authEndpoint(auth AuthService, register bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Login    string `json:"login"`
			Password string `json:"password"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&input); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF || strings.TrimSpace(input.Login) == "" || input.Password == "" {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		var token string
		var err error
		if register {
			token, err = auth.Register(ctx, input.Login, input.Password)
		} else {
			token, err = auth.Login(ctx, input.Login, input.Password)
		}
		if errors.Is(err, service.ErrLoginTaken) {
			http.Error(w, "login already taken", http.StatusConflict)
			return
		}
		if errors.Is(err, service.ErrCredentials) {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "session", Value: token, Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteLaxMode, MaxAge: 86400, Expires: time.Now().Add(authjwt.TTL)})
		w.Header().Set("Authorization", "Bearer "+token)
		w.WriteHeader(http.StatusOK)
	}
}
