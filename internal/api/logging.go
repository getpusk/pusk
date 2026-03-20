// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package api

import (
	"log/slog"
	"net/http"
	"regexp"
	"time"
)

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	code int
}

func (w *statusWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
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

		ip := r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip = r.RemoteAddr
		}

		slog.Info("http request",
			"method", r.Method,
			"path", normalizePath(r.URL.Path),
			"status", sw.code,
			"duration_ms", time.Since(start).Milliseconds(),
			"client_ip", ip,
		)
	})
}
