// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package api

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pusk-platform/pusk/internal/auth"
	"github.com/pusk-platform/pusk/internal/bot"
	"github.com/pusk-platform/pusk/internal/notify"
	"github.com/pusk-platform/pusk/internal/org"
	"github.com/pusk-platform/pusk/internal/store"
	"github.com/pusk-platform/pusk/internal/ws"
)

// Version is set via -ldflags at build time
var Version = "dev"

var upgrader = websocket.Upgrader{
	CheckOrigin: checkWSOrigin,
}

// ClientAPI handles PWA client requests
type ClientAPI struct {
	orgs     *org.Manager
	store    *store.Store
	hub      *ws.Hub
	push     *notify.PushService
	relay    *bot.RelayHub
	updates  *bot.UpdateQueue
	vapidPub string
	jwt      *auth.JWTService
}

func NewClientAPI(orgs *org.Manager, s *store.Store, hub *ws.Hub, push *notify.PushService, relay *bot.RelayHub, updates *bot.UpdateQueue, vapidPub string, jwtSvc *auth.JWTService) *ClientAPI {
	return &ClientAPI{orgs: orgs, store: s, hub: hub, push: push, relay: relay, updates: updates, vapidPub: vapidPub, jwt: jwtSvc}
}

// db returns the Store for the org derived from JWT claims in context.
// Falls back to parsing the token from request if claims are not in context.
func (a *ClientAPI) db(r *http.Request) *store.Store {
	if claims := ClaimsFromCtx(r.Context()); claims != nil && claims.OrgID != "" {
		if a.orgs != nil {
			if s, err := a.orgs.Get(claims.OrgID); err == nil {
				return s
			}
		}
	}
	// Fallback: parse token directly (for routes without AuthRequired)
	tokenStr := r.Header.Get("Authorization")
	if tokenStr == "" {
		tokenStr = r.URL.Query().Get("token")
	}
	if a.orgs != nil && a.jwt != nil && tokenStr != "" {
		if claims, err := a.jwt.Validate(tokenStr); err == nil && claims.OrgID != "" {
			if s, err := a.orgs.Get(claims.OrgID); err == nil {
				return s
			}
		}
	}
	return a.store
}

func (a *ClientAPI) Route(mux *http.ServeMux) {
	authRL := NewRateLimiter(10, time.Minute) // 10 auth/min per IP
	regRL := NewRateLimiter(10, time.Minute)
	sendRL := NewRateLimiter(30, time.Minute)   // 30 msgs/min per IP
	uploadRL := NewRateLimiter(10, time.Minute) // 10 uploads/min per IP

	// Public routes (no auth required)
	mux.HandleFunc("POST /api/auth", RateLimit(authRL, limitBody(a.auth)))
	mux.HandleFunc("POST /api/register", RateLimit(regRL, limitBody(a.register)))
	mux.HandleFunc("GET /api/health", a.health)
	mux.HandleFunc("GET /api/push/vapid", a.vapidKey)
	mux.HandleFunc("POST /api/invite/accept", RateLimit(regRL, limitBody(a.acceptInvite)))
	mux.HandleFunc("GET /api/invite/check-user", a.checkInviteUser)

	// Auth-required routes: Chat
	mux.HandleFunc("GET /api/bots", a.AuthRequired(a.listBots))
	mux.HandleFunc("GET /api/chats", a.AuthRequired(a.listChats))
	mux.HandleFunc("GET /api/chats/{chatID}/messages", a.AuthRequired(a.chatMessages))
	mux.HandleFunc("POST /api/chats/{chatID}/send", a.AuthRequired(RateLimit(sendRL, limitBody(a.sendToBot))))
	mux.HandleFunc("POST /api/chats/{chatID}/callback", a.AuthRequired(limitBody(a.callback)))
	mux.HandleFunc("POST /api/bots/{botID}/start", a.AuthRequired(limitBody(a.startChat)))
	mux.HandleFunc("DELETE /api/messages/{msgID}", a.AuthRequired(a.deleteMessage))

	// Auth-required routes: Channels
	mux.HandleFunc("GET /api/channels", a.AuthRequired(a.listChannels))
	mux.HandleFunc("POST /api/channels/{channelID}/subscribe", a.AuthRequired(a.subscribe))
	mux.HandleFunc("POST /api/channels/{channelID}/unsubscribe", a.AuthRequired(a.unsubscribe))
	mux.HandleFunc("GET /api/channels/{channelID}/messages", a.AuthRequired(a.channelMessages))
	mux.HandleFunc("GET /api/channels/{channelID}/readers", a.AuthRequired(a.channelReaders))
	mux.HandleFunc("POST /api/channels/{channelID}/send", a.AuthRequired(RateLimit(sendRL, limitBody(a.sendToChannel))))
	mux.HandleFunc("POST /api/channels/{channelID}/ack", a.AuthRequired(limitBody(a.ackChannelMessage)))
	mux.HandleFunc("PUT /api/channels/{channelID}/messages/{msgID}", a.AuthRequired(limitBody(a.editChannelMessage)))
	mux.HandleFunc("DELETE /api/channels/messages/{msgID}", a.AuthRequired(a.deleteChannelMessage))
	mux.HandleFunc("POST /api/channels/{channelID}/pin", a.AuthRequired(limitBody(a.pinMessage)))
	mux.HandleFunc("POST /api/channels/{channelID}/upload", a.AuthRequired(RateLimit(uploadRL, a.uploadToChannel)))

	// Auth-required routes: Users & Roles
	mux.HandleFunc("GET /api/users", a.AuthRequired(a.listUsers))
	mux.HandleFunc("POST /api/users/{userID}/role", a.AuthRequired(limitBody(a.setUserRole)))
	mux.HandleFunc("DELETE /api/users/{userID}", a.AuthRequired(a.deleteUser))

	// Auth-required routes: Infra
	mux.HandleFunc("GET /api/ws", a.AuthRequired(a.websocket))
	mux.HandleFunc("GET /api/online", a.AuthRequired(a.onlineUsers))
	mux.HandleFunc("POST /api/push/subscribe", a.AuthRequired(limitBody(a.pushSubscribe)))
	mux.HandleFunc("DELETE /api/push/subscribe", a.AuthRequired(limitBody(a.pushUnsubscribe)))
	mux.HandleFunc("POST /api/push/test", a.AuthRequired(a.testPush))

	// Auth-required routes: Invites
	mux.HandleFunc("POST /api/invite", a.AuthRequired(limitBody(a.createInvite)))
	mux.HandleFunc("GET /api/invite/active", a.AuthRequired(a.activeInvite))
	mux.HandleFunc("DELETE /api/invite", a.AuthRequired(limitBody(a.revokeInvite)))
	mux.HandleFunc("POST /api/file-token", a.AuthRequired(a.createFileToken))

	// Self-service password change
	mux.HandleFunc("POST /api/change-password", a.AuthRequired(limitBody(a.changePassword)))
}

// ── Helpers ──

func limitBody(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		next(w, r)
	}
}

func jsonErr(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func checkWSOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	originHost := u.Hostname()
	requestHost := r.Host
	if h, _, err := net.SplitHostPort(requestHost); err == nil {
		requestHost = h
	}
	if originHost == requestHost {
		return true
	}
	if originHost == "localhost" || originHost == "127.0.0.1" {
		return true
	}
	return false
}
