// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package api

import (
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pusk-platform/pusk/internal/auth"
	"github.com/pusk-platform/pusk/internal/org"
	"github.com/pusk-platform/pusk/internal/store"
)

// AdminAPI handles admin and org-registration endpoints.
type AdminAPI struct {
	orgs       *org.Manager
	store      *store.Store
	jwt        *auth.JWTService
	adminToken string
}

// NewAdminAPI creates a new AdminAPI.
func NewAdminAPI(orgs *org.Manager, s *store.Store, jwt *auth.JWTService, adminToken string) *AdminAPI {
	return &AdminAPI{orgs: orgs, store: s, jwt: jwt, adminToken: adminToken}
}

// Route registers admin routes on the given mux.
func (a *AdminAPI) Route(mux *http.ServeMux) {
	mux.HandleFunc("POST /admin/bots", a.registerBot)
	mux.HandleFunc("POST /admin/channel", a.createChannel)
	mux.HandleFunc("DELETE /admin/channel/{channelID}", a.deleteChannel)
	mux.HandleFunc("PUT /admin/channel/{channelID}", a.renameChannel)
	mux.HandleFunc("PUT /admin/channel/{channelID}/bot", a.updateChannelBot)
	mux.HandleFunc("PUT /admin/bots/{botID}", a.renameBot)

	mux.HandleFunc("DELETE /admin/org/{slug}", a.deleteOrg)
	mux.HandleFunc("POST /admin/reset-password", a.resetPassword)
	mux.HandleFunc("POST /admin/set-role", a.adminSetRole)

	orgRL := NewRateLimiter(10, time.Minute)
	mux.HandleFunc("GET /api/org/info", a.orgInfo)
	mux.HandleFunc("POST /api/org/register", RateLimit(orgRL, a.registerOrg))
}

// getOrgStore resolves the org Store from ADMIN_TOKEN or JWT.
func (a *AdminAPI) getOrgStore(r *http.Request) (*store.Store, bool) {
	authHeader := r.Header.Get("Authorization")
	// Try ADMIN_TOKEN first (global admin → default org)
	if a.adminToken != "" && subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(authHeader, "Bearer ")), []byte(a.adminToken)) == 1 {
		return a.store, true
	}
	// Try JWT (org user → their org store)
	if a.jwt != nil && authHeader != "" {
		if claims, err := a.jwt.Validate(authHeader); err == nil {
			if s, err := a.orgs.Get(claims.OrgID); err == nil {
				return s, true
			}
		}
	}
	return nil, false
}

// requireAdmin checks that the request is from admin token or a JWT user with admin role.
func (a *AdminAPI) requireAdmin(r *http.Request, s *store.Store) bool {
	authHeader := r.Header.Get("Authorization")
	if a.adminToken != "" && subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(authHeader, "Bearer ")), []byte(a.adminToken)) == 1 {
		return true // global admin token
	}
	// JWT user — verify admin role
	if a.jwt != nil && authHeader != "" {
		if claims, err := a.jwt.Validate(authHeader); err == nil {
			return s.IsAdmin(claims.UserID)
		}
	}
	return false
}

func (a *AdminAPI) registerBot(w http.ResponseWriter, r *http.Request) {
	s, ok := a.getOrgStore(r)
	if !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if !a.requireAdmin(r, s) {
		http.Error(w, `{"error":"admin only"}`, http.StatusForbidden)
		return
	}
	var req struct {
		Token string `json:"token"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, "invalid request body", 400)
		return
	}
	b, err := s.CreateBot(req.Token, req.Name)
	if err != nil {
		jsonErr(w, "failed to create bot", 500)
		return
	}
	// Register token globally for Bot API routing
	claims, _ := a.jwt.Validate(r.Header.Get("Authorization"))
	if claims != nil {
		a.orgs.RegisterToken(req.Token, claims.OrgID)
	} else {
		a.orgs.RegisterToken(req.Token, "default")
	}
	slog.Info("bot registered", "bot", b.Name, "token_prefix", b.Token[:8])
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(b)
}

func (a *AdminAPI) createChannel(w http.ResponseWriter, r *http.Request) {
	s, ok := a.getOrgStore(r)
	if !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if !a.requireAdmin(r, s) {
		http.Error(w, `{"error":"admin only"}`, http.StatusForbidden)
		return
	}
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		BotID       int64  `json:"bot_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	// BUG-7: validate channel name
	if len(req.Name) < 1 || len(req.Name) > 64 {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "channel name must be 1-64 characters"})
		return
	}
	bots, _ := s.ListBots()
	if len(bots) == 0 {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "create a bot first"})
		return
	}
	var botID int64
	if req.BotID > 0 {
		found := false
		for _, b := range bots {
			if b.ID == req.BotID {
				botID = b.ID
				found = true
				break
			}
		}
		if !found {
			w.WriteHeader(400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "bot not found"})
			return
		}
	} else {
		botID = bots[0].ID
	}
	ch, err := s.CreateChannel(botID, req.Name, req.Description)
	if err != nil {
		errMsg := "Channel already exists / Канал уже существует"
		if !strings.Contains(err.Error(), "UNIQUE") {
			errMsg = err.Error()
		}
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": errMsg})
		return
	}
	// Auto-subscribe all org members to new channel (push ON by default)
	users, _ := s.ListUsers()
	for _, u := range users {
		_ = s.Subscribe(ch.ID, u.ID)
	}
	slog.Info("channel created", "channel", ch.Name)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "result": ch})
}

