// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/pusk-platform/pusk/internal/bot"
	"github.com/pusk-platform/pusk/internal/store"
	"github.com/pusk-platform/pusk/internal/ws"
)

func (a *ClientAPI) listBots(w http.ResponseWriter, r *http.Request) {
	s := a.db(r)
	bots, err := s.ListBots()
	if err != nil {
		jsonErr(w, "internal error", 500)
		return
	}
	type botInfo struct {
		ID      int64  `json:"id"`
		Name    string `json:"name"`
		Token   string `json:"token,omitempty"`
		Webhook string `json:"webhook,omitempty"`
		IconURL string `json:"icon_url,omitempty"`
	}
	isAdmin := s.IsAdmin(UserIDFromCtx(r.Context()))
	result := make([]botInfo, len(bots))
	for i, b := range bots {
		bi := botInfo{ID: b.ID, Name: b.Name, IconURL: b.IconURL}
		if isAdmin {
			bi.Token = b.Token
			bi.Webhook = "/hook/" + b.Token + "?format=raw"
		}
		result[i] = bi
	}
	json.NewEncoder(w).Encode(result)
}

func (a *ClientAPI) listChats(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromCtx(r.Context())
	if claims != nil && (claims.OrgID == "" || claims.OrgID == "default") {
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	userID := UserIDFromCtx(r.Context())
	chats, err := a.db(r).UserChats(userID)
	if err != nil {
		jsonErr(w, "internal error", 500)
		return
	}
	json.NewEncoder(w).Encode(chats)
}

func (a *ClientAPI) startChat(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromCtx(r.Context())
	botID, _ := strconv.ParseInt(r.PathValue("botID"), 10, 64)

	s := a.db(r)
	chat, err := s.GetOrCreateChat(userID, botID)
	if err != nil {
		jsonErr(w, "internal error", 500)
		return
	}

	// Only send /start to bot if this is a new chat (no messages yet)
	msgs, _ := s.ChatMessages(chat.ID, 1)
	b, _ := s.BotByID(botID)
	if b != nil && len(msgs) == 0 {
		startMsg := map[string]interface{}{
			"message_id": 0,
			"chat":       map[string]interface{}{"id": chat.ID, "type": "private"},
			"from":       map[string]interface{}{"id": userID, "is_bot": false, "first_name": "User"},
			"text":       "/start",
			"date":       time.Now().Unix(),
			"entities":   []map[string]interface{}{{"type": "bot_command", "offset": 0, "length": 6}},
		}
		update := map[string]interface{}{
			"update_id": 0,
			"message":   startMsg,
		}
		go func() {
			// Push to update queue for getUpdates long polling
			if a.updates != nil {
				a.updates.Push(b.ID, bot.Update{
					Message: startMsg,
				})
			}
			if a.relay != nil && a.relay.Send(b.ID, update) {
				return
			}
			if b.WebhookURL != "" && !bot.IsLocalURL(b.WebhookURL) {
				sendWebhook(b.WebhookURL, update)
			}
		}()
	}

	json.NewEncoder(w).Encode(chat)
}

func (a *ClientAPI) chatMessages(w http.ResponseWriter, r *http.Request) {
	chatID, _ := strconv.ParseInt(r.PathValue("chatID"), 10, 64)
	if !a.checkChatAccess(w, r, chatID) {
		return
	}
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		limit, _ = strconv.Atoi(l)
	}
	if limit > 200 {
		limit = 200
	}

	msgs, err := a.db(r).ChatMessages(chatID, limit)
	if err != nil {
		jsonErr(w, "internal error", 500)
		return
	}
	if msgs == nil {
		msgs = []store.Message{}
	}
	json.NewEncoder(w).Encode(msgs)
}

