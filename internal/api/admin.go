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

	orgRL := NewRateLimiter(10, time.Minute)
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
		http.Error(w, err.Error(), 400)
		return
	}
	b, err := s.CreateBot(req.Token, req.Name)
	if err != nil {
		http.Error(w, err.Error(), 500)
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
	json.NewEncoder(w).Encode(b)
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
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	bots, _ := s.ListBots()
	if len(bots) == 0 {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "create a bot first"})
		return
	}
	ch, err := s.CreateChannel(bots[0].ID, req.Name, req.Description)
	if err != nil {
		errMsg := "Channel already exists / Канал уже существует"
		if !strings.Contains(err.Error(), "UNIQUE") {
			errMsg = err.Error()
		}
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": errMsg})
		return
	}
	slog.Info("channel created", "channel", ch.Name)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "result": ch})
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
	if err := s.DeleteChannel(channelID); err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	slog.Info("channel deleted", "channel_id", channelID)
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

func (a *AdminAPI) registerOrg(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Slug     string `json:"slug"`
		Name     string `json:"name"`
		Username string `json:"username"`
		Pin      string `json:"pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, 400)
		return
	}
	if req.Slug == "" || req.Username == "" || req.Pin == "" {
		http.Error(w, `{"error":"slug, username and pin required"}`, 400)
		return
	}
	if len(req.Pin) < 6 {
		http.Error(w, `{"error":"password must be at least 6 characters"}`, 400)
		return
	}
	if err := a.orgs.Register(req.Slug, req.Name, req.Username, req.Pin); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, 400)
		return
	}
	// Generate JWT for the new admin
	s, _ := a.orgs.Get(req.Slug)
	user, _ := s.AuthUser(req.Username, req.Pin)
	tok, _ := a.jwt.Generate(user.ID, req.Slug, req.Username)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":       true,
		"org":      req.Slug,
		"token":    tok,
		"user_id":  user.ID,
		"username": req.Username,
	})
	slog.Info("org registered", "slug", req.Slug, "admin", req.Username)
}
