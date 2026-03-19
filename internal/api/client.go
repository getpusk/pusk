// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package api

import (
	"bytes"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pusk-platform/pusk/internal/auth"
	"github.com/pusk-platform/pusk/internal/bot"
	"github.com/pusk-platform/pusk/internal/notify"
	"github.com/pusk-platform/pusk/internal/org"
	"github.com/pusk-platform/pusk/internal/store"
	"github.com/pusk-platform/pusk/internal/ws"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: checkWSOrigin,
}

func checkWSOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true // non-browser clients (curl, bots) don't send Origin
	}
	// Parse origin to extract host
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	originHost := u.Hostname()
	requestHost := r.Host
	// Strip port from request host
	if h, _, err := net.SplitHostPort(requestHost); err == nil {
		requestHost = h
	}
	// Exact host match
	if originHost == requestHost {
		return true
	}
	// Allow localhost for development
	if originHost == "localhost" || originHost == "127.0.0.1" {
		return true
	}
	return false
}

// ClientAPI handles PWA client requests
type ClientAPI struct {
	orgs     *org.Manager
	store    *store.Store // default org store
	hub      *ws.Hub
	push     *notify.PushService
	relay    *bot.RelayHub
	vapidPub string
	jwt      *auth.JWTService
}

func NewClientAPI(orgs *org.Manager, s *store.Store, hub *ws.Hub, push *notify.PushService, relay *bot.RelayHub, vapidPub string, jwtSvcParam *auth.JWTService) *ClientAPI {
	svc := &ClientAPI{orgs: orgs, store: s, hub: hub, push: push, relay: relay, vapidPub: vapidPub, jwt: jwtSvcParam}
	jwtSvc = jwtSvcParam
	return svc
}

// db returns the Store for the org derived from JWT claims in the request
func (a *ClientAPI) db(r *http.Request) *store.Store {
	tokenStr := r.Header.Get("Authorization")
	if tokenStr == "" {
		tokenStr = r.URL.Query().Get("token")
	}
	if a.orgs != nil && jwtSvc != nil && tokenStr != "" {
		if claims, err := jwtSvc.Validate(tokenStr); err == nil && claims.OrgID != "" {
			if s, err := a.orgs.Get(claims.OrgID); err == nil {
				return s
			}
		}
	}
	return a.store
}

func (a *ClientAPI) Route(mux *http.ServeMux) {
	authRL := NewRateLimiter(20, time.Minute) // 20 attempts per minute per IP
	regRL := NewRateLimiter(10, time.Minute)  // 10 registrations per minute per IP
	mux.HandleFunc("POST /api/auth", RateLimit(authRL, a.auth))
	mux.HandleFunc("POST /api/register", RateLimit(regRL, a.register))
	mux.HandleFunc("GET /api/bots", a.listBots)
	mux.HandleFunc("GET /api/chats", a.listChats)
	mux.HandleFunc("GET /api/chats/{chatID}/messages", a.chatMessages)
	mux.HandleFunc("POST /api/chats/{chatID}/send", a.sendToBot)
	mux.HandleFunc("POST /api/chats/{chatID}/callback", a.callback)
	mux.HandleFunc("POST /api/bots/{botID}/start", a.startChat)
	mux.HandleFunc("GET /api/ws", a.websocket)
	mux.HandleFunc("GET /api/health", a.health)
	mux.HandleFunc("GET /api/channels", a.listChannels)
	mux.HandleFunc("POST /api/channels/{channelID}/subscribe", a.subscribe)
	mux.HandleFunc("POST /api/channels/{channelID}/unsubscribe", a.unsubscribe)
	mux.HandleFunc("GET /api/channels/{channelID}/messages", a.channelMessages)
	mux.HandleFunc("DELETE /api/messages/{msgID}", a.deleteMessage)
	mux.HandleFunc("POST /api/push/subscribe", a.pushSubscribe)
	mux.HandleFunc("GET /api/push/vapid", a.vapidKey)
}

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
		http.Error(w, `{"error":"token error"}`, 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":    token,
		"user_id":  user.ID,
		"username": user.Username,
		"org":      orgSlug,
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
		"token":    token,
		"user_id":  user.ID,
		"username": req.Username,
		"org":      orgSlug,
	})
}

