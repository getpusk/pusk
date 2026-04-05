// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// ── normalizePath ──

func TestNormalizePath_NoIDs(t *testing.T) {
	if p := normalizePath("/api/health"); p != "/api/health" {
		t.Errorf("got %q, want /api/health", p)
	}
}

func TestNormalizePath_WithID(t *testing.T) {
	if p := normalizePath("/api/channels/42/messages"); p != "/api/channels/{id}/messages" {
		t.Errorf("got %q, want /api/channels/{id}/messages", p)
	}
}

func TestNormalizePath_MultipleIDs(t *testing.T) {
	if p := normalizePath("/admin/channel/5/msg/123"); p != "/admin/channel/{id}/msg/{id}" {
		t.Errorf("got %q", p)
	}
}

// ── statusWriter ──

func TestStatusWriter_Default200(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rec, code: http.StatusOK}
	if sw.code != 200 {
		t.Errorf("default code = %d, want 200", sw.code)
	}
}

func TestStatusWriter_WriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rec, code: http.StatusOK}
	sw.WriteHeader(404)
	if sw.code != 404 {
		t.Errorf("code = %d, want 404", sw.code)
	}
	if rec.Code != 404 {
		t.Errorf("underlying code = %d, want 404", rec.Code)
	}
}

func TestStatusWriter_Hijack_NotSupported(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rec, code: http.StatusOK}
	_, _, err := sw.Hijack()
	if err == nil {
		t.Error("expected error from Hijack on non-hijackable writer")
	}
}

// ── RequestLogger ──

func TestRequestLogger_SetsStatusCode(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
	})
	logged := RequestLogger(handler)
	req := httptest.NewRequest("GET", "/api/test", nil)
	rec := httptest.NewRecorder()
	logged.ServeHTTP(rec, req)
	if rec.Code != 201 {
		t.Errorf("code = %d, want 201", rec.Code)
	}
}

func TestRequestLogger_PassesThrough(t *testing.T) {
	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(200)
	})
	logged := RequestLogger(handler)
	req := httptest.NewRequest("POST", "/api/auth", nil)
	rec := httptest.NewRecorder()
	logged.ServeHTTP(rec, req)
	if !called {
		t.Error("handler should be called")
	}
}

// ── truncateText ──

func TestTruncateText_Short(t *testing.T) {
	if s := truncateText("hello", 10); s != "hello" {
		t.Errorf("got %q", s)
	}
}

func TestTruncateText_Exact(t *testing.T) {
	if s := truncateText("hello", 5); s != "hello" {
		t.Errorf("got %q", s)
	}
}

func TestTruncateText_Long(t *testing.T) {
	s := truncateText("hello world", 5)
	if s != "hello..." {
		t.Errorf("got %q, want hello...", s)
	}
}
