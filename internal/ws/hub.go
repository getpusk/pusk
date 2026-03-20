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

// Hub manages WebSocket connections per user
type Hub struct {
	mu    sync.RWMutex
	conns map[int64][]*Conn // userID -> connections
}

func NewHub() *Hub {
	return &Hub{conns: make(map[int64][]*Conn)}
}

func (h *Hub) Register(userID int64, c *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conns[userID] = append(h.conns[userID], c)
	metrics.WSConnections.Inc()
	slog.Info("ws user connected", "user_id", userID, "total", len(h.conns[userID]))
}

func (h *Hub) Unregister(userID int64, c *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	conns := h.conns[userID]
	for i, conn := range conns {
		if conn == c {
			h.conns[userID] = append(conns[:i], conns[i+1:]...)
			metrics.WSConnections.Dec()
			break
		}
	}
	if len(h.conns[userID]) == 0 {
		delete(h.conns, userID)
	}
}

func (h *Hub) SendToUser(userID int64, evt Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	data, err := json.Marshal(evt)
	if err != nil {
		return
	}
	for _, c := range h.conns[userID] {
		c.Send(data)
	}
}

func (h *Hub) Online() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.conns)
}
