// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/pusk-platform/pusk/internal/auth"
	"github.com/pusk-platform/pusk/internal/org"
)

type adminEnv struct {
	admin *AdminAPI
	mux   *http.ServeMux
	jwt   *auth.JWTService
	orgs  *org.Manager
	dir   string
	token string // ADMIN_TOKEN
	addr  string
}

func newAdminEnv(t *testing.T) *adminEnv {
	t.Helper()
	dir, err := os.MkdirTemp("", "pusk-admin-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	mgr, err := org.NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { mgr.Close() })

	jwtSvc := auth.NewJWTService("admin-test-secret", 168)

	s, err := mgr.Get("default")
	if err != nil {
		t.Fatal(err)
	}

	adminToken := "super-secret-admin-token"
	api := NewAdminAPI(mgr, s, jwtSvc, adminToken)

	mux := http.NewServeMux()
	api.Route(mux)

	n := testIPCounter.Add(1)
	addr := fmt.Sprintf("198.51.100.%d:%d", n%250+1, 5000+n)

	return &adminEnv{admin: api, mux: mux, jwt: jwtSvc, orgs: mgr, dir: dir, token: adminToken, addr: addr}
}

// doReq sends a request. Authorization header is set raw (no "Bearer " prefix)
// because JWTService.Validate expects raw token string.
func (e *adminEnv) doReq(method, path string, body interface{}, authToken string) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if authToken != "" {
		req.Header.Set("Authorization", authToken)
	}
	req.RemoteAddr = e.addr
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, req)
	return rec
}

// ── registerOrg ──

