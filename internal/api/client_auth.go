// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package api

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func (a *ClientAPI) auth(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Pin      string `json:"pin"`
		Org      string `json:"org"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	orgSlug := req.Org
	if orgSlug == "" {
		orgSlug = "default"
	}
	s, err := a.orgs.Get(orgSlug)
	if err != nil {
		jsonErr(w, "org not found", 400)
		return
	}

	user, err := s.AuthUser(req.Username, req.Pin)
	if err != nil {
		jsonErr(w, "invalid credentials", 401)
		return
	}
	token, err := a.jwt.Generate(user.ID, orgSlug, user.Username)
	if err != nil {
		jsonErr(w, "token error", 500)
		return
	}
	role := "member"
	if s.IsAdmin(user.ID) {
		role = "admin"
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token": token, "user_id": user.ID, "username": user.Username, "org": orgSlug, "role": role,
	})
}

func (a *ClientAPI) register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username    string `json:"username"`
		Pin         string `json:"pin"`
		DisplayName string `json:"display_name"`
		Org         string `json:"org"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	orgSlug := req.Org
	if orgSlug == "" {
		orgSlug = "default"
	}
	s, err := a.orgs.Get(orgSlug)
	if err != nil {
		jsonErr(w, "org not found", 400)
		return
	}

	user, err := s.CreateUser(req.Username, req.Pin, req.DisplayName)
	if err != nil {
		jsonErr(w, err.Error(), 400)
		return
	}
	token, _ := a.jwt.Generate(user.ID, orgSlug, req.Username)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token": token, "user_id": user.ID, "username": req.Username, "org": orgSlug, "role": "member",
	})
}

func (a *ClientAPI) getUserID(r *http.Request) int64 {
	tokenStr := r.Header.Get("Authorization")
	if tokenStr == "" {
		tokenStr = r.URL.Query().Get("token")
	}
	if tokenStr == "" {
		return 0
	}
	if a.jwt != nil {
		claims, err := a.jwt.Validate(tokenStr)
		if err == nil {
			return claims.UserID
		}
	}
	return 0
}

func (a *ClientAPI) requireAuth(w http.ResponseWriter, r *http.Request) int64 {
	uid := a.getUserID(r)
	if uid == 0 {
		jsonErr(w, "unauthorized", 401)
	}
	return uid
}

func (a *ClientAPI) checkChatAccess(w http.ResponseWriter, r *http.Request, chatID int64) bool {
	userID := a.requireAuth(w, r)
	if userID == 0 {
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
	if a.requireAuth(w, r) == 0 {
		return
	}
	s := a.db(r)
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
	json.NewDecoder(r.Body).Decode(&req)

	if req.Code == "" || req.Username == "" || req.Pin == "" {
		jsonErr(w, "code, username and pin required", 400)
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

	if err := s.UseInvite(req.Code); err != nil {
		jsonErr(w, err.Error(), 400)
		return
	}

	user, err := s.CreateUser(req.Username, req.Pin, req.DisplayName)
	if err != nil {
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
		"token": token, "user_id": user.ID, "username": req.Username, "org": orgSlug, "role": "member",
	})
}
