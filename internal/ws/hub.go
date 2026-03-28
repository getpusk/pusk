// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package ws

import (
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/pusk-platform/pusk/internal/metrics"
)

// Event sent to PWA clients via WebSocket
type Event struct {
	Type    string          `json:"type"`
	ChatID  int64           `json:"chat_id,omitempty"`
	Payload json.RawMessage `json:"payload"`
}

// Hub manages WebSocket connections per user.
// Keys are "orgID:userID" strings to ensure multi-tenant isolation.
type Hub struct {
	mu            sync.RWMutex
	conns         map[string][]*Conn // "orgID:userID" -> connections
	status        map[string]string  // key -> "online" | "away"
	activeChannel map[string]int64   // key -> currently viewed channel ID (0 = none)
}

func NewHub() *Hub {
	return &Hub{
		conns:         make(map[string][]*Conn),
		status:        make(map[string]string),
		activeChannel: make(map[string]int64),
	}
}

func (h *Hub) Register(key string, c *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conns[key] = append(h.conns[key], c)
	h.status[key] = "online"
	metrics.WSConnections.Inc()
	slog.Info("ws user connected", "key", key, "total", len(h.conns[key]))
}

func (h *Hub) Unregister(key string, c *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	conns := h.conns[key]
	for i, conn := range conns {
		if conn == c {
			h.conns[key] = append(conns[:i], conns[i+1:]...)
			metrics.WSConnections.Dec()
			break
		}
	}
	if len(h.conns[key]) == 0 {
		delete(h.conns, key)
		delete(h.status, key)
		delete(h.activeChannel, key)
	}
}

func (h *Hub) SetStatus(key, status string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.conns[key]; ok {
		// Multi-device: "online" wins — only set "away" if no other conn is online
		if status == "away" && h.status[key] == "online" && len(h.conns[key]) > 1 {
			return
		}
		h.status[key] = status
	}
}

func (h *Hub) GetStatus(key string) string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if s, ok := h.status[key]; ok {
		return s
	}
	return "offline"
}

func (h *Hub) SetActiveChannel(key string, channelID int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.activeChannel[key] = channelID
}

func (h *Hub) GetActiveChannel(key string) int64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.activeChannel[key]
}

func (h *Hub) SendToUser(key string, evt Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	data, err := json.Marshal(evt)
	if err != nil {
		return
	}
	for _, c := range h.conns[key] {
		c.Send(data)
	}
}

func (h *Hub) Online() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.conns)
}

// OrgOnline returns count of connected users for a specific org.
func (h *Hub) OrgOnline(orgPrefix string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	count := 0
	for k := range h.conns {
		if len(k) > len(orgPrefix) && k[:len(orgPrefix)] == orgPrefix {
			count++
		}
	}
	return count
}

// IsConnected checks if a specific key has active WebSocket connections.
func (h *Hub) IsConnected(key string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	conns, ok := h.conns[key]
	return ok && len(conns) > 0
}

// OnlineKeys returns all connected keys ("orgID:userID" format).
func (h *Hub) OnlineKeys() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	keys := make([]string, 0, len(h.conns))
	for k := range h.conns {
		keys = append(keys, k)
	}
	return keys
}
