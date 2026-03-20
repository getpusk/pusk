// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/pusk-platform/pusk/internal/auth"
	"github.com/pusk-platform/pusk/internal/store"
	"github.com/pusk-platform/pusk/internal/ws"
)

func (a *ClientAPI) ackChannelMessage(w http.ResponseWriter, r *http.Request) {
	userID := a.requireAuth(w, r)
	if userID == 0 {
		return
	}
	channelID, _ := strconv.ParseInt(r.PathValue("channelID"), 10, 64)
	s := a.db(r)

	var req struct {
		MessageID int64  `json:"message_id"`
		Action    string `json:"action"` // ack, mute, resolved
	}
	json.NewDecoder(r.Body).Decode(&req)

	username := ""
	if claims := a.getJWTClaims(r); claims != nil {
		username = claims.Username
	}

	// Get current message text and append ACK status
	msgs, _ := s.ChannelMessages(channelID, 200)
	for _, m := range msgs {
		if m.ID == req.MessageID {
			now := time.Now().Format("15:04")
			status := ""
			switch req.Action {
			case "ack":
				status = fmt.Sprintf("\n\n**ACK**: @%s в %s", username, now)
			case "mute":
				status = fmt.Sprintf("\n\n**Muted 1h**: @%s в %s", username, now)
			case "resolved":
				status = fmt.Sprintf("\n\n**Resolved**: @%s в %s", username, now)
			}
			newText := m.Text + status
			s.UpdateChannelMessageText(m.ID, newText, "")
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})
			return
		}
	}
	jsonErr(w, "message not found", 404)
}

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
		Unread      int    `json:"unread"`
	}
	result := make([]channelInfo, 0, len(channels))
	for _, ch := range channels {
		result = append(result, channelInfo{
			ID: ch.ID, Name: ch.Name, Description: ch.Description,
			Subscribed: a.db(r).IsSubscribed(ch.ID, userID),
			Unread:     a.db(r).UnreadCount(ch.ID, userID),
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
	userID := a.requireAuth(w, r)
	if userID == 0 {
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
	// Mark channel as read
	if len(msgs) > 0 {
		a.db(r).MarkChannelRead(channelID, userID, msgs[0].ID) // msgs[0] = newest (DESC order)
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
		Text    string `json:"text"`
		ReplyTo int64  `json:"reply_to"`
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
	if msg != nil && req.ReplyTo > 0 {
		s.SetChannelMessageReplyTo(msg.ID, req.ReplyTo)
		msg.ReplyTo = req.ReplyTo
	}
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
