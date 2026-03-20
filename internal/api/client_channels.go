// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/pusk-platform/pusk/internal/auth"
	"github.com/pusk-platform/pusk/internal/store"
	"github.com/pusk-platform/pusk/internal/ws"
)

func (a *ClientAPI) getJWTClaims(r *http.Request) *auth.Claims {
	tokenStr := r.Header.Get("Authorization")
	if tokenStr == "" {
		tokenStr = r.URL.Query().Get("token")
	}
	if a.jwt != nil && tokenStr != "" {
		claims, err := a.jwt.Validate(tokenStr)
		if err == nil {
			return claims
		}
	}
	return nil
}

func (a *ClientAPI) listChannels(w http.ResponseWriter, r *http.Request) {
	userID := a.requireAuth(w, r)
	if userID == 0 {
		return
	}
	channels, err := a.db(r).ListChannels()
	if err != nil {
		jsonErr(w, "internal error", 500)
		return
	}
	type channelInfo struct {
		ID          int64  `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		Subscribed  bool   `json:"subscribed"`
	}
	result := make([]channelInfo, 0, len(channels))
	for _, ch := range channels {
		result = append(result, channelInfo{
			ID: ch.ID, Name: ch.Name, Description: ch.Description,
			Subscribed: a.db(r).IsSubscribed(ch.ID, userID),
		})
	}
	json.NewEncoder(w).Encode(result)
}

func (a *ClientAPI) subscribe(w http.ResponseWriter, r *http.Request) {
	userID := a.requireAuth(w, r)
	if userID == 0 {
		return
	}
	channelID, _ := strconv.ParseInt(r.PathValue("channelID"), 10, 64)
	if err := a.db(r).Subscribe(channelID, userID); err != nil {
		jsonErr(w, "internal error", 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (a *ClientAPI) unsubscribe(w http.ResponseWriter, r *http.Request) {
	userID := a.requireAuth(w, r)
	if userID == 0 {
		return
	}
	channelID, _ := strconv.ParseInt(r.PathValue("channelID"), 10, 64)
	if err := a.db(r).Unsubscribe(channelID, userID); err != nil {
		jsonErr(w, "internal error", 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (a *ClientAPI) channelMessages(w http.ResponseWriter, r *http.Request) {
	if a.requireAuth(w, r) == 0 {
		return
	}
	channelID, _ := strconv.ParseInt(r.PathValue("channelID"), 10, 64)
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		limit, _ = strconv.Atoi(l)
	}
	if limit > 200 {
		limit = 200
	}
	msgs, err := a.db(r).ChannelMessages(channelID, limit)
	if err != nil {
		jsonErr(w, "internal error", 500)
		return
	}
	if msgs == nil {
		msgs = []store.ChannelMessage{}
	}
	json.NewEncoder(w).Encode(msgs)
}

func (a *ClientAPI) sendToChannel(w http.ResponseWriter, r *http.Request) {
	userID := a.requireAuth(w, r)
	if userID == 0 {
		return
	}
	channelID, _ := strconv.ParseInt(r.PathValue("channelID"), 10, 64)
	s := a.db(r)

	// Must be subscribed to send
	if !s.IsSubscribed(channelID, userID) {
		jsonErr(w, "not subscribed", 403)
		return
	}

	var req struct {
		Text string `json:"text"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Text == "" {
		jsonErr(w, "text required", 400)
		return
	}

	// Get username for sender_name
	username := ""
	if claims := a.getJWTClaims(r); claims != nil {
		username = claims.Username
	}

	msg, err := s.SaveChannelMessageFrom(channelID, "user", username, req.Text, "", "", "")
	if err != nil {
		jsonErr(w, "internal error", 500)
		return
	}

	// Push to all subscribers via WebSocket
	ch, _ := s.ChannelByID(channelID)
	if ch != nil {
		subs, _ := s.ChannelSubscribers(ch.ID)
		payload, _ := json.Marshal(map[string]interface{}{
			"message":      msg,
			"channel_name": ch.Name,
			"sender_name":  username,
		})
		for _, uid := range subs {
			a.hub.SendToUser(uid, ws.Event{Type: "channel_message", ChatID: ch.ID, Payload: payload})
		}
	}

	json.NewEncoder(w).Encode(msg)
}

func (a *ClientAPI) pushSubscribe(w http.ResponseWriter, r *http.Request) {
	userID := a.getUserID(r)
	var req struct {
		Endpoint string `json:"endpoint"`
		Keys     struct {
			P256dh string `json:"p256dh"`
			Auth   string `json:"auth"`
		} `json:"keys"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if err := a.db(r).SavePushSubscription(userID, req.Endpoint, req.Keys.P256dh, req.Keys.Auth); err != nil {
		jsonErr(w, "internal error", 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (a *ClientAPI) vapidKey(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"key": a.vapidPub})
}
