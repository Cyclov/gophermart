package server

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"
)

func Run(ctx context.Context, addr string, handler http.Handler, logger *slog.Logger) error {
	if ctx.Err() != nil {
		return nil
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return errors.New("cannot listen on HTTP address")
	}
	return serve(ctx, listener, handler, logger, 10*time.Second)
}

func serve(ctx context.Context, listener net.Listener, handler http.Handler, logger *slog.Logger, shutdownTimeout time.Duration) error {
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve(listener) }()
	logger.Info("HTTP server started", "addr", listener.Addr().String())
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return errors.New("HTTP server failed")
	case <-ctx.Done():
		logger.Info("HTTP server stopping")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			_ = srv.Close()
			<-done
			return errors.New("HTTP server shutdown timed out or failed")
		}
		err := <-done
		if !errors.Is(err, http.ErrServerClosed) {
			return errors.New("HTTP server failed")
		}
		logger.Info("HTTP server stopped")
		return nil
	}
}
