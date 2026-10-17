package handler

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGzipResponse(t *testing.T) {
	for _, tt := range []struct {
		name, accept, contentType string
		compressed                bool
	}{
		{"json", "gzip", "application/json", true},
		{"html", "gzip", "text/html; charset=utf-8", true},
		{"no negotiation", "", "application/json", false},
		{"plain text", "gzip", "text/plain", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			payload := `{"value":42}`
			h := GzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				w.Header().Set("Content-Length", "12")
				w.WriteHeader(http.StatusOK)
				_, _ = io.WriteString(w, payload)
			}))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Accept-Encoding", tt.accept)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			body := w.Body.Bytes()
			if got := w.Header().Get("Content-Encoding") == "gzip"; got != tt.compressed {
				t.Fatalf("compression = %v, want %v", got, tt.compressed)
			}
			if tt.compressed {
				if w.Header().Get("Content-Length") != "" {
					t.Fatal("original content length retained")
				}
				if !strings.Contains(w.Header().Get("Vary"), "Accept-Encoding") {
					t.Fatal("missing Vary header")
				}
				reader, err := gzip.NewReader(bytes.NewReader(body))
				if err != nil {
					t.Fatal(err)
				}
				defer reader.Close()
				body, err = io.ReadAll(reader)
				if err != nil {
					t.Fatal(err)
				}
			}
			if string(body) != payload {
				t.Fatalf("body = %q", body)
			}
		})
	}
}

func TestGzipRequest(t *testing.T) {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	_, _ = io.WriteString(writer, "00123")
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name   string
		body   []byte
		status int
	}{
		{"valid", compressed.Bytes(), http.StatusNoContent},
		{"invalid", []byte("not gzip"), http.StatusBadRequest},
	} {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			h := GzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatal(err)
				}
				if string(body) != "00123" {
					t.Fatalf("body = %q", body)
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(tt.body))
			req.Header.Set("Content-Encoding", "gzip")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != tt.status {
				t.Fatalf("status = %d, want %d", w.Code, tt.status)
			}
			if called != (tt.status == http.StatusNoContent) {
				t.Fatal("unexpected handler invocation")
			}
		})
	}
}

func TestGzipNegotiation(t *testing.T) {
	for _, tt := range []struct {
		header string
		want   bool
	}{{"gzip", true}, {"br, gzip;q=0.5", true}, {"gzip;q=0", false}, {"*;q=1", true}, {"*;q=1,gzip;q=0", false}, {"xgzip", false}, {"gzip;q=bad", false}, {"", false}} {
		if got := acceptsGzip(tt.header); got != tt.want {
			t.Errorf("%s: %v", tt.header, got)
		}
	}
}
func TestGzipImplicitHeaderAndEmptyResponse(t *testing.T) {
	for _, status := range []int{200, 204, 304} {
		h := GzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if status == 200 {
				_, _ = w.Write([]byte(`{}`))
				w.WriteHeader(500)
			} else {
				w.WriteHeader(status)
			}
		}))
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != status {
			t.Fatal("status overwritten")
		}
		if status != 200 {
			if w.Body.Len() != 0 || w.Header().Get("Content-Encoding") != "" {
				t.Fatal("compressed empty response")
			}
			continue
		}
		reader, err := gzip.NewReader(w.Body)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := io.ReadAll(reader)
		reader.Close()
		if err != nil || string(decoded) != "{}" {
			t.Fatal("implicit header response broken")
		}
	}
}