func TestAdminRegisterOrg_Success(t *testing.T) {
	env := newAdminEnv(t)
	rec := env.doReq("POST", "/api/org/register", map[string]string{
		"slug": "neworg", "name": "New Org", "username": "admin", "pin": "admin12345",
	}, "")
	if rec.Code != 200 {
		t.Fatalf("register org: got %d, body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["ok"] != true {
		t.Errorf("expected ok=true, got %v", resp["ok"])
	}
	if resp["org"] != "neworg" {
		t.Errorf("org = %v, want neworg", resp["org"])
	}
	if resp["token"] == nil || resp["token"] == "" {
		t.Error("expected JWT token in response")
	}
}

func TestAdminRegisterOrg_MissingFields(t *testing.T) {
	env := newAdminEnv(t)
	rec := env.doReq("POST", "/api/org/register", map[string]string{
		"slug": "x",
	}, "")
	if rec.Code != 400 {
		t.Fatalf("register org missing fields: got %d, want 400", rec.Code)
	}
}

func TestAdminRegisterOrg_ShortPin(t *testing.T) {
	env := newAdminEnv(t)
	rec := env.doReq("POST", "/api/org/register", map[string]string{
		"slug": "shortpin", "username": "admin", "pin": "abc",
	}, "")
	if rec.Code != 400 {
		t.Fatalf("register org short pin: got %d, want 400", rec.Code)
	}
}

func TestAdminRegisterOrg_Duplicate(t *testing.T) {
	env := newAdminEnv(t)
	env.doReq("POST", "/api/org/register", map[string]string{
		"slug": "duporg", "name": "Dup", "username": "admin", "pin": "admin12345",
	}, "")
	rec := env.doReq("POST", "/api/org/register", map[string]string{
		"slug": "duporg", "name": "Dup2", "username": "admin2", "pin": "admin12345",
	}, "")
	if rec.Code != 400 {
		t.Fatalf("register org duplicate: got %d, want 400", rec.Code)
	}
}

// ── registerBot ──

func TestAdminRegisterBot_WithToken(t *testing.T) {
	env := newAdminEnv(t)
	rec := env.doReq("POST", "/admin/bots", map[string]string{
		"token": "bot-tok-12345678", "name": "TestBot",
	}, env.token)
	if rec.Code != 200 {
		t.Fatalf("register bot: got %d, body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["name"] != "TestBot" {
		t.Errorf("name = %v, want TestBot", resp["name"])
	}
}

func TestAdminRegisterBot_Forbidden(t *testing.T) {
	env := newAdminEnv(t)
	rec := env.doReq("POST", "/admin/bots", map[string]string{
		"token": "tok-12345678abcd", "name": "Bot",
	}, "wrong-token")
	if rec.Code != 403 {
		t.Fatalf("register bot bad token: got %d, want 403", rec.Code)
	}
}

func TestAdminRegisterBot_NoAuth(t *testing.T) {
	env := newAdminEnv(t)
	rec := env.doReq("POST", "/admin/bots", map[string]string{
		"token": "tok-12345678abce", "name": "Bot",
	}, "")
	if rec.Code != 403 {
		t.Fatalf("register bot no auth: got %d, want 403", rec.Code)
	}
}

func TestAdminRegisterBot_WithJWT(t *testing.T) {
	env := newAdminEnv(t)
	// Register org to get JWT admin token
	rec := env.doReq("POST", "/api/org/register", map[string]string{
		"slug": "botorg", "name": "BotOrg", "username": "admin", "pin": "admin12345",
	}, "")
	var orgResp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &orgResp); err != nil {
		t.Fatal(err)
	}
	jwtToken := orgResp["token"].(string)

	rec = env.doReq("POST", "/admin/bots", map[string]string{
		"token": "jwt-bot-12345678", "name": "JWTBot",
	}, jwtToken)
	if rec.Code != 200 {
		t.Fatalf("register bot with JWT: got %d, body: %s", rec.Code, rec.Body.String())
	}
}

// ── createChannel ──

func TestAdminCreateChannel_Success(t *testing.T) {
	env := newAdminEnv(t)
	env.doReq("POST", "/admin/bots", map[string]string{
		"token": "ch-bot-12345678", "name": "ChBot",
	}, env.token)

	rec := env.doReq("POST", "/admin/channel", map[string]string{
		"name": "alerts", "description": "Alert channel",
	}, env.token)
	if rec.Code != 200 {
		t.Fatalf("create channel: got %d, body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["ok"] != true {
		t.Errorf("expected ok=true, got %v", resp["ok"])
	}
}

func TestAdminCreateChannel_NoBots(t *testing.T) {
	env := newAdminEnv(t)
	rec := env.doReq("POST", "/admin/channel", map[string]string{
		"name": "test",
	}, env.token)
	if rec.Code != 400 {
		t.Fatalf("create channel no bots: got %d, want 400", rec.Code)
	}
}

func TestAdminCreateChannel_Forbidden(t *testing.T) {
	env := newAdminEnv(t)
	rec := env.doReq("POST", "/admin/channel", map[string]string{
		"name": "test",
	}, "wrong")
	if rec.Code != 403 {
		t.Fatalf("create channel forbidden: got %d, want 403", rec.Code)
	}
}

func TestAdminCreateChannel_EmptyName(t *testing.T) {
	env := newAdminEnv(t)
	env.doReq("POST", "/admin/bots", map[string]string{
		"token": "en-bot-12345678", "name": "Bot",
	}, env.token)

	rec := env.doReq("POST", "/admin/channel", map[string]string{
		"name": "",
	}, env.token)
	if rec.Code != 400 {
		t.Fatalf("create channel empty name: got %d, want 400", rec.Code)
	}
}

func TestAdminCreateChannel_Duplicate(t *testing.T) {
	env := newAdminEnv(t)
	env.doReq("POST", "/admin/bots", map[string]string{
		"token": "dup-bot-1234567", "name": "Bot",
	}, env.token)
	env.doReq("POST", "/admin/channel", map[string]string{
		"name": "dupchan",
	}, env.token)

	rec := env.doReq("POST", "/admin/channel", map[string]string{
		"name": "dupchan",
	}, env.token)
	if rec.Code != 400 {
		t.Fatalf("create channel duplicate: got %d, want 400", rec.Code)
	}
}

// ── deleteChannel ──

func TestAdminDeleteChannel_Success(t *testing.T) {
	env := newAdminEnv(t)
	env.doReq("POST", "/admin/bots", map[string]string{
		"token": "del-bot-1234567", "name": "Bot",
	}, env.token)
	rec := env.doReq("POST", "/admin/channel", map[string]string{
		"name": "todelete",
	}, env.token)
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	result := resp["result"].(map[string]interface{})
	chID := fmt.Sprintf("%.0f", result["id"].(float64))

	rec = env.doReq("DELETE", "/admin/channel/"+chID, nil, env.token)
	var delResp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &delResp); err != nil {
		t.Fatal(err)
	}
	if delResp["ok"] != true {
		t.Errorf("expected ok=true after delete, got %v", delResp["ok"])
	}
}

func TestAdminDeleteChannel_ProtectGeneral(t *testing.T) {
	env := newAdminEnv(t)
	env.doReq("POST", "/admin/bots", map[string]string{
		"token": "gen-bot-1234567", "name": "Bot",
	}, env.token)
	rec := env.doReq("POST", "/admin/channel", map[string]string{
		"name": "general",
	}, env.token)
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	result := resp["result"].(map[string]interface{})
	chID := fmt.Sprintf("%.0f", result["id"].(float64))

	rec = env.doReq("DELETE", "/admin/channel/"+chID, nil, env.token)
	if rec.Code != 400 {
		t.Fatalf("delete general: got %d, want 400", rec.Code)
	}
}

func TestAdminDeleteChannel_Forbidden(t *testing.T) {
	env := newAdminEnv(t)
	rec := env.doReq("DELETE", "/admin/channel/1", nil, "bad-token")
	if rec.Code != 403 {
		t.Fatalf("delete channel forbidden: got %d, want 403", rec.Code)
	}
}

// ── renameChannel ──

func TestAdminRenameChannel_Success(t *testing.T) {
	env := newAdminEnv(t)
	env.doReq("POST", "/admin/bots", map[string]string{
		"token": "ren-bot-1234567", "name": "Bot",
	}, env.token)
	rec := env.doReq("POST", "/admin/channel", map[string]string{
		"name": "oldname",
	}, env.token)
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	result := resp["result"].(map[string]interface{})
	chID := fmt.Sprintf("%.0f", result["id"].(float64))

	rec = env.doReq("PUT", "/admin/channel/"+chID, map[string]string{
		"name": "newname",
	}, env.token)
	var renResp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &renResp); err != nil {
		t.Fatal(err)
	}
	if renResp["ok"] != true {
		t.Errorf("expected ok=true after rename, got %v", renResp["ok"])
	}
}

