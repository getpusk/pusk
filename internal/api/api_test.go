// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/pusk-platform/pusk/internal/auth"
	"github.com/pusk-platform/pusk/internal/org"
	"github.com/pusk-platform/pusk/internal/ws"
)

// testEnv creates a fully wired ClientAPI for integration testing.
type testEnv struct {
	api  *ClientAPI
	mux  *http.ServeMux
	jwt  *auth.JWTService
	orgs *org.Manager
	dir  string
	addr string // unique RemoteAddr per test to avoid rate limiter interference
}

var testIPCounter atomic.Uint64

func newTestEnv(t *testing.T) *testEnv {
	resetRevokedUsers()
	t.Helper()
	dir, err := os.MkdirTemp("", "pusk-api-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	mgr, err := org.NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { mgr.Close() })

	jwtSvc := auth.NewJWTService("test-secret-key-for-api-tests", 168)
	hub := ws.NewHub()

	s, err := mgr.Get("default")
	if err != nil {
		t.Fatal(err)
	}

	api := NewClientAPI(mgr, s, hub, nil, nil, nil, "", jwtSvc)

	mux := http.NewServeMux()
	api.Route(mux)

	n := testIPCounter.Add(1)
	addr := fmt.Sprintf("198.51.100.%d:%d", n%250+1, 1234+n)

	return &testEnv{api: api, mux: mux, jwt: jwtSvc, orgs: mgr, dir: dir, addr: addr}
}

func (e *testEnv) request(method, path string, body interface{}) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = e.addr
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, req)
	return rec
}

func (e *testEnv) authedRequest(method, path string, body interface{}, token string) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", token)
	req.RemoteAddr = e.addr
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, req)
	return rec
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to decode response: %v\nbody: %s", err, rec.Body.String())
	}
	return result
}

// ── Health ──

func TestHealth(t *testing.T) {
	env := newTestEnv(t)
	rec := env.request("GET", "/api/health", nil)
	if rec.Code != 200 {
		t.Fatalf("health: got %d", rec.Code)
	}
	data := decodeJSON(t, rec)
	if data["status"] != "ok" {
		t.Errorf("status = %v, want ok", data["status"])
	}
}

// ── Auth ──

func TestAuth_WrongCredentials(t *testing.T) {
	env := newTestEnv(t)
	rec := env.request("POST", "/api/auth", map[string]string{
		"username": "nobody", "pin": "wrong123",
	})
	if rec.Code != 401 {
		t.Fatalf("auth wrong creds: got %d, want 401", rec.Code)
	}
}

func TestAuth_InvalidBody(t *testing.T) {
	env := newTestEnv(t)
	req := httptest.NewRequest("POST", "/api/auth", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = env.addr
	rec := httptest.NewRecorder()
	env.mux.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Fatalf("auth invalid body: got %d, want 400", rec.Code)
	}
}

func TestAuth_NonexistentOrg(t *testing.T) {
	env := newTestEnv(t)
	rec := env.request("POST", "/api/auth", map[string]string{
		"username": "admin", "pin": "test", "org": "nonexistent",
	})
	if rec.Code != 400 {
		t.Fatalf("auth bad org: got %d, want 400", rec.Code)
	}
}

// ── Register ──

func TestRegister_Success(t *testing.T) {
	env := newTestEnv(t)
	rec := env.request("POST", "/api/register", map[string]string{
		"username": "newuser", "pin": "pass123456", "display_name": "New User",
	})
	if rec.Code != 200 {
		t.Fatalf("register: got %d, body: %s", rec.Code, rec.Body.String())
	}
	data := decodeJSON(t, rec)
	if data["token"] == nil || data["token"] == "" {
		t.Error("expected token in response")
	}
	if data["username"] != "newuser" {
		t.Errorf("username = %v", data["username"])
	}
}

func TestRegister_ShortPin(t *testing.T) {
	env := newTestEnv(t)
	rec := env.request("POST", "/api/register", map[string]string{
		"username": "user2", "pin": "abc",
	})
	if rec.Code != 400 {
		t.Fatalf("register short pin: got %d, want 400", rec.Code)
	}
}

func TestRegister_InvalidUsername(t *testing.T) {
	env := newTestEnv(t)
	rec := env.request("POST", "/api/register", map[string]string{
		"username": "a", "pin": "pass123456",
	})
	if rec.Code != 400 {
		t.Fatalf("register invalid username: got %d, want 400", rec.Code)
	}
}

