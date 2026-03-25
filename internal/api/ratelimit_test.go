// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package api

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRateLimiter_FirstCallAllowed(t *testing.T) {
	rl := NewRateLimiter(5, time.Second)
	defer rl.Stop()

	if !rl.Allow("key1") {
		t.Error("first call should be allowed")
	}
}

func TestRateLimiter_ExhaustsTokens(t *testing.T) {
	rl := NewRateLimiter(3, time.Second)
	defer rl.Stop()

	// 3 tokens: first call uses 1, so calls 1,2,3 allowed, call 4 denied
	for i := 0; i < 3; i++ {
		if !rl.Allow("k") {
			t.Errorf("call %d should be allowed", i+1)
		}
	}
	if rl.Allow("k") {
		t.Error("4th call should be denied (tokens exhausted)")
	}
}

func TestRateLimiter_DifferentKeys(t *testing.T) {
	rl := NewRateLimiter(1, time.Second)
	defer rl.Stop()

	if !rl.Allow("a") {
		t.Error("first call for 'a' should pass")
	}
	if rl.Allow("a") {
		t.Error("second call for 'a' should fail")
	}
	// Different key should have its own bucket
	if !rl.Allow("b") {
		t.Error("first call for 'b' should pass")
	}
}

func TestRateLimiter_TokenRefill(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping refill test in -short mode")
	}
	// Use a very short interval for fast test
	rl := NewRateLimiter(2, 50*time.Millisecond)
	defer rl.Stop()

	// Exhaust tokens
	rl.Allow("r")
	rl.Allow("r")
	if rl.Allow("r") {
		t.Error("should be exhausted")
	}

	// Wait for refill
	time.Sleep(60 * time.Millisecond)

	if !rl.Allow("r") {
		t.Error("should be allowed after refill")
	}
}

func TestRateLimiter_TokensCappedAtRate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping cap test in -short mode")
	}
	rl := NewRateLimiter(2, 50*time.Millisecond)
	defer rl.Stop()

	rl.Allow("cap") // 1 token left

	// Wait long enough for multiple refills
	time.Sleep(150 * time.Millisecond)

	// Should have exactly rate=2 tokens (capped), not more
	if !rl.Allow("cap") {
		t.Error("call 1 should pass")
	}
	if !rl.Allow("cap") {
		t.Error("call 2 should pass")
	}
	if rl.Allow("cap") {
		t.Error("call 3 should fail (capped at 2)")
	}
}

func TestRateLimiter_Concurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrent test in -short mode")
	}
	rl := NewRateLimiter(100, time.Second)
	defer rl.Stop()

	var allowed atomic.Int64
	var wg sync.WaitGroup

	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if rl.Allow("concurrent") {
				allowed.Add(1)
			}
		}()
	}
	wg.Wait()

	n := allowed.Load()
	if n > 100 {
		t.Errorf("allowed %d, expected max 100", n)
	}
	if n < 90 {
		t.Errorf("allowed only %d, expected close to 100 (flaky?)", n)
	}
}

func TestRateLimiter_ZeroRate(t *testing.T) {
	rl := NewRateLimiter(0, time.Second)
	defer rl.Stop()

	// rate=0: first call sets tokens to 0-1=-1, but the logic is:
	// first call: tokens = rate - 1 = -1, return true (first-time bucket creation)
	// This is a known edge case. The limiter still grants the first request.
	// After that, everything is blocked.
	rl.Allow("zero") // first call always allowed
	if rl.Allow("zero") {
		t.Error("zero-rate limiter should block after first call")
	}
}

// ── clientIP ──

func TestClientIP_Direct(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "203.0.113.1:12345"
	if ip := clientIP(req); ip != "203.0.113.1" {
		t.Errorf("expected 203.0.113.1, got %s", ip)
	}
}

func TestClientIP_XFFFromLoopback(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.5, 172.16.0.1")
	if ip := clientIP(req); ip != "198.51.100.5" {
		t.Errorf("expected 198.51.100.5, got %s", ip)
	}
}

func TestClientIP_XFFFromPrivate(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "172.16.0.1:5555"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	if ip := clientIP(req); ip != "1.2.3.4" {
		t.Errorf("expected 1.2.3.4, got %s", ip)
	}
}

func TestClientIP_XFFIgnoredFromPublic(t *testing.T) {
	// X-Forwarded-For from a public IP should be ignored (spoofable)
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "203.0.113.1:9999"
	req.Header.Set("X-Forwarded-For", "198.51.100.99")
	if ip := clientIP(req); ip != "203.0.113.1" {
		t.Errorf("expected 203.0.113.1 (ignored XFF), got %s", ip)
	}
}

// ── RateLimit middleware ──

func TestRateLimitMiddleware_Returns429(t *testing.T) {
	rl := NewRateLimiter(1, time.Second)
	defer rl.Stop()

	handler := RateLimit(rl, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	// First request passes
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "203.0.113.1:1111"
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != 200 {
		t.Errorf("first request: got %d", rec.Code)
	}

	// Second request blocked
	rec = httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != 429 {
		t.Errorf("second request: got %d, expected 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") != "60" {
		t.Error("missing Retry-After header")
	}
}

// ── Cleanup ──

func TestRateLimiter_Cleanup(t *testing.T) {
	rl := NewRateLimiter(5, time.Second)
	defer rl.Stop()

	rl.Allow("stale")

	// Manually backdate the bucket
	rl.mu.Lock()
	rl.buckets["stale"].lastAdd = time.Now().Add(-15 * time.Minute)
	rl.mu.Unlock()

	rl.cleanup()

	rl.mu.Lock()
	_, exists := rl.buckets["stale"]
	rl.mu.Unlock()

	if exists {
		t.Error("stale bucket should be cleaned up")
	}
}

func TestRateLimiter_CleanupKeepsFresh(t *testing.T) {
	rl := NewRateLimiter(5, time.Second)
	defer rl.Stop()

	rl.Allow("fresh")
	rl.cleanup()

	rl.mu.Lock()
	_, exists := rl.buckets["fresh"]
	rl.mu.Unlock()

	if !exists {
		t.Error("fresh bucket should NOT be cleaned up")
	}
}
