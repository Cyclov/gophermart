package server

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestGracefulShutdown(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	entered, release := make(chan struct{}), make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(entered)
			<-release
			w.WriteHeader(http.StatusNoContent)
		}), slog.New(slog.NewTextHandler(io.Discard, nil)), time.Second)
	}()
	response := make(chan error, 1)
	go func() {
		client := &http.Client{Timeout: 3 * time.Second}
		resp, err := client.Get("http://" + listener.Addr().String())
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode != http.StatusNoContent {
				response <- context.Canceled
				return
			}
		}
		response <- err
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		close(release)
		t.Fatal("request did not enter handler")
	}
	cancel()
	select {
	case err := <-done:
		close(release)
		t.Fatalf("server stopped before request completed: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	select {
	case err := <-response:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("request timeout")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown timeout")
	}
}

func TestOccupiedAddress(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := Run(context.Background(), listener.Addr().String(), http.NotFoundHandler(), logger); err == nil {
		t.Fatal("expected listen error")
	}
}