func (a *AdminAPI) deleteChannel(w http.ResponseWriter, r *http.Request) {
	s, ok := a.getOrgStore(r)
	if !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if !a.requireAdmin(r, s) {
		http.Error(w, `{"error":"admin only"}`, http.StatusForbidden)
		return
	}
	channelID, _ := strconv.ParseInt(r.PathValue("channelID"), 10, 64)
	// Protect #general from deletion
	if chs, err := s.ListChannels(); err == nil {
		for _, ch := range chs {
			if ch.ID == channelID && ch.Name == "general" {
				jsonErr(w, "cannot delete #general", 400)
				return
			}
		}
	}
	if err := s.DeleteChannel(channelID); err != nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	slog.Info("channel deleted", "channel_id", channelID)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

func (a *AdminAPI) renameChannel(w http.ResponseWriter, r *http.Request) {
	s, ok := a.getOrgStore(r)
	if !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if !a.requireAdmin(r, s) {
		http.Error(w, `{"error":"admin only"}`, http.StatusForbidden)
		return
	}
	channelID, _ := strconv.ParseInt(r.PathValue("channelID"), 10, 64)
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, "invalid request", 400)
		return
	}
	if len(req.Name) < 1 || len(req.Name) > 64 {
		jsonErr(w, "name must be 1-64 characters", 400)
		return
	}
	// Protect #general from rename
	if chs, err := s.ListChannels(); err == nil {
		for _, ch := range chs {
			if ch.ID == channelID && ch.Name == "general" {
				jsonErr(w, "cannot rename #general", 400)
				return
			}
		}
	}
	if err := s.RenameChannel(channelID, req.Name); err != nil {
		jsonErr(w, "rename failed", 500)
		return
	}
	slog.Info("channel renamed", "channel_id", channelID, "new_name", req.Name)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

func (a *AdminAPI) updateChannelBot(w http.ResponseWriter, r *http.Request) {
	s, ok := a.getOrgStore(r)
	if !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if !a.requireAdmin(r, s) {
		http.Error(w, `{"error":"admin only"}`, http.StatusForbidden)
		return
	}
	channelID, _ := strconv.ParseInt(r.PathValue("channelID"), 10, 64)
	var req struct {
		BotID int64 `json:"bot_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, "invalid request", 400)
		return
	}
	if req.BotID <= 0 {
		jsonErr(w, "bot_id required", 400)
		return
	}
	bots, _ := s.ListBots()
	found := false
	for _, b := range bots {
		if b.ID == req.BotID {
			found = true
			break
		}
	}
	if !found {
		jsonErr(w, "bot not found", 404)
		return
	}
	if err := s.UpdateChannelBot(channelID, req.BotID); err != nil {
		jsonErr(w, "update failed", 500)
		return
	}
	slog.Info("channel bot updated", "channel_id", channelID, "bot_id", req.BotID)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

func (a *AdminAPI) renameBot(w http.ResponseWriter, r *http.Request) {
	s, ok := a.getOrgStore(r)
	if !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if !a.requireAdmin(r, s) {
		http.Error(w, `{"error":"admin only"}`, http.StatusForbidden)
		return
	}
	botID, _ := strconv.ParseInt(r.PathValue("botID"), 10, 64)
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, "invalid request", 400)
		return
	}
	if len(req.Name) < 1 || len(req.Name) > 64 {
		jsonErr(w, "name must be 1-64 characters", 400)
		return
	}
	if err := s.RenameBot(botID, req.Name); err != nil {
		jsonErr(w, "rename failed", 500)
		return
	}
	slog.Info("bot renamed", "bot_id", botID, "new_name", req.Name)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

func (a *AdminAPI) isGlobalAdmin(r *http.Request) bool {
	if a.adminToken == "" {
		return false
	}
	authHeader := r.Header.Get("Authorization")
	return subtle.ConstantTimeCompare(
		[]byte(strings.TrimPrefix(authHeader, "Bearer ")),
		[]byte(a.adminToken),
	) == 1
}

func (a *AdminAPI) orgInfo(w http.ResponseWriter, r *http.Request) {
	orgs := a.orgs.List()
	userOrgs := 0
	for _, o := range orgs {
		if o.Slug != "default" {
			userOrgs++
		}
	}
	canCreate := a.orgs.CanCreateOrg()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"can_create_org": canCreate,
		"count":          userOrgs,
		"max":            a.orgs.MaxOrgs,
	})
}

func (a *AdminAPI) registerOrg(w http.ResponseWriter, r *http.Request) {
	// Enforce org limit unless caller has admin token
	if !a.orgs.CanCreateOrg() && !a.isGlobalAdmin(r) {
		jsonErr(w, "org registration disabled — limit reached", http.StatusForbidden)
		return
	}

	var req struct {
		Slug     string `json:"slug"`
		Name     string `json:"name"`
		Username string `json:"username"`
		Pin      string `json:"pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, err.Error(), 400)
		return
	}
	if req.Slug == "" || req.Username == "" || req.Pin == "" {
		http.Error(w, `{"error":"slug, username and pin required"}`, 400)
		return
	}
	if len(req.Pin) < 8 {
		http.Error(w, `{"error":"password must be at least 8 characters"}`, 400)
		return
	}
	if err := a.orgs.Register(req.Slug, req.Name, req.Username, req.Pin, a.isGlobalAdmin(r)); err != nil {
		jsonErr(w, err.Error(), 400)
		return
	}
	// Generate JWT for the new admin
	s, err := a.orgs.Get(req.Slug)
	if err != nil || s == nil {
		jsonErr(w, "org created but store unavailable", 500)
		return
	}
	user, err := s.AuthUser(req.Username, req.Pin)
	if err != nil || user == nil {
		jsonErr(w, "org created but auth failed", 500)
		return
	}
	tok, err := a.jwt.Generate(user.ID, req.Slug, req.Username)
	if err != nil {
		jsonErr(w, "org created but token generation failed", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":       true,
		"org":      req.Slug,
		"token":    tok,
		"user_id":  user.ID,
		"username": req.Username,
		"role":     "admin",
	})
	slog.Info("org registered", "slug", req.Slug, "admin", req.Username)
}

