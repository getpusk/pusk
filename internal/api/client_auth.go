// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package api

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

// PIN auth lockout: 5 failed attempts = 15 min lockout
var authFailures sync.Map // key: "orgSlug:username" -> *failureInfo

type failureInfo struct {
	mu       sync.Mutex
	count    int
	lockedAt time.Time
}

func init() {
	go func() {
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			authFailures.Range(func(key, value interface{}) bool {
				f := value.(*failureInfo)
				f.mu.Lock()
				if time.Since(f.lockedAt) > 15*time.Minute {
					f.mu.Unlock()
					authFailures.Delete(key)
				} else {
					f.mu.Unlock()
				}
				return true
			})
		}
	}()
}

func (a *ClientAPI) auth(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Pin      string `json:"pin"`
		Org      string `json:"org"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, "invalid request body", 400)
		return
	}

	orgSlug := req.Org
	if orgSlug == "" {
		orgSlug = "default"
	}
	s, err := a.orgs.Get(orgSlug)
	if err != nil {
		jsonErr(w, "org not found", 400)
		return
	}

	// Check lockout
	lockKey := orgSlug + ":" + req.Username
	if fi, ok := authFailures.Load(lockKey); ok {
		f := fi.(*failureInfo)
		f.mu.Lock()
		locked := f.count >= 5 && time.Since(f.lockedAt) < 15*time.Minute
		f.mu.Unlock()
		if locked {
			jsonErr(w, "too many attempts, try in 15 minutes", 429)
			return
		}
	}

	user, err := s.AuthUser(req.Username, req.Pin)
	if err != nil {
		// Track failed attempt
		fi, _ := authFailures.LoadOrStore(lockKey, &failureInfo{})
		f := fi.(*failureInfo)
		f.mu.Lock()
		f.count++
		f.lockedAt = time.Now()
		f.mu.Unlock()
		if req.Org == "" {
			jsonErr(w, "invalid credentials — specify org / укажите организацию", 401)
		} else {
			jsonErr(w, "invalid credentials", 401)
		}
		return
	}
	// Clear failed attempts on success
	authFailures.Delete(lockKey)
	token, err := a.jwt.Generate(user.ID, orgSlug, user.Username)
	if err != nil {
		jsonErr(w, "token error", 500)
		return
	}
	role := "member"
	if s.IsAdmin(user.ID) {
		role = "admin"
	}
	slog.Info("auth success", "username", user.Username, "org", orgSlug, "role", role, "user_id", user.ID)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token": token, "user_id": user.ID, "username": user.Username, "org": orgSlug, "role": role, "display_name": user.DisplayName,
	})
}

func (a *ClientAPI) register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username    string `json:"username"`
		Pin         string `json:"pin"`
		DisplayName string `json:"display_name"`
		Org         string `json:"org"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, "invalid request body", 400)
		return
	}

	orgSlug := req.Org
	if orgSlug == "" {
		orgSlug = "default"
	}
	s, err := a.orgs.Get(orgSlug)
	if err != nil {
		jsonErr(w, "org not found", 400)
		return
	}

	// BUG-1: non-default orgs require invite link
	if orgSlug != "default" {
		jsonErr(w, "use invite link to join this organization", 403)
		return
	}

	if !regexp.MustCompile(`^[a-zA-Z0-9_-]{2,32}$`).MatchString(req.Username) {
		jsonErr(w, "username must be 2-32 alphanumeric characters", 400)
		return
	}
	if len(req.Pin) < 6 {
		jsonErr(w, "password must be at least 6 characters", 400)
		return
	}

	user, err := s.CreateUser(req.Username, req.Pin, req.DisplayName)
	if err != nil {
		// BUG-10: sanitize SQLite errors
		if strings.Contains(err.Error(), "UNIQUE") {
			jsonErr(w, "username already taken", 400)
		} else {
			jsonErr(w, "registration failed", 400)
		}
		return
	}
	token, _ := a.jwt.Generate(user.ID, orgSlug, req.Username)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token": token, "user_id": user.ID, "username": req.Username, "org": orgSlug, "role": "member", "display_name": req.DisplayName,
	})
}

// checkChatAccess verifies the authenticated user owns the given chat.
func (a *ClientAPI) checkChatAccess(w http.ResponseWriter, r *http.Request, chatID int64) bool {
	userID := UserIDFromCtx(r.Context())
	if userID == 0 {
		jsonErr(w, "unauthorized", 401)
		return false
	}
	ownerID, err := a.db(r).ChatUserID(chatID)
	if err != nil || ownerID != userID {
		jsonErr(w, "forbidden", 403)
		return false
	}
	return true
}

