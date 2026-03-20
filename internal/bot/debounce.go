// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package bot

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// Debouncer deduplicates webhook payloads within a time window.
// Alertmanager and similar systems often send burst calls with identical
// payloads — this prevents processing the same alert multiple times.
type Debouncer struct {
	mu     sync.Mutex
	seen   map[string]time.Time
	window time.Duration
}

// NewDebouncer creates a Debouncer that treats identical payloads
// received within the given window as duplicates.
func NewDebouncer(window time.Duration) *Debouncer {
	d := &Debouncer{seen: make(map[string]time.Time), window: window}
	go d.cleanup()
	return d
}

// IsDuplicate returns true if data was already seen within the window.
func (d *Debouncer) IsDuplicate(data []byte) bool {
	h := sha256.Sum256(data)
	key := hex.EncodeToString(h[:8]) // 8 bytes = 16 hex chars, enough
	d.mu.Lock()
	defer d.mu.Unlock()
	if t, ok := d.seen[key]; ok && time.Since(t) < d.window {
		return true
	}
	d.seen[key] = time.Now()
	return false
}

func (d *Debouncer) cleanup() {
	ticker := time.NewTicker(d.window)
	defer ticker.Stop()
	for range ticker.C {
		d.mu.Lock()
		now := time.Now()
		for k, t := range d.seen {
			if now.Sub(t) > d.window {
				delete(d.seen, k)
			}
		}
		d.mu.Unlock()
	}
}