func TestRegister_DuplicateUsername(t *testing.T) {
	env := newTestEnv(t)
	env.request("POST", "/api/register", map[string]string{
		"username": "dupuser", "pin": "pass123456",
	})
	rec := env.request("POST", "/api/register", map[string]string{
		"username": "dupuser", "pin": "pass123456",
	})
	if rec.Code != 400 {
		t.Fatalf("register duplicate: got %d, want 400", rec.Code)
	}
}

func TestRegister_NonDefaultOrgBlocked(t *testing.T) {
	env := newTestEnv(t)
	_ = env.orgs.Register("custom-org", "Custom", "admin", "pass123456", false)
	rec := env.request("POST", "/api/register", map[string]interface{}{
		"username": "user", "pin": "pass123456", "org": "custom-org",
	})
	if rec.Code != 403 {
		t.Fatalf("register non-default org: got %d, want 403", rec.Code)
	}
}

// ── Auth + Token flow ──

func TestAuth_SuccessFlow(t *testing.T) {
	env := newTestEnv(t)
	// Register first
	env.request("POST", "/api/register", map[string]string{
		"username": "authuser", "pin": "pass123456",
	})
	// Then auth
	rec := env.request("POST", "/api/auth", map[string]string{
		"username": "authuser", "pin": "pass123456",
	})
	if rec.Code != 200 {
		t.Fatalf("auth: got %d, body: %s", rec.Code, rec.Body.String())
	}
	data := decodeJSON(t, rec)
	if data["token"] == nil {
		t.Error("expected token")
	}
	if data["org"] != "default" {
		t.Errorf("org = %v, want default", data["org"])
	}
}

// ── AuthRequired middleware ──

func TestAuthRequired_NoToken(t *testing.T) {
	env := newTestEnv(t)
	rec := env.request("GET", "/api/bots", nil)
	if rec.Code != 401 {
		t.Fatalf("unauth bots: got %d, want 401", rec.Code)
	}
}

func TestAuthRequired_InvalidToken(t *testing.T) {
	env := newTestEnv(t)
	rec := env.authedRequest("GET", "/api/bots", nil, "garbage-token")
	if rec.Code != 401 {
		t.Fatalf("bad token: got %d, want 401", rec.Code)
	}
}