func TestAdminRenameChannel_ProtectGeneral(t *testing.T) {
	env := newAdminEnv(t)
	env.doReq("POST", "/admin/bots", map[string]string{
		"token": "rg-bot-12345678", "name": "Bot",
	}, env.token)
	rec := env.doReq("POST", "/admin/channel", map[string]string{
		"name": "general",
	}, env.token)
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	result := resp["result"].(map[string]interface{})
	chID := fmt.Sprintf("%.0f", result["id"].(float64))

	rec = env.doReq("PUT", "/admin/channel/"+chID, map[string]string{
		"name": "renamed",
	}, env.token)
	if rec.Code != 400 {
		t.Fatalf("rename general: got %d, want 400", rec.Code)
	}
}

func TestAdminRenameChannel_EmptyName(t *testing.T) {
	env := newAdminEnv(t)
	env.doReq("POST", "/admin/bots", map[string]string{
		"token": "rc-bot-12345678", "name": "Bot",
	}, env.token)
	rec := env.doReq("POST", "/admin/channel", map[string]string{
		"name": "torename",
	}, env.token)
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	result := resp["result"].(map[string]interface{})
	chID := fmt.Sprintf("%.0f", result["id"].(float64))

	rec = env.doReq("PUT", "/admin/channel/"+chID, map[string]string{
		"name": "",
	}, env.token)
	if rec.Code != 400 {
		t.Fatalf("rename channel empty: got %d, want 400", rec.Code)
	}
}

// ── renameBot ──

func TestAdminRenameBot_Success(t *testing.T) {
	env := newAdminEnv(t)
	rec := env.doReq("POST", "/admin/bots", map[string]string{
		"token": "rb-bot-12345678", "name": "OldBot",
	}, env.token)
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	botID := fmt.Sprintf("%.0f", resp["id"].(float64))

	rec = env.doReq("PUT", "/admin/bots/"+botID, map[string]string{
		"name": "NewBot",
	}, env.token)
	var renResp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &renResp); err != nil {
		t.Fatal(err)
	}
	if renResp["ok"] != true {
		t.Errorf("expected ok=true after rename, got %v", renResp["ok"])
	}
}

func TestAdminRenameBot_EmptyName(t *testing.T) {
	env := newAdminEnv(t)
	rec := env.doReq("POST", "/admin/bots", map[string]string{
		"token": "rbe-bot-1234567", "name": "Bot",
	}, env.token)
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	botID := fmt.Sprintf("%.0f", resp["id"].(float64))

	rec = env.doReq("PUT", "/admin/bots/"+botID, map[string]string{
		"name": "",
	}, env.token)
	if rec.Code != 400 {
		t.Fatalf("rename bot empty: got %d, want 400", rec.Code)
	}
}

func TestAdminRenameBot_Forbidden(t *testing.T) {
	env := newAdminEnv(t)
	rec := env.doReq("PUT", "/admin/bots/1", map[string]string{
		"name": "x",
	}, "bad-token")
	if rec.Code != 403 {
		t.Fatalf("rename bot forbidden: got %d, want 403", rec.Code)
	}
}

// ── resetPassword ──

