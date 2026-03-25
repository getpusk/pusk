// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package bot

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ── extractTokenMethod ──

func TestExtractTokenMethod_Standard(t *testing.T) {
	token, method, ok := extractTokenMethod("/bot/mytoken123/sendMessage")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if token != "mytoken123" {
		t.Errorf("token = %q", token)
	}
	if method != "sendMessage" {
		t.Errorf("method = %q", method)
	}
}

func TestExtractTokenMethod_TelegramNative(t *testing.T) {
	token, method, ok := extractTokenMethod("/botTOKEN123/getMe")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if token != "TOKEN123" {
		t.Errorf("token = %q", token)
	}
	if method != "getMe" {
		t.Errorf("method = %q", method)
	}
}

func TestExtractTokenMethod_NoMethod(t *testing.T) {
	_, _, ok := extractTokenMethod("/bot/token_only")
	if ok {
		t.Error("expected ok=false when no method")
	}
}

func TestExtractTokenMethod_EmptyPath(t *testing.T) {
	_, _, ok := extractTokenMethod("")
	if ok {
		t.Error("expected ok=false for empty path")
	}
}

func TestExtractTokenMethod_JustBot(t *testing.T) {
	_, _, ok := extractTokenMethod("/bot")
	if ok {
		t.Error("expected ok=false for /bot only")
	}
}

func TestExtractTokenMethod_JustBotSlash(t *testing.T) {
	_, _, ok := extractTokenMethod("/bot/")
	if ok {
		t.Error("expected ok=false for /bot/ (empty token)")
	}
}

func TestExtractTokenMethod_ColonInToken(t *testing.T) {
	// Telegram tokens contain colons: 123456:ABC-DEF
	token, method, ok := extractTokenMethod("/bot/123456:ABC-DEF/sendMessage")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if token != "123456:ABC-DEF" {
		t.Errorf("token = %q", token)
	}
	if method != "sendMessage" {
		t.Errorf("method = %q", method)
	}
}

func TestExtractTokenMethod_MethodWithSlash(t *testing.T) {
	// If method contains slashes, everything after second slash is the method
	token, method, ok := extractTokenMethod("/bot/tok/sub/path")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if token != "tok" {
		t.Errorf("token = %q", token)
	}
	if method != "sub/path" {
		t.Errorf("method = %q, expected sub/path", method)
	}
}

// ── TelegramCompat middleware ──

func TestTelegramCompat_Rewrite(t *testing.T) {
	var gotPath string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
	})

	handler := TelegramCompat(inner)
	req := httptest.NewRequest("POST", "/botMYTOKEN/sendMessage", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if gotPath != "/bot/MYTOKEN/sendMessage" {
		t.Errorf("expected rewritten path /bot/MYTOKEN/sendMessage, got %q", gotPath)
	}
}

func TestTelegramCompat_AlreadyStandard(t *testing.T) {
	var gotPath string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
	})

	handler := TelegramCompat(inner)
	req := httptest.NewRequest("POST", "/bot/TOKEN/sendMessage", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if gotPath != "/bot/TOKEN/sendMessage" {
		t.Errorf("path should stay unchanged, got %q", gotPath)
	}
}

func TestTelegramCompat_NonBotPath(t *testing.T) {
	var gotPath string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
	})

	handler := TelegramCompat(inner)
	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if gotPath != "/api/health" {
		t.Errorf("non-bot path should be unchanged, got %q", gotPath)
	}
}

func TestTelegramCompat_JustBot(t *testing.T) {
	// "/bot" alone (len=4) should NOT trigger rewrite — the condition is len(p) > 4
	var gotPath string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
	})

	handler := TelegramCompat(inner)
	req := httptest.NewRequest("GET", "/bot", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if gotPath != "/bot" {
		t.Errorf("expected /bot unchanged, got %q", gotPath)
	}
}

// ── unwrapMarkup ──

func TestUnwrapMarkup_Nil(t *testing.T) {
	if got := unwrapMarkup(nil); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestUnwrapMarkup_RawObject(t *testing.T) {
	raw := json.RawMessage(`{"inline_keyboard":[]}`)
	got := unwrapMarkup(raw)
	if got != `{"inline_keyboard":[]}` {
		t.Errorf("got %q", got)
	}
}

func TestUnwrapMarkup_StringEncoded(t *testing.T) {
	// PTB sends reply_markup as a JSON-encoded string: "\"{ ... }\""
	inner := `{"inline_keyboard":[[{"text":"OK","callback_data":"ok"}]]}`
	encoded, _ := json.Marshal(inner)
	raw := json.RawMessage(encoded)

	got := unwrapMarkup(raw)
	if got != inner {
		t.Errorf("expected unwrapped JSON object, got %q", got)
	}
}

func TestUnwrapMarkup_EmptyString(t *testing.T) {
	raw := json.RawMessage(`""`)
	got := unwrapMarkup(raw)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestUnwrapMarkup_Array(t *testing.T) {
	raw := json.RawMessage(`[1,2,3]`)
	got := unwrapMarkup(raw)
	if got != `[1,2,3]` {
		t.Errorf("got %q", got)
	}
}

func TestUnwrapMarkup_Number(t *testing.T) {
	raw := json.RawMessage(`42`)
	got := unwrapMarkup(raw)
	if got != `42` {
		t.Errorf("got %q", got)
	}
}

// ── isHex ──

func TestIsHex(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"", false},
		{"abcdef0123456789", true},
		{"ABCDEF", true},
		{"0123456789abcdefABCDEF", true},
		{"xyz", false},
		{"12g4", false},
		{"12 34", false},
	}
	for _, tc := range tests {
		if got := isHex(tc.input); got != tc.want {
			t.Errorf("isHex(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

// ── formToMap ──

func TestFormToMap_Types(t *testing.T) {
	req := httptest.NewRequest("POST", "/test", nil)
	req.Form = map[string][]string{
		"chat_id": {"42"},
		"text":    {"hello"},
		"flag":    {"true"},
	}
	m := formToMap(req)
	if v, ok := m["chat_id"].(int64); !ok || v != 42 {
		t.Errorf("chat_id: %v (%T)", m["chat_id"], m["chat_id"])
	}
	if v, ok := m["text"].(string); !ok || v != "hello" {
		t.Errorf("text: %v", m["text"])
	}
	if v, ok := m["flag"].(bool); !ok || v != true {
		t.Errorf("flag: %v", m["flag"])
	}
}

// ── truncate ──

func TestTruncate(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Errorf("got %q", got)
	}
	if got := truncate("this is a longer string", 10); got != "this is a ..." {
		t.Errorf("got %q", got)
	}
	if got := truncate("", 5); got != "" {
		t.Errorf("got %q", got)
	}
}