func (a *ClientAPI) listBots(w http.ResponseWriter, r *http.Request) {
	if requireAuth(w, r) == 0 {
		return
	}
	bots, err := a.db(r).ListBots()
	if err != nil {
		jsonErr(w, "internal error", 500)
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
	userID := requireAuth(w, r)
	if userID == 0 {
		return
	}
	chats, err := a.db(r).UserChats(userID)
	if err != nil {
		jsonErr(w, "internal error", 500)
		return
	}
	json.NewEncoder(w).Encode(chats)
}

func (a *ClientAPI) startChat(w http.ResponseWriter, r *http.Request) {
	userID := requireAuth(w, r)
	if userID == 0 {
		return
	}
	botID, _ := strconv.ParseInt(r.PathValue("botID"), 10, 64)

	chat, err := a.db(r).GetOrCreateChat(userID, botID)
	if err != nil {
		jsonErr(w, "internal error", 500)
		return
	}

	// Send /start to bot via relay or webhook
	b, _ := a.db(r).BotByID(botID)
	if b != nil {
		update := map[string]interface{}{
			"update_id": chat.ID,
			"message": map[string]interface{}{
				"message_id": 0,
				"chat":       map[string]interface{}{"id": chat.ID},
				"from":       map[string]interface{}{"id": userID},
				"text":       "/start",
			},
		}
		go func() {
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
	userID := getUserID(r)

	var req struct {
		Text string `json:"text"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	msg, err := a.db(r).SaveMessage(chatID, "user", req.Text, "", "", "")
	if err != nil {
		jsonErr(w, "internal error", 500)
		return
	}

	// Forward to bot webhook (Telegram Update format)
	s := a.db(r)
	go a.forwardToBot(s, chatID, userID, msg)

	json.NewEncoder(w).Encode(msg)
}

func (a *ClientAPI) callback(w http.ResponseWriter, r *http.Request) {
	chatID, _ := strconv.ParseInt(r.PathValue("chatID"), 10, 64)
	if !a.checkChatAccess(w, r, chatID) {
		return
	}
	userID := getUserID(r)

	var req struct {
		Data      string `json:"data"`
		MessageID int64  `json:"message_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	// Forward callback_query to bot webhook
	s := a.db(r)
	go a.forwardCallback(s, chatID, userID, req.Data, req.MessageID)

	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (a *ClientAPI) websocket(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	if userID == 0 {
		jsonErr(w, "invalid credentials", 401)
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
		"version": Version,
	})
}

func (a *ClientAPI) listChannels(w http.ResponseWriter, r *http.Request) {
	userID := requireAuth(w, r)
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
	userID := requireAuth(w, r)
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
	userID := requireAuth(w, r)
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
	if requireAuth(w, r) == 0 {
		return
	}
	channelID, _ := strconv.ParseInt(r.PathValue("channelID"), 10, 64)
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		limit, _ = strconv.Atoi(l)
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

func (a *ClientAPI) pushSubscribe(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
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

func (a *ClientAPI) deleteMessage(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	if userID == 0 {
		jsonErr(w, "unauthorized", 401)
		return
	}
	msgID, _ := strconv.ParseInt(r.PathValue("msgID"), 10, 64)
	// Verify user owns the chat this message belongs to
	msg, err := a.db(r).GetMessage(msgID)
	if err != nil {
		jsonErr(w, "not found", 404)
		return
	}
	ownerID, _ := a.db(r).ChatUserID(msg.ChatID)
	if ownerID != userID {
		jsonErr(w, "forbidden", 403)
		return
	}
	if err := a.db(r).DeleteMessage(msgID); err != nil {
		jsonErr(w, "internal error", 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// ── Internal ──

// jwtSvc is set during init — package-level for getUserID access
// Version is set via -ldflags at build time
var Version = "dev"

var jwtSvc *auth.JWTService

func jsonErr(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func getUserID(r *http.Request) int64 {
	tokenStr := r.Header.Get("Authorization")
	if tokenStr == "" {
		tokenStr = r.URL.Query().Get("token")
	}
	if tokenStr == "" {
		return 0
	}
	// Try JWT first
	if jwtSvc != nil {
		claims, err := jwtSvc.Validate(tokenStr)
		if err == nil {
			return claims.UserID
		}
	}
	return 0
}

// requireAuth returns userID or writes 401 and returns 0
func requireAuth(w http.ResponseWriter, r *http.Request) int64 {
	uid := getUserID(r)
	if uid == 0 {
		jsonErr(w, "unauthorized", 401)
	}
	return uid
}

// checkChatAccess verifies the requesting user owns the chat
func (a *ClientAPI) checkChatAccess(w http.ResponseWriter, r *http.Request, chatID int64) bool {
	userID := getUserID(r)
	ownerID, err := a.db(r).ChatUserID(chatID)
	if err != nil || ownerID != userID {
		jsonErr(w, "forbidden", 403)
		return false
	}
	return true
}

func (a *ClientAPI) forwardToBot(s *store.Store, chatID, userID int64, msg *store.Message) {
	botID, err := s.ChatBotID(chatID)
	if err != nil || botID == 0 {
		log.Printf("[webhook] no bot for chat %d", chatID)
		return
	}

	b, err := s.BotByID(botID)
	if err != nil {
		log.Printf("[webhook] bot %d not found", botID)
		return
	}

	update := map[string]interface{}{
		"update_id": msg.ID,
		"message": map[string]interface{}{
			"message_id": msg.ID,
			"chat":       map[string]interface{}{"id": chatID},
			"from":       map[string]interface{}{"id": userID},
			"text":       msg.Text,
			"date":       msg.CreatedAt,
		},
	}

	// Try relay first (bot connected via WebSocket)
	if a.relay != nil && a.relay.Send(botID, update) {
		log.Printf("[relay] forwarded to bot %s (ws)", b.Name)
		return
	}

	// Fall back to HTTP webhook
	if b.WebhookURL == "" || bot.IsLocalURL(b.WebhookURL) {
		log.Printf("[webhook] bot %s: no webhook and not connected via relay", b.Name)
		return
	}
	sendWebhook(b.WebhookURL, update)
}

func (a *ClientAPI) forwardCallback(s *store.Store, chatID, userID int64, data string, messageID int64) {
	botID, err := s.ChatBotID(chatID)
	if err != nil || botID == 0 {
		return
	}

	b, err := s.BotByID(botID)
	if err != nil {
		return
	}

	update := map[string]interface{}{
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
	}

	// Try relay first
	if a.relay != nil && a.relay.Send(botID, update) {
		return
	}

	// Fall back to HTTP webhook
	if b.WebhookURL == "" || bot.IsLocalURL(b.WebhookURL) {
		return
	}
	sendWebhook(b.WebhookURL, update)
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
