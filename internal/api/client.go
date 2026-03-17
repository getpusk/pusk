// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package api

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/websocket"
	"github.com/pusk-platform/pusk/internal/store"
	"github.com/pusk-platform/pusk/internal/ws"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ClientAPI handles PWA client requests
type ClientAPI struct {
	store *store.Store
	hub   *ws.Hub
}

func NewClientAPI(s *store.Store, hub *ws.Hub) *ClientAPI {
	return &ClientAPI{store: s, hub: hub}
}

func (a *ClientAPI) Route(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth", a.auth)
	mux.HandleFunc("POST /api/register", a.register)
	mux.HandleFunc("GET /api/bots", a.listBots)
	mux.HandleFunc("GET /api/chats", a.listChats)
	mux.HandleFunc("GET /api/chats/{chatID}/messages", a.chatMessages)
	mux.HandleFunc("POST /api/chats/{chatID}/send", a.sendToBot)
	mux.HandleFunc("POST /api/chats/{chatID}/callback", a.callback)
	mux.HandleFunc("POST /api/bots/{botID}/start", a.startChat)
	mux.HandleFunc("GET /api/ws", a.websocket)
	mux.HandleFunc("GET /api/health", a.health)
}

func (a *ClientAPI) auth(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Pin      string `json:"pin"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	user, err := a.store.AuthUser(req.Username, req.Pin)
	if err != nil {
		http.Error(w, `{"error":"invalid credentials"}`, 401)
		return
	}
	// Simple token = userID (in production: JWT)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":   strconv.FormatInt(user.ID, 10),
		"user_id": user.ID,
		"username": user.Username,
	})
}

func (a *ClientAPI) register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username    string `json:"username"`
		Pin         string `json:"pin"`
		DisplayName string `json:"display_name"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	user, err := a.store.CreateUser(req.Username, req.Pin, req.DisplayName)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, 400)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":   strconv.FormatInt(user.ID, 10),
		"user_id": user.ID,
	})
}

func (a *ClientAPI) listBots(w http.ResponseWriter, r *http.Request) {
	bots, err := a.store.ListBots()
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, 500)
		return
	}
	// Strip tokens from response
	type safeBotInfo struct {
		ID      int64  `json:"id"`
		Name    string `json:"name"`
		IconURL string `json:"icon_url,omitempty"`
	}
	safe := make([]safeBotInfo, len(bots))
	for i, b := range bots {
		safe[i] = safeBotInfo{ID: b.ID, Name: b.Name, IconURL: b.IconURL}
	}
	json.NewEncoder(w).Encode(safe)
}

func (a *ClientAPI) listChats(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	chats, err := a.store.UserChats(userID)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, 500)
		return
	}
	json.NewEncoder(w).Encode(chats)
}

func (a *ClientAPI) startChat(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	botID, _ := strconv.ParseInt(r.PathValue("botID"), 10, 64)

	chat, err := a.store.GetOrCreateChat(userID, botID)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, 500)
		return
	}

	// Send /start to bot webhook
	bot, _ := a.store.BotByID(botID)
	if bot != nil && bot.WebhookURL != "" {
		go sendWebhook(bot.WebhookURL, map[string]interface{}{
			"update_id": chat.ID,
			"message": map[string]interface{}{
				"message_id": 0,
				"chat":       map[string]interface{}{"id": chat.ID},
				"from":       map[string]interface{}{"id": userID},
				"text":       "/start",
			},
		})
	}

	json.NewEncoder(w).Encode(chat)
}

func (a *ClientAPI) chatMessages(w http.ResponseWriter, r *http.Request) {
	chatID, _ := strconv.ParseInt(r.PathValue("chatID"), 10, 64)
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		limit, _ = strconv.Atoi(l)
	}

	msgs, err := a.store.ChatMessages(chatID, limit)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, 500)
		return
	}
	if msgs == nil {
		msgs = []store.Message{}
	}
	json.NewEncoder(w).Encode(msgs)
}

func (a *ClientAPI) sendToBot(w http.ResponseWriter, r *http.Request) {
	chatID, _ := strconv.ParseInt(r.PathValue("chatID"), 10, 64)
	userID := getUserID(r)

	var req struct {
		Text string `json:"text"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	msg, err := a.store.SaveMessage(chatID, "user", req.Text, "", "", "")
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, 500)
		return
	}

	// Forward to bot webhook (Telegram Update format)
	go a.forwardToBot(chatID, userID, msg)

	json.NewEncoder(w).Encode(msg)
}

func (a *ClientAPI) callback(w http.ResponseWriter, r *http.Request) {
	chatID, _ := strconv.ParseInt(r.PathValue("chatID"), 10, 64)
	userID := getUserID(r)

	var req struct {
		Data      string `json:"data"`
		MessageID int64  `json:"message_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	// Forward callback_query to bot webhook
	go a.forwardCallback(chatID, userID, req.Data, req.MessageID)

	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (a *ClientAPI) websocket(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	if userID == 0 {
		http.Error(w, "unauthorized", 401)
		return
	}

	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	conn := ws.NewConn(wsConn, userID)
	a.hub.Register(userID, conn)

	go conn.WritePump()
	conn.ReadPump(a.hub, nil)
}

func (a *ClientAPI) health(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"online":  a.hub.Online(),
		"version": "0.1.0",
	})
}

// ── Internal ──

func getUserID(r *http.Request) int64 {
	token := r.Header.Get("Authorization")
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	id, _ := strconv.ParseInt(token, 10, 64)
	return id
}

func (a *ClientAPI) forwardToBot(chatID, userID int64, msg *store.Message) {
	botID, err := a.store.ChatBotID(chatID)
	if err != nil || botID == 0 {
		log.Printf("[webhook] no bot for chat %d", chatID)
		return
	}

	bot, err := a.store.BotByID(botID)
	if err != nil || bot.WebhookURL == "" {
		log.Printf("[webhook] bot %d has no webhook", botID)
		return
	}

	sendWebhook(bot.WebhookURL, map[string]interface{}{
		"update_id": msg.ID,
		"message": map[string]interface{}{
			"message_id": msg.ID,
			"chat":       map[string]interface{}{"id": chatID},
			"from":       map[string]interface{}{"id": userID},
			"text":       msg.Text,
			"date":       msg.CreatedAt,
		},
	})
}

func (a *ClientAPI) forwardCallback(chatID, userID int64, data string, messageID int64) {
	botID, err := a.store.ChatBotID(chatID)
	if err != nil || botID == 0 {
		return
	}

	bot, err := a.store.BotByID(botID)
	if err != nil || bot.WebhookURL == "" {
		return
	}

	sendWebhook(bot.WebhookURL, map[string]interface{}{
		"update_id": messageID,
		"callback_query": map[string]interface{}{
			"id":   strconv.FormatInt(messageID, 10),
			"from": map[string]interface{}{"id": userID},
			"message": map[string]interface{}{
				"message_id": messageID,
				"chat":       map[string]interface{}{"id": chatID},
			},
			"data": data,
		},
	})
}

func sendWebhook(url string, payload interface{}) {
	data, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		log.Printf("[webhook] error sending to %s: %v", url, err)
		return
	}
	resp.Body.Close()
	log.Printf("[webhook] sent to %s: %d", url, resp.StatusCode)
}