func (a *AdminAPI) resetPassword(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if a.adminToken == "" || subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(authHeader, "Bearer ")), []byte(a.adminToken)) != 1 {
		http.Error(w, `{"error":"admin token required"}`, http.StatusForbidden)
		return
	}
	var req struct {
		Org      string `json:"org"`
		Username string `json:"username"`
		NewPin   string `json:"new_pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Org == "" || req.Username == "" || req.NewPin == "" {
		http.Error(w, `{"error":"org, username and new_pin required"}`, 400)
		return
	}
	if len(req.NewPin) < 8 {
		http.Error(w, `{"error":"password must be at least 8 characters"}`, 400)
		return
	}
	s, err := a.orgs.Get(req.Org)
	if err != nil {
		jsonErr(w, "org not found", 404)
		return
	}
	if err := s.ResetPassword(req.Username, req.NewPin); err != nil {
		jsonErr(w, err.Error(), 400)
		return
	}
	// Revoke old tokens
	users, _ := s.ListUsers()
	for _, u := range users {
		if u.Username == req.Username {
			RevokeUser(req.Org, u.ID)
			break
		}
	}
	slog.Info("password reset", "org", req.Org, "username", req.Username)
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (a *AdminAPI) adminSetRole(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if a.adminToken == "" || subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(authHeader, "Bearer ")), []byte(a.adminToken)) != 1 {
		http.Error(w, `{"error":"admin token required"}`, http.StatusForbidden)
		return
	}
	var req struct {
		Org    string `json:"org"`
		UserID int64  `json:"user_id"`
		Role   string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Org == "" || req.UserID == 0 || (req.Role != "admin" && req.Role != "member") {
		http.Error(w, `{"error":"org, user_id and role (admin/member) required"}`, 400)
		return
	}
	s, err := a.orgs.Get(req.Org)
	if err != nil {
		jsonErr(w, "org not found", 404)
		return
	}
	_ = s.SetUserRole(req.UserID, req.Role)
	slog.Info("admin role set", "org", req.Org, "user_id", req.UserID, "role", req.Role)
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (a *AdminAPI) deleteOrg(w http.ResponseWriter, r *http.Request) {
	// Requires ADMIN_TOKEN (global admin only)
	authHeader := r.Header.Get("Authorization")
	if a.adminToken == "" || subtle.ConstantTimeCompare(
		[]byte(strings.TrimPrefix(authHeader, "Bearer ")),
		[]byte(a.adminToken),
	) != 1 {
		http.Error(w, `{"error":"admin token required"}`, http.StatusForbidden)
		return
	}

	slug := r.PathValue("slug")
	if slug == "" {
		jsonErr(w, "slug required", 400)
		return
	}
	if slug == "default" {
		jsonErr(w, "cannot delete default org", 400)
		return
	}

	if err := a.orgs.DeleteOrg(slug); err != nil {
		jsonErr(w, err.Error(), 404)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "deleted": slug})
}
