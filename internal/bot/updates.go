// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package bot

import (
	"log/slog"
	"sync"
	"time"
)

// Update represents a Telegram-compatible update object for long polling.
type Update struct {
	UpdateID int64       `json:"update_id"`
	Message  interface{} `json:"message,omitempty"`
	Callback interface{} `json:"callback_query,omitempty"`
}

// UpdateQueue manages per-bot buffered channels for getUpdates long polling.
type UpdateQueue struct {
	mu       sync.Mutex
	channels map[int64]chan Update // botID -> buffered channel
}

// NewUpdateQueue creates a new UpdateQueue.
func NewUpdateQueue() *UpdateQueue {
	return &UpdateQueue{
		channels: make(map[int64]chan Update),
	}
}

// channel returns (or creates) the buffered channel for a bot.
func (q *UpdateQueue) channel(botID int64) chan Update {
	ch, ok := q.channels[botID]
	if !ok {
		ch = make(chan Update, 100)
		q.channels[botID] = ch
	}
	return ch
}

// Push adds an update to the bot's queue. Non-blocking: drops if buffer full.
func (q *UpdateQueue) Push(botID int64, u Update) {
	q.mu.Lock()
	ch := q.channel(botID)
	q.mu.Unlock()

	select {
	case ch <- u:
		slog.Debug("update queued", "bot_id", botID, "update_id", u.UpdateID)
	default:
		slog.Warn("update queue full, dropping", "bot_id", botID, "update_id", u.UpdateID)
	}
}

// Poll waits for updates with long polling. Returns when updates are available
// or timeout expires. Skips updates with UpdateID <= offset.
func (q *UpdateQueue) Poll(botID int64, offset int64, timeout time.Duration) []Update {
	q.mu.Lock()
	ch := q.channel(botID)
	q.mu.Unlock()

	var updates []Update

	// First, drain any already-buffered updates
	for {
		select {
		case u := <-ch:
			if u.UpdateID > offset {
				updates = append(updates, u)
			}
		default:
			goto drained
		}
	}
drained:

	// If we already have updates, return immediately
	if len(updates) > 0 {
		return updates
	}

	// Otherwise, wait for the first update or timeout
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case u := <-ch:
		if u.UpdateID > offset {
			updates = append(updates, u)
		}
		// After receiving one, drain any more that arrived
		for {
			select {
			case u2 := <-ch:
				if u2.UpdateID > offset {
					updates = append(updates, u2)
				}
			default:
				return updates
			}
		}
	case <-timer.C:
		return updates
	}
}