func (a *ClientAPI) sendToBot(w http.ResponseWriter, r *http.Request) {
	chatID, _ := strconv.ParseInt(r.PathValue("chatID"), 10, 64)
	if !a.checkChatAccess(w, r, chatID) {
		return
	}
	userID := UserIDFromCtx(r.Context())

	var req struct {
		Text string `json:"text"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	// BUG-9: reject empty text
	if req.Text == "" {
		jsonErr(w, "text is required", 400)
		return
	}
	// EDGE-10: message text length limit
	if len(req.Text) > 4096 {
		jsonErr(w, "text too long (max 4096 chars)", 400)
		return
	}

	s := a.db(r)
	msg, err := s.SaveMessage(chatID, "user", req.Text, "", "", "")
	if err != nil {
		jsonErr(w, "internal error", 500)
		return
	}

	a.pushToUpdateQueue(s, chatID, userID, msg)
	go a.forwardToBot(s, chatID, userID, msg)

	json.NewEncoder(w).Encode(msg)
}

func (a *ClientAPI) callback(w http.ResponseWriter, r *http.Request) {
	chatID, _ := strconv.ParseInt(r.PathValue("chatID"), 10, 64)
	if !a.checkChatAccess(w, r, chatID) {
		return
	}
	userID := UserIDFromCtx(r.Context())

	var req struct {
		Data      string `json:"data"`
		MessageID int64  `json:"message_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	s := a.db(r)
	a.pushCallbackToQueue(s, chatID, userID, req.Data, req.MessageID)
	go a.forwardCallback(s, chatID, userID, req.Data, req.MessageID)

	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (a *ClientAPI) deleteMessage(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromCtx(r.Context())
	msgID, _ := strconv.ParseInt(r.PathValue("msgID"), 10, 64)
	s := a.db(r)
	msg, err := s.GetMessage(msgID)
	if err != nil {
		jsonErr(w, "not found", 404)
		return
	}
	ownerID, _ := s.ChatUserID(msg.ChatID)
	if ownerID != userID {
		jsonErr(w, "forbidden", 403)
		return
	}
	if err := s.DeleteMessage(msgID); err != nil {
		jsonErr(w, "internal error", 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (a *ClientAPI) websocket(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromCtx(r.Context())
	if claims == nil || claims.UserID == 0 {
		jsonErr(w, "invalid credentials", 401)
		return
	}

	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	key := claims.OrgID + ":" + fmt.Sprintf("%d", claims.UserID)
	conn := ws.NewConn(wsConn, claims.UserID)
	conn.Key = key
	a.hub.Register(key, conn)

	go conn.WritePump()
	conn.ReadPump(a.hub, func(userID int64, data []byte) {
		var msg struct {
			Type      string `json:"type"`
			Status    string `json:"status,omitempty"`
			ChannelID int64  `json:"channel_id,omitempty"`
		}
		if json.Unmarshal(data, &msg) != nil {
			return
		}
		switch msg.Type {
		case "status":
			if msg.Status == "online" || msg.Status == "away" {
				a.hub.SetStatus(key, msg.Status)
			}
		case "typing":
			if msg.ChannelID > 0 {
				s := a.db(r)
				subs, _ := s.ChannelSubscribers(msg.ChannelID)
				payload, _ := json.Marshal(map[string]interface{}{
					"username":   claims.Username,
					"channel_id": msg.ChannelID,
				})
				for _, uid := range subs {
					if uid == claims.UserID {
						continue
					}
					subKey := claims.OrgID + ":" + fmt.Sprintf("%d", uid)
					a.hub.SendToUser(subKey, ws.Event{Type: "typing", ChatID: msg.ChannelID, Payload: payload})
				}
			}
		}
	})
}

func (a *ClientAPI) health(w http.ResponseWriter, r *http.Request) {
	s := a.db(r)
	dbOK := s.Ping() == nil

	status := "ok"
	if !dbOK {
		status = "degraded"
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  status,
		"online":  a.hub.Online(),
		"version": Version,
		"db":      dbOK,
	})
}