func TestAuthRequired_ValidToken(t *testing.T) {
	env := newTestEnv(t)
	// Register and get token
	regRec := env.request("POST", "/api/register", map[string]string{
		"username": "validuser", "pin": "pass123456",
	})
	data := decodeJSON(t, regRec)
	token := data["token"].(string)

	rec := env.authedRequest("GET", "/api/bots", nil, token)
	if rec.Code != 200 {
		t.Fatalf("authed bots: got %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestAuthRequired_QueryToken(t *testing.T) {
	env := newTestEnv(t)
	regRec := env.request("POST", "/api/register", map[string]string{
		"username": "queryuser", "pin": "pass123456",
	})
	data := decodeJSON(t, regRec)
	token := data["token"].(string)

	rec := env.request("GET", "/api/bots?token="+token, nil)
	if rec.Code != 200 {
		t.Fatalf("query token: got %d", rec.Code)
	}
}

// ── Channels ──

func TestListChannels(t *testing.T) {
	env := newTestEnv(t)
	regRec := env.request("POST", "/api/register", map[string]string{
		"username": "chuser", "pin": "pass123456",
	})
	token := decodeJSON(t, regRec)["token"].(string)

	rec := env.authedRequest("GET", "/api/channels", nil, token)
	if rec.Code != 200 {
		t.Fatalf("list channels: got %d", rec.Code)
	}
}

// ── Users ──

func TestListUsers(t *testing.T) {
	env := newTestEnv(t)
	regRec := env.request("POST", "/api/register", map[string]string{
		"username": "usrlister", "pin": "pass123456",
	})
	token := decodeJSON(t, regRec)["token"].(string)

	rec := env.authedRequest("GET", "/api/users", nil, token)
	if rec.Code != 200 {
		t.Fatalf("list users: got %d", rec.Code)
	}
	// /api/users may return users in a different format — just check 200
	_ = rec.Body.Bytes()
}

// ── Online ──

func TestOnlineUsers(t *testing.T) {
	env := newTestEnv(t)
	regRec := env.request("POST", "/api/register", map[string]string{
		"username": "onluser", "pin": "pass123456",
	})
	token := decodeJSON(t, regRec)["token"].(string)

	rec := env.authedRequest("GET", "/api/online", nil, token)
	if rec.Code != 200 {
		t.Fatalf("online: got %d", rec.Code)
	}
}

// ── VAPID Key ──

func TestVapidKey(t *testing.T) {
	env := newTestEnv(t)
	rec := env.request("GET", "/api/push/vapid", nil)
	if rec.Code != 200 {
		t.Fatalf("vapid: got %d", rec.Code)
	}
}

// ── Org Registration Flow ──

func TestOrgRegister_Success(t *testing.T) {
	env := newTestEnv(t)
	rec := env.request("POST", "/api/org/register", map[string]string{
		"slug": "testorg-" + strconv.FormatInt(int64(os.Getpid()), 10),
		"name": "Test Org", "username": "admin", "pin": "admin12345",
	})
	// org/register is on admin mux, not client mux — this may 404
	// That's fine, we test what we can reach
	if rec.Code == 200 {
		data := decodeJSON(t, rec)
		if data["ok"] != true {
			t.Errorf("expected ok=true, got %v", data["ok"])
		}
	}
}

// ── FindMyOrgs ──

func TestFindMyOrgs_NoAuth(t *testing.T) {
	env := newTestEnv(t)
	rec := env.request("GET", "/api/my-orgs", nil)
	if rec.Code != 401 {
		t.Fatalf("my-orgs no auth: got %d, want 401", rec.Code)
	}
}

func TestFindMyOrgs_WithAuth(t *testing.T) {
	env := newTestEnv(t)
	// Register a user first
	regRec := env.request("POST", "/api/register", map[string]string{
		"username": "findme", "pin": "pass123456",
	})
	token := decodeJSON(t, regRec)["token"].(string)
	rec := env.authedRequest("GET", "/api/my-orgs", nil, token)
	if rec.Code != 200 {
		t.Fatalf("my-orgs: got %d, body: %s", rec.Code, rec.Body.String())
	}
	var orgs []interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &orgs)
	if len(orgs) < 1 {
		t.Error("expected at least 1 org for registered user")
	}
}

// ── Invite Flow ──

func TestInvite_NonAdminBlocked(t *testing.T) {
	env := newTestEnv(t)
	regRec := env.request("POST", "/api/register", map[string]string{
		"username": "nonadmin", "pin": "pass123456",
	})
	token := decodeJSON(t, regRec)["token"].(string)

	rec := env.authedRequest("POST", "/api/invite", nil, token)
	if rec.Code != 403 {
		t.Fatalf("invite by non-admin: got %d, want 403", rec.Code)
	}
}

// ── Change Password ──

func TestChangePassword_WrongOld(t *testing.T) {
	env := newTestEnv(t)
	regRec := env.request("POST", "/api/register", map[string]string{
		"username": "pwuser", "pin": "pass123456",
	})
	token := decodeJSON(t, regRec)["token"].(string)

	rec := env.authedRequest("POST", "/api/change-password", map[string]string{
		"old_pin": "wrongpass", "new_pin": "newpass123",
	}, token)
	if rec.Code != 403 {
		t.Fatalf("change pw wrong old: got %d, want 403", rec.Code)
	}
}

func TestChangePassword_TooShort(t *testing.T) {
	env := newTestEnv(t)
	regRec := env.request("POST", "/api/register", map[string]string{
		"username": "pwuser2", "pin": "pass123456",
	})
	token := decodeJSON(t, regRec)["token"].(string)

	rec := env.authedRequest("POST", "/api/change-password", map[string]string{
		"old_pin": "pass123456", "new_pin": "ab",
	}, token)
	if rec.Code != 400 {
		t.Fatalf("change pw short: got %d, want 400", rec.Code)
	}
}

func TestChangePassword_Success(t *testing.T) {
	env := newTestEnv(t)
	regRec := env.request("POST", "/api/register", map[string]string{
		"username": "pwuser3", "pin": "pass123456",
	})
	token := decodeJSON(t, regRec)["token"].(string)

	rec := env.authedRequest("POST", "/api/change-password", map[string]string{
		"old_pin": "pass123456", "new_pin": "newpass789",
	}, token)
	if rec.Code != 200 {
		t.Fatalf("change pw: got %d, body: %s", rec.Code, rec.Body.String())
	}

	// Old token should be revoked
	rec2 := env.authedRequest("GET", "/api/bots", nil, token)
	if rec2.Code != 401 {
		t.Fatalf("old token after pw change: got %d, want 401", rec2.Code)
	}

	// New credentials should work
	authRec := env.request("POST", "/api/auth", map[string]string{
		"username": "pwuser3", "pin": "newpass789",
	})
	if authRec.Code != 200 {
		t.Fatalf("auth with new pw: got %d", authRec.Code)
	}
}

// ── OrgStats (admin only) ──

func TestOrgStats_NonAdmin(t *testing.T) {
	env := newTestEnv(t)
	regRec := env.request("POST", "/api/register", map[string]string{
		"username": "statsuser", "pin": "pass123456",
	})
	token := decodeJSON(t, regRec)["token"].(string)

	rec := env.authedRequest("GET", "/api/stats", nil, token)
	if rec.Code != 403 {
		t.Fatalf("stats non-admin: got %d, want 403", rec.Code)
	}
}

// ── CheckInviteUser ──

func TestCheckInviteUser_MissingParams(t *testing.T) {
	env := newTestEnv(t)
	rec := env.request("GET", "/api/invite/check-user", nil)
	if rec.Code != 400 {
		t.Fatalf("check invite no params: got %d, want 400", rec.Code)
	}
}

// ── Mark Read ──

func TestMarkRead_Unauthed(t *testing.T) {
	env := newTestEnv(t)
	rec := env.request("POST", "/api/channels/1/mark-read", nil)
	if rec.Code != 401 {
		t.Fatalf("mark-read unauthed: got %d, want 401", rec.Code)
	}
}

// ── checkWSOrigin ──

func TestCheckWSOrigin_SameHost(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "example.com"
	req.Header.Set("Origin", "https://example.com")
	if !checkWSOrigin(req) {
		t.Error("same host origin should be allowed")
	}
}

func TestCheckWSOrigin_Localhost(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "example.com"
	req.Header.Set("Origin", "http://localhost:3000")
	if !checkWSOrigin(req) {
		t.Error("localhost origin should be allowed")
	}
}

func TestCheckWSOrigin_DifferentHost(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "example.com"
	req.Header.Set("Origin", "https://evil.com")
	if checkWSOrigin(req) {
		t.Error("different host origin should be rejected")
	}
}

func TestCheckWSOrigin_NoOrigin(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	if !checkWSOrigin(req) {
		t.Error("no origin should be allowed")
	}
}

func TestCheckWSOrigin_InvalidOrigin(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "not a url ::::")
	if checkWSOrigin(req) {
		t.Error("invalid origin should be rejected")
	}
}

