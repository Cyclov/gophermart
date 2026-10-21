package handler

import (
	"compress/gzip"
	"io"
	"net/http"
	"strconv"
	"strings"
)

func GzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		encoding := strings.TrimSpace(r.Header.Get("Content-Encoding"))
		if encoding != "" && !strings.EqualFold(encoding, "identity") {
			if !strings.EqualFold(encoding, "gzip") {
				http.Error(w, "unsupported content encoding", http.StatusUnsupportedMediaType)
				return
			}
			original := r.Body
			reader, err := gzip.NewReader(original)
			if err != nil {
				http.Error(w, "invalid gzip body", http.StatusBadRequest)
				return
			}
			defer reader.Close()
			defer original.Close()
			r.Body = io.NopCloser(reader)
			r.ContentLength = -1
			r.Header.Del("Content-Encoding")
			r.Header.Del("Content-Length")
		}
		w.Header().Add("Vary", "Accept-Encoding")
		if !acceptsGzip(r.Header.Get("Accept-Encoding")) || r.Method == http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		writer := &gzipResponseWriter{ResponseWriter: w}
		defer func() {
			if writer.writer != nil {
				_ = writer.writer.Close()
			}
		}()
		next.ServeHTTP(writer, r)
	})
}

func acceptsGzip(header string) bool {
	wildcard := false
	for _, item := range strings.Split(header, ",") {
		parts := strings.Split(strings.TrimSpace(item), ";")
		coding := strings.TrimSpace(parts[0])
		quality := 1.0
		for _, param := range parts[1:] {
			key, value, ok := strings.Cut(strings.TrimSpace(param), "=")
			if ok && strings.EqualFold(key, "q") {
				var err error
				quality, err = strconv.ParseFloat(value, 64)
				if err != nil || quality < 0 || quality > 1 {
					quality = 0
				}
			}
		}
		if strings.EqualFold(coding, "gzip") {
			return quality > 0
		}
		if coding == "*" && quality > 0 {
			wildcard = true
		}
	}
	return wildcard
}

type gzipResponseWriter struct {
	http.ResponseWriter
	writer      *gzip.Writer
	wroteHeader bool
}

func (w *gzipResponseWriter) WriteHeader(status int) {
	if status >= 100 && status < 200 {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	contentType := w.Header().Get("Content-Type")
	if status != 204 && status != 304 && w.Header().Get("Content-Encoding") == "" && (strings.HasPrefix(contentType, "application/json") || strings.HasPrefix(contentType, "text/html")) {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Del("Content-Length")
		w.writer = gzip.NewWriter(w.ResponseWriter)
	}
	w.ResponseWriter.WriteHeader(status)
}
func (w *gzipResponseWriter) Write(body []byte) (int, error) {
	if !w.wroteHeader {
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", http.DetectContentType(body))
		}
		w.WriteHeader(http.StatusOK)
	}
	if w.writer != nil {
		return w.writer.Write(body)
	}
	return w.ResponseWriter.Write(body)
}
func (w *gzipResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
