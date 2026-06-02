// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package bot

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/pusk-platform/pusk/internal/metrics"
	"github.com/pusk-platform/pusk/internal/ws"
)

var relayUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}
		host := u.Hostname()
		reqHost := r.Host
		if h, _, err := net.SplitHostPort(reqHost); err == nil {
			reqHost = h
		}
		return host == reqHost || host == "localhost" || host == "127.0.0.1"
	},
}

// RelayHub manages WebSocket connections from bots for webhook relay.
// When a bot connects via /bot/{token}/relay, updates are forwarded
// through WebSocket instead of HTTP POST — no ngrok needed.
type RelayHub struct {
	mu    sync.RWMutex
	conns map[int64]*ws.Conn // botID -> connection
}

func NewRelayHub() *RelayHub {
	return &RelayHub{conns: make(map[int64]*ws.Conn)}
}

func (rh *RelayHub) Register(botID int64, c *ws.Conn) {
	rh.mu.Lock()
	defer rh.mu.Unlock()
	// Close previous connection if exists
	if old, ok := rh.conns[botID]; ok {
		old.Send([]byte(`{"type":"replaced"}`))
	}
	rh.conns[botID] = c
	metrics.RelayBotsConnected.Set(float64(len(rh.conns)))
	slog.Info("relay bot connected", "bot_id", botID)
}

func (rh *RelayHub) Unregister(botID int64, c *ws.Conn) {
	rh.mu.Lock()
	defer rh.mu.Unlock()
	if cur, ok := rh.conns[botID]; ok && cur == c {
		delete(rh.conns, botID)
		metrics.RelayBotsConnected.Set(float64(len(rh.conns)))
		slog.Info("relay bot disconnected", "bot_id", botID)
	}
}

// Send sends a Telegram-compatible Update to the bot via WebSocket.
// Returns true if delivered, false if bot is not connected.
func (rh *RelayHub) Send(botID int64, update interface{}) bool {
	rh.mu.RLock()
	c, ok := rh.conns[botID]
	rh.mu.RUnlock()
	if !ok {
		return false
	}
	data, err := json.Marshal(update)
	if err != nil {
		return false
	}
	c.Send(data)
	return true
}

func (rh *RelayHub) IsConnected(botID int64) bool {
	rh.mu.RLock()
	defer rh.mu.RUnlock()
	_, ok := rh.conns[botID]
	return ok
}

func (rh *RelayHub) Online() int {
	rh.mu.RLock()
	defer rh.mu.RUnlock()
	return len(rh.conns)
}

// IsLocalURL returns true if the webhook URL points to localhost, private
// networks, or cloud metadata endpoints — meaning the bot should use relay
// instead of HTTP POST, or the URL should be blocked (SSRF protection).
// IsLocalURL returns true if the webhook URL points to localhost, private
// networks, or cloud metadata — meaning relay should be used or URL blocked.
func IsLocalURL(rawURL string) bool {
	if rawURL == "" {
		return true
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return true // can't parse = block
	}

	host := strings.ToLower(u.Hostname())

	// Direct hostname checks
	if host == "localhost" || host == "metadata.google.internal" {
		return true
	}

	// Literal IP — check directly.
	if ip := net.ParseIP(host); ip != nil {
		return IsBlockedIP(ip)
	}

	// Hostname — resolve and block if ANY resolved address is internal, so a
	// public A-record paired with a private one cannot slip through. Resolution
	// failure stays fail-open here: IsLocalURL is only a fast pre-filter, and
	// the webhook HTTP client re-validates the actual connected IP at dial time
	// (closing the DNS-rebinding TOCTOU), so an unresolvable corporate hostname
	// may still attempt delivery.
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return false
	}
	for _, ip := range ips {
		if IsBlockedIP(ip) {
			return true
		}
	}
	return false
}

// IsBlockedIP reports whether ip is an SSRF-sensitive internal destination:
// loopback, private, link-local, or the unspecified address (0.0.0.0 / ::).
func IsBlockedIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}
