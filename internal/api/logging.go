// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package api

import (
	"bufio"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/pusk-platform/pusk/internal/metrics"
)

// statusWriter wraps http.ResponseWriter to capture the status code.
// Implements http.Hijacker so WebSocket upgrades work through the middleware.
type statusWriter struct {
	http.ResponseWriter
	code int
}

func (w *statusWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, fmt.Errorf("ResponseWriter does not implement http.Hijacker")
}

var numericID = regexp.MustCompile(`/\d+`)

// normalizePath replaces numeric path segments with {id} to reduce cardinality.
func normalizePath(p string) string {
	return numericID.ReplaceAllString(p, "/{id}")
}

// RequestLogger is HTTP middleware that logs every request with structured fields.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		sw := &statusWriter{ResponseWriter: w, code: http.StatusOK}
		next.ServeHTTP(sw, r)

		duration := time.Since(start)
		path := normalizePath(r.URL.Path)

		ip := r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip = r.RemoteAddr
		}

		slog.Info("http request",
			"method", r.Method,
			"path", path,
			"status", sw.code,
			"duration_ms", duration.Milliseconds(),
			"client_ip", ip,
		)

		metrics.HTTPRequests.WithLabelValues(r.Method, path, strconv.Itoa(sw.code)).Inc()
		metrics.HTTPDuration.WithLabelValues(r.Method, path).Observe(duration.Seconds())
	})
}