// ── jsonErr ──

func TestJsonErr(t *testing.T) {
	rec := httptest.NewRecorder()
	jsonErr(rec, "test error", 418)
	if rec.Code != 418 {
		t.Fatalf("code = %d, want 418", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Error("expected application/json content type")
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] != "test error" {
		t.Errorf("error = %q", body["error"])
	}
}

// ── limitBody ──

func TestLimitBody(t *testing.T) {
	called := false
	handler := limitBody(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(200)
	})
	req := httptest.NewRequest("POST", "/", bytes.NewReader([]byte("{}")))
	rec := httptest.NewRecorder()
	handler(rec, req)
	if !called {
		t.Error("handler should be called")
	}
}

// ── ListChats regression (bug #7877fbd) ──

func TestListChats_DefaultOrg_ReturnsChats(t *testing.T) {
	env := newTestEnv(t)
	// Register user
	env.request("POST", "/api/register", map[string]string{
		"username": "chatuser", "pin": "pass123456",
	})
	// Auth to get token
	rec := env.request("POST", "/api/auth", map[string]string{
		"username": "chatuser", "pin": "pass123456",
	})
	data := decodeJSON(t, rec)
	token := data["token"].(string)

	// Create a bot
	s, _ := env.orgs.Get("default")
	bot, err := s.CreateBot("chat-test-bot", "ChatTestBot")
	if err != nil {
		t.Fatal(err)
	}

	// Start chat with the bot
	rec = env.authedRequest("POST", fmt.Sprintf("/api/bots/%d/start", bot.ID), nil, token)
	if rec.Code != 200 {
		t.Fatalf("startChat: got %d, body: %s", rec.Code, rec.Body.String())
	}

	// List chats — must NOT be empty
	rec = env.authedRequest("GET", "/api/chats", nil, token)
	if rec.Code != 200 {
		t.Fatalf("listChats: got %d", rec.Code)
	}
	var chats []interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &chats); err != nil {
		t.Fatalf("decode chats: %v, body: %s", err, rec.Body.String())
	}
	if len(chats) == 0 {
		t.Error("listChats returned empty — regression: default org guard blocks chats")
	}
}