func (a *ClientAPI) createInvite(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromCtx(r.Context())
	if userID == 0 {
		jsonErr(w, "unauthorized", 401)
		return
	}
	s := a.db(r)
	// EDGE-5: only admins can create invites
	if !s.IsAdmin(userID) {
		jsonErr(w, "admin only", 403)
		return
	}
	b := make([]byte, 16)
	rand.Read(b)
	code := fmt.Sprintf("%x", b)

	if err := s.CreateInvite(code, 24*time.Hour); err != nil {
		jsonErr(w, "internal error", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"code": code, "url": "/invite/" + code})
}

func (a *ClientAPI) acceptInvite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code        string `json:"code"`
		Username    string `json:"username"`
		Pin         string `json:"pin"`
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, "invalid request body", 400)
		return
	}

	if req.Code == "" || req.Username == "" || req.Pin == "" {
		jsonErr(w, "code, username and pin required", 400)
		return
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9_-]{2,32}$`).MatchString(req.Username) {
		jsonErr(w, "username must be 2-32 alphanumeric characters", 400)
		return
	}
	if len(req.Pin) < 6 {
		jsonErr(w, "password must be at least 6 characters", 400)
		return
	}

	orgSlug := r.URL.Query().Get("org")
	if orgSlug == "" {
		orgSlug = "default"
	}
	s, err := a.orgs.Get(orgSlug)
	if err != nil {
		jsonErr(w, "org not found", 400)
		return
	}

	// Validate invite before creating user
	if err := s.ValidateInvite(req.Code); err != nil {
		jsonErr(w, err.Error(), 400)
		return
	}

	user, err := s.CreateUser(req.Username, req.Pin, req.DisplayName)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			jsonErr(w, "username already taken", 400)
		} else {
			jsonErr(w, "registration failed", 400)
		}
		return
	}

	// BUG-3: consume invite only after successful user creation
	if err := s.UseInvite(req.Code); err != nil {
		// User created but invite failed — unlikely but not fatal
		jsonErr(w, err.Error(), 400)
		return
	}

	// Auto-subscribe to all channels in org
	channels, _ := s.ListChannels()
	for _, ch := range channels {
		s.Subscribe(ch.ID, user.ID)
	}

	token, _ := a.jwt.Generate(user.ID, orgSlug, req.Username)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token": token, "user_id": user.ID, "username": req.Username, "org": orgSlug, "role": "member", "display_name": req.DisplayName,
	})
}

func (a *ClientAPI) changePassword(w http.ResponseWriter, r *http.Request) {
	s := a.db(r)
	var req struct {
		OldPin string `json:"old_pin"`
		NewPin string `json:"new_pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, "invalid request body", 400)
		return
	}
	if req.OldPin == "" || req.NewPin == "" {
		jsonErr(w, "old_pin and new_pin required", 400)
		return
	}
	if len(req.NewPin) < 6 {
		jsonErr(w, "password must be at least 6 characters", 400)
		return
	}
	claims := ClaimsFromCtx(r.Context())
	if claims == nil {
		jsonErr(w, "unauthorized", 401)
		return
	}
	_, err := s.AuthUser(claims.Username, req.OldPin)
	if err != nil {
		jsonErr(w, "wrong current password", 403)
		return
	}
	if err := s.ResetPassword(claims.Username, req.NewPin); err != nil {
		jsonErr(w, "failed to update password", 500)
		return
	}
	RevokeUser(claims.OrgID, claims.UserID)
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// myOrgs returns all organizations where the current user has an account.
func (a *ClientAPI) myOrgs(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromCtx(r.Context())
	if claims == nil {
		jsonErr(w, "unauthorized", 401)
		return
	}
	username := claims.Username
	if username == "guest" {
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	if a.orgs == nil {
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}

	type orgInfo struct {
		Slug string `json:"slug"`
		Name string `json:"name"`
		Role string `json:"role"`
	}
	var result []orgInfo
	for _, o := range a.orgs.List() {
		if o.Slug == "default" {
			continue
		}
		s, err := a.orgs.Get(o.Slug)
		if err != nil {
			continue
		}
		users, _ := s.ListUsers()
		for _, u := range users {
			if u.Username == username {
				role := "member"
				if s.IsAdmin(u.ID) {
					role = "admin"
				}
				result = append(result, orgInfo{Slug: o.Slug, Name: o.Name, Role: role})
				break
			}
		}
	}
	if result == nil {
		result = []orgInfo{}
	}
	json.NewEncoder(w).Encode(result)
}
