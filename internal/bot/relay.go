// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package bot

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/pusk-platform/pusk/internal/ws"
)

var relayUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
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
	log.Printf("[relay] bot %d connected", botID)
}

func (rh *RelayHub) Unregister(botID int64, c *ws.Conn) {
	rh.mu.Lock()
	defer rh.mu.Unlock()
	if cur, ok := rh.conns[botID]; ok && cur == c {
		delete(rh.conns, botID)
		log.Printf("[relay] bot %d disconnected", botID)
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
func IsLocalURL(url string) bool {
	if url == "" {
		return true
	}
	lower := strings.ToLower(url)
	blocklist := []string{
		"localhost", "127.0.0.1", "127.1", "0.0.0.0",
		"[::1]", "[::]",
		"169.254.169.254", // cloud metadata (AWS/GCP)
		"metadata.google",
		"10.", "192.168.", "172.16.", "172.17.", "172.18.", "172.19.",
		"172.20.", "172.21.", "172.22.", "172.23.", "172.24.", "172.25.",
		"172.26.", "172.27.", "172.28.", "172.29.", "172.30.", "172.31.",
		"0x7f", // hex loopback
	}
	for _, b := range blocklist {
		if strings.Contains(lower, b) {
			return true
		}
	}
	return false
}