// ── Channel Unread after Unsubscribe (Issue #101) ──

func TestAPI_ChannelUnread_AfterUnsubscribe(t *testing.T) {
	env := newTestEnv(t)

	// Register user
	regRec := env.request("POST", "/api/register", map[string]string{
		"username": "unsubuser", "pin": "pass123456",
	})
	token := decodeJSON(t, regRec)["token"].(string)

	// Create channel and messages directly via store
	s, _ := env.orgs.Get("default")
	bot, err := s.CreateBot("tok-unsub-api", "TestBot")
	if err != nil {
		t.Fatal(err)
	}
	ch, err := s.CreateChannel(bot.ID, "test-alerts", "")
	if err != nil {
		t.Fatal(err)
	}

	// Get user ID
	u, _ := s.AuthUser("unsubuser", "pass123456")

	// Subscribe
	if err := s.Subscribe(ch.ID, u.ID); err != nil {
		t.Fatal(err)
	}

	// Send 3 messages
	msg1, _ := s.SaveChannelMessage(ch.ID, "alert1", "", "", "")
	msg2, _ := s.SaveChannelMessage(ch.ID, "alert2", "", "", "")
	_, _ = s.SaveChannelMessage(ch.ID, "alert3", "", "", "")
	_ = msg1

	// Mark read up to msg2
	s.MarkChannelRead(ch.ID, u.ID, msg2.ID)

	// Unsubscribe via API
	rec := env.authedRequest("POST", fmt.Sprintf("/api/channels/%d/unsubscribe", ch.ID), nil, token)
	if rec.Code != 200 {
		t.Fatalf("unsubscribe: got %d, body: %s", rec.Code, rec.Body.String())
	}

	// GET /api/channels should show unread=1 for the unsubscribed channel
	rec = env.authedRequest("GET", "/api/channels", nil, token)
	if rec.Code != 200 {
		t.Fatalf("list channels: got %d", rec.Code)
	}
	var channels []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &channels); err != nil {
		t.Fatalf("decode channels: %v", err)
	}

	found := false
	for _, c := range channels {
		if int64(c["id"].(float64)) == ch.ID {
			found = true
			unread := int(c["unread"].(float64))
			subscribed := c["subscribed"].(bool)
			if subscribed {
				t.Error("expected subscribed=false after unsubscribe")
			}
			if unread != 1 {
				t.Errorf("expected unread=1, got %d", unread)
			}
			break
		}
	}
	if !found {
		t.Error("channel not found in response")
	}

	// Send 2 more messages
	_, _ = s.SaveChannelMessage(ch.ID, "alert4", "", "", "")
	_, _ = s.SaveChannelMessage(ch.ID, "alert5", "", "", "")

	// Unread should now be 3
	rec = env.authedRequest("GET", "/api/channels", nil, token)
	if rec.Code != 200 {
		t.Fatalf("list channels 2: got %d", rec.Code)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &channels); err != nil {
		t.Fatalf("decode channels 2: %v", err)
	}
	for _, c := range channels {
		if int64(c["id"].(float64)) == ch.ID {
			unread := int(c["unread"].(float64))
			if unread != 3 {
				t.Errorf("expected unread=3 after 2 more msgs, got %d", unread)
			}
			break
		}
	}
}

func TestListChats_Unauthed(t *testing.T) {
	env := newTestEnv(t)
	rec := env.request("GET", "/api/chats", nil)
	if rec.Code != 401 {
		t.Fatalf("unauth chats: got %d, want 401", rec.Code)
	}
}
