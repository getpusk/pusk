// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package api

import (
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// RateLimiter implements a simple in-memory token bucket per key (IP).
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rate     int // tokens per interval
	interval time.Duration
	stop     chan struct{}
}

type bucket struct {
	tokens  int
	lastAdd time.Time
}

func NewRateLimiter(rate int, interval time.Duration) *RateLimiter {
	rl := &RateLimiter{
		buckets:  make(map[string]*bucket),
		rate:     rate,
		interval: interval,
		stop:     make(chan struct{}),
	}
	// Cleanup stale buckets every 5 minutes
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				rl.cleanup()
			case <-rl.stop:
				return
			}
		}
	}()
	return rl
}

func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, ok := rl.buckets[key]
	now := time.Now()

	if !ok {
		rl.buckets[key] = &bucket{tokens: rl.rate - 1, lastAdd: now}
		return true
	}

	// Refill tokens based on elapsed time
	elapsed := now.Sub(b.lastAdd)
	refill := int(elapsed/rl.interval) * rl.rate
	if refill > 0 {
		b.tokens += refill
		if b.tokens > rl.rate {
			b.tokens = rl.rate
		}
		b.lastAdd = now
	}

	if b.tokens <= 0 {
		return false
	}
	b.tokens--
	return true
}

func (rl *RateLimiter) Stop() {
	close(rl.stop)
}

func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	cutoff := time.Now().Add(-10 * time.Minute)
	for k, b := range rl.buckets {
		if b.lastAdd.Before(cutoff) {
			delete(rl.buckets, k)
		}
	}
}

// RateLimit wraps a handler with rate limiting by client IP.
// Returns 429 Too Many Requests when limit exceeded.
func RateLimit(rl *RateLimiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip = r.RemoteAddr
		}
		if !rl.Allow(ip) {
			slog.Warn("rate limit hit", "client_ip", ip, "path", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"too many requests"}`))
			return
		}
		next(w, r)
	}
}
