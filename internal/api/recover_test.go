// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestRecover_PanicReturns500 covers REL-1: a panicking handler is caught,
// logged, and turned into a 500 rather than propagating.
func TestRecover_PanicReturns500(t *testing.T) {
	h := Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))
	rec := httptest.NewRecorder()
	// ServeHTTP must not re-panic.
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500", rec.Code)
	}
}

// TestRecover_PassesThrough confirms a normal handler is unaffected.
func TestRecover_PassesThrough(t *testing.T) {
	h := Recover(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != http.StatusTeapot {
		t.Fatalf("got %d, want 418", rec.Code)
	}
}

// TestRecoverLog_RecoversGoroutinePanic locks the primitive that startChat's
// inline /start forward goroutine relies on: `defer recoverLog(...)` must swallow
// a panic so a panicking spawned goroutine cannot crash the whole process (a
// spawned goroutine's panic is not covered by the request-chain Recover
// middleware). The contract gate enforces that startChat actually defers
// recoverLog; this proves recoverLog does recover. The outer deferred recover()
// runs after recoverLog (LIFO): it observes nil iff recoverLog already consumed
// the panic, and otherwise catches it itself so a regression fails cleanly here
// instead of taking down the test binary.
func TestRecoverLog_RecoversGoroutinePanic(t *testing.T) {
	recovered := make(chan bool, 1)
	go func() {
		defer func() { recovered <- (recover() == nil) }()
		defer recoverLog("startChat")
		panic("boom in forward goroutine")
	}()
	select {
	case ok := <-recovered:
		if !ok {
			t.Fatal("recoverLog did not recover the panic — a spawned goroutine panic would crash the process")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("goroutine did not complete")
	}
}
