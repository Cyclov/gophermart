package handler

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

func GzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
			body := r.Body
			reader, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, "Invalid gzip body", http.StatusBadRequest)
				return
			}
			defer reader.Close()
			defer body.Close()
			r.Body = io.NopCloser(reader)
		}

		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		writer := &gzipResponseWriter{ResponseWriter: w}
		next.ServeHTTP(writer, r)
		if writer.writer != nil {
			_ = writer.writer.Close()
		}
	})
}

type gzipResponseWriter struct {
	http.ResponseWriter
	writer *gzip.Writer
}

func (w *gzipResponseWriter) WriteHeader(status int) {
	w.enableCompression()
	w.ResponseWriter.WriteHeader(status)
}

func (w *gzipResponseWriter) Write(data []byte) (int, error) {
	if w.writer != nil {
		return w.writer.Write(data)
	}
	return w.ResponseWriter.Write(data)
}
func (w *gzipResponseWriter) enableCompression() {
	contentType := w.Header().Get("Content-Type")
	if strings.HasPrefix(contentType, "application/json") ||
		strings.HasPrefix(contentType, "text/html") {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Add("Vary", "Accept-Encoding")
		w.Header().Del("Content-Length")
		w.writer = gzip.NewWriter(w.ResponseWriter)
	}
}
