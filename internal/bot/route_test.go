// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package bot

import (
	"bytes"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
)

// ── botIPFromRequest ──

func TestBotIPFromRequest_Direct(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "203.0.113.5:12345"
	ip := botIPFromRequest(req)
	if ip != "203.0.113.5" {
		t.Errorf("ip = %q, want 203.0.113.5", ip)
	}
}

func TestBotIPFromRequest_LoopbackWithXFF(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "198.51.100.1, 203.0.113.2")
	ip := botIPFromRequest(req)
	if ip != "198.51.100.1" {
		t.Errorf("ip = %q, want 198.51.100.1 (first XFF)", ip)
	}
}

func TestBotIPFromRequest_PublicIgnoresXFF(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "203.0.113.5:12345"
	req.Header.Set("X-Forwarded-For", "198.51.100.1")
	ip := botIPFromRequest(req)
	if ip != "203.0.113.5" {
		t.Errorf("ip = %q, want 203.0.113.5 (public ignores XFF)", ip)
	}
}

// ── checkBotAuthRL ──

func TestCheckBotAuthRL_NoFailures(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "198.51.100.50:1234"
	if checkBotAuthRL(req, "unique-tok-nofail") {
		t.Error("should not be rate limited with no failures")
	}
}

func TestCheckBotAuthRL_UnderLimit(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "198.51.100.51:1234"
	req.Header.Set("X-Bot-Token", "under-limit-tok")
	for i := 0; i < 10; i++ {
		trackBotAuthFail(req)
	}
	if checkBotAuthRL(req, "under-limit-tok") {
		t.Error("should not be rate limited under 50 failures")
	}
}

// ── trackBotAuthFail ──

func TestTrackBotAuthFail_Records(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "198.51.100.52:1234"
	req.Header.Set("X-Bot-Token", "track-fail-tok")
	trackBotAuthFail(req)
	// Should not panic and should record
	if checkBotAuthRL(req, "track-fail-tok") {
		t.Error("single failure should not trigger rate limit")
	}
}

// The rate-limit warning must not carry the full X-Bot-Token: a valid token
// also lands here when the store lookup fails, and logs are shipped around.
func TestTrackBotAuthFail_LogsTokenPrefixOnly(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(prev)

	cases := []struct{ remote, tok, prefix string }{
		{"198.51.100.60:1234", "0123456789abcdef0123456789abcdef", "token_prefix=01234567"},
		{"198.51.100.61:1234", "abc", "token_prefix=abc"}, // shorter than the prefix: no panic
	}
	for _, c := range cases {
		buf.Reset()
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = c.remote
		req.Header.Set("X-Bot-Token", c.tok)
		for i := 0; i < 50; i++ {
			trackBotAuthFail(req)
		}
		out := buf.String()
		if !strings.Contains(out, "bot auth rate limit triggered") {
			t.Fatalf("token %q: warning not logged: %q", c.tok, out)
		}
		if len(c.tok) > 8 && strings.Contains(out, c.tok) {
			t.Errorf("full token in log: %q", out)
		}
		if !strings.Contains(out, c.prefix) {
			t.Errorf("want %s in log: %q", c.prefix, out)
		}
	}
}

// ── webhookAllowed ──

func TestWebhookAllowed_FirstCall(t *testing.T) {
	if !webhookAllowed("unique-webhook-tok-1") {
		t.Error("first call should be allowed")
	}
}

func TestWebhookAllowed_UnderLimit(t *testing.T) {
	tok := "under-limit-webhook-tok"
	for i := 0; i < 10; i++ {
		if !webhookAllowed(tok) {
			t.Errorf("call %d should be allowed", i)
		}
	}
}

// ── truncateStr ──

func TestTruncateStr_Short(t *testing.T) {
	if s := truncateStr("hello", 10); s != "hello" {
		t.Errorf("got %q, want hello", s)
	}
}

func TestTruncateStr_Exact(t *testing.T) {
	if s := truncateStr("hello", 5); s != "hello" {
		t.Errorf("got %q, want hello", s)
	}
}

func TestTruncateStr_Long(t *testing.T) {
	s := truncateStr("hello world", 5)
	if s != "hello..." {
		t.Errorf("got %q, want hello...", s)
	}
}

func TestTruncateStr_Unicode(t *testing.T) {
	s := truncateStr("привет мир", 6)
	if s != "привет..." {
		t.Errorf("got %q, want привет...", s)
	}
}