func TestAdminResetPassword_Success(t *testing.T) {
	env := newAdminEnv(t)
	env.doReq("POST", "/api/org/register", map[string]string{
		"slug": "rporg", "name": "RP", "username": "user1", "pin": "oldpass123",
	}, "")

	rec := env.doReq("POST", "/admin/reset-password", map[string]interface{}{
		"org": "rporg", "username": "user1", "new_pin": "newpass789",
	}, env.token)
	if rec.Code != 200 {
		t.Fatalf("reset password: got %d, body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["ok"] != true {
		t.Errorf("expected ok=true, got %v", resp["ok"])
	}
}

func TestAdminResetPassword_NotAdminToken(t *testing.T) {
	env := newAdminEnv(t)
	rec := env.doReq("POST", "/admin/reset-password", map[string]interface{}{
		"org": "default", "username": "x", "new_pin": "newpass789",
	}, "wrong-token")
	if rec.Code != 403 {
		t.Fatalf("reset password wrong token: got %d, want 403", rec.Code)
	}
}

func TestAdminResetPassword_MissingFields(t *testing.T) {
	env := newAdminEnv(t)
	rec := env.doReq("POST", "/admin/reset-password", map[string]interface{}{
		"org": "default",
	}, env.token)
	if rec.Code != 400 {
		t.Fatalf("reset password missing fields: got %d, want 400", rec.Code)
	}
}

func TestAdminResetPassword_ShortPin(t *testing.T) {
	env := newAdminEnv(t)
	rec := env.doReq("POST", "/admin/reset-password", map[string]interface{}{
		"org": "default", "username": "x", "new_pin": "ab",
	}, env.token)
	if rec.Code != 400 {
		t.Fatalf("reset password short pin: got %d, want 400", rec.Code)
	}
}

func TestAdminResetPassword_BadOrg(t *testing.T) {
	env := newAdminEnv(t)
	rec := env.doReq("POST", "/admin/reset-password", map[string]interface{}{
		"org": "nonexistent", "username": "x", "new_pin": "newpass789",
	}, env.token)
	if rec.Code != 404 {
		t.Fatalf("reset password bad org: got %d, want 404", rec.Code)
	}
}

// ── adminSetRole ──

func TestAdminSetRole_Success(t *testing.T) {
	env := newAdminEnv(t)
	rec := env.doReq("POST", "/api/org/register", map[string]string{
		"slug": "srorg", "name": "SR", "username": "user1", "pin": "pass123456",
	}, "")
	var orgResp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &orgResp); err != nil {
		t.Fatal(err)
	}
	userID := orgResp["user_id"].(float64)

	rec = env.doReq("POST", "/admin/set-role", map[string]interface{}{
		"org": "srorg", "user_id": userID, "role": "member",
	}, env.token)
	if rec.Code != 200 {
		t.Fatalf("set role: got %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminSetRole_NotAdminToken(t *testing.T) {
	env := newAdminEnv(t)
	rec := env.doReq("POST", "/admin/set-role", map[string]interface{}{
		"org": "default", "user_id": 1, "role": "admin",
	}, "wrong-token")
	if rec.Code != 403 {
		t.Fatalf("set role wrong token: got %d, want 403", rec.Code)
	}
}

func TestAdminSetRole_MissingFields(t *testing.T) {
	env := newAdminEnv(t)
	rec := env.doReq("POST", "/admin/set-role", map[string]interface{}{
		"org": "default",
	}, env.token)
	if rec.Code != 400 {
		t.Fatalf("set role missing fields: got %d, want 400", rec.Code)
	}
}

func TestAdminSetRole_InvalidRole(t *testing.T) {
	env := newAdminEnv(t)
	rec := env.doReq("POST", "/admin/set-role", map[string]interface{}{
		"org": "default", "user_id": 1, "role": "superuser",
	}, env.token)
	if rec.Code != 400 {
		t.Fatalf("set role invalid role: got %d, want 400", rec.Code)
	}
}

func TestAdminSetRole_BadOrg(t *testing.T) {
	env := newAdminEnv(t)
	rec := env.doReq("POST", "/admin/set-role", map[string]interface{}{
		"org": "nonexistent", "user_id": 1, "role": "admin",
	}, env.token)
	if rec.Code != 404 {
		t.Fatalf("set role bad org: got %d, want 404", rec.Code)
	}
}

// ── NewAdminAPI ──

func TestNewAdminAPI(t *testing.T) {
	env := newAdminEnv(t)
	if env.admin == nil {
		t.Fatal("expected non-nil AdminAPI")
	}
}
