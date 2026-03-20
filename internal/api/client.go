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
	vapidPub string
	jwt      *auth.JWTService
}

func NewClientAPI(orgs *org.Manager, s *store.Store, hub *ws.Hub, push *notify.PushService, relay *bot.RelayHub, vapidPub string, jwtSvc *auth.JWTService) *ClientAPI {
	return &ClientAPI{orgs: orgs, store: s, hub: hub, push: push, relay: relay, vapidPub: vapidPub, jwt: jwtSvc}
}

// db returns the Store for the org derived from JWT claims
func (a *ClientAPI) db(r *http.Request) *store.Store {
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
	authRL := NewRateLimiter(20, time.Minute)
	regRL := NewRateLimiter(10, time.Minute)

	// Auth
	mux.HandleFunc("POST /api/auth", RateLimit(authRL, limitBody(a.auth)))
	mux.HandleFunc("POST /api/register", RateLimit(regRL, limitBody(a.register)))

	// Chat
	mux.HandleFunc("GET /api/bots", a.listBots)
	mux.HandleFunc("GET /api/chats", a.listChats)
	mux.HandleFunc("GET /api/chats/{chatID}/messages", a.chatMessages)
	mux.HandleFunc("POST /api/chats/{chatID}/send", limitBody(a.sendToBot))
	mux.HandleFunc("POST /api/chats/{chatID}/callback", limitBody(a.callback))
	mux.HandleFunc("POST /api/bots/{botID}/start", limitBody(a.startChat))
	mux.HandleFunc("DELETE /api/messages/{msgID}", a.deleteMessage)

	// Channels
	mux.HandleFunc("GET /api/channels", a.listChannels)
	mux.HandleFunc("POST /api/channels/{channelID}/subscribe", a.subscribe)
	mux.HandleFunc("POST /api/channels/{channelID}/unsubscribe", a.unsubscribe)
	mux.HandleFunc("GET /api/channels/{channelID}/messages", a.channelMessages)
	mux.HandleFunc("POST /api/channels/{channelID}/send", limitBody(a.sendToChannel))

	// Infra
	mux.HandleFunc("GET /api/ws", a.websocket)
	mux.HandleFunc("GET /api/health", a.health)
	mux.HandleFunc("POST /api/push/subscribe", limitBody(a.pushSubscribe))
	mux.HandleFunc("GET /api/push/vapid", a.vapidKey)

	// Invites
	mux.HandleFunc("POST /api/invite", limitBody(a.createInvite))
	mux.HandleFunc("POST /api/invite/accept", limitBody(a.acceptInvite))
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
