// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package bot

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/pusk-platform/pusk/internal/metrics"
	"github.com/pusk-platform/pusk/internal/store"
)

// ── Per-bot webhook rate limiter ──

var (
	webhookLimits   sync.Map // bot token -> *rateBucket
	webhookLimitMax = 60     // requests per minute per bot
)

type rateBucket struct {
	mu    sync.Mutex
	count int
	reset time.Time
}

func init() {
	if v := os.Getenv("PUSK_WEBHOOK_RATE_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			webhookLimitMax = n
		}
	}
}

func webhookAllowed(token string) bool {
	now := time.Now()
	v, _ := webhookLimits.LoadOrStore(token, &rateBucket{reset: now.Add(time.Minute)})
	b := v.(*rateBucket)
	b.mu.Lock()
	defer b.mu.Unlock()
	if now.After(b.reset) {
		b.count = 0
		b.reset = now.Add(time.Minute)
	}
	b.count++
	return b.count <= webhookLimitMax
}

// WebhookHandler handles incoming webhooks from monitoring systems
// and converts them to Pusk channel messages.
//
// Usage:
//
//	POST /hook/{token}?format=alertmanager|zabbix|grafana|raw
//
// The token is a bot token. Messages are routed to:
// 1. Channel specified by ?channel= param, or
// 2. First existing channel owned by the bot, or
// 3. Auto-created "alerts" channel (last resort).
func (h *Handler) webhook(w http.ResponseWriter, r *http.Request) {
	bot, err := h.authBot(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if !webhookAllowed(bot.Token) {
		slog.Warn("webhook rate limited", "bot", bot.Name, "token_prefix", bot.Token[:8])
		w.WriteHeader(200)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "rate_limited"})
		return
	}

	s := h.db(r)
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "raw"
	}
	channelName := r.URL.Query().Get("channel")

	// Read body bytes for dedup check before parsing
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20) // 2MB max for webhooks
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(200)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "error": "read error"})
		return
	}

	// Deduplicate burst webhook calls (e.g. Alertmanager retries)
	if h.debounce != nil && h.debounce.IsDuplicate(bodyBytes) {
		slog.Info("webhook deduplicated", "format", format, "bot", bot.Name)
		metrics.WebhooksDedupedTotal.Inc()
		w.WriteHeader(200)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "deduped": "true"})
		return
	}

	// Parse JSON body
	var payload map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		// Try as array (some systems send arrays)
		w.WriteHeader(200) // Always 200 for webhook senders
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "error": "invalid json"})
		return
	}

	// Format message via template engine
	var text string
	switch format {
	case "alertmanager", "zabbix", "grafana", "raw":
		var err error
		text, err = h.templates.Render(format, payload)
		if err != nil {
			slog.Warn("template error, raw fallback", "format", format, "err", err)
			raw, _ := json.MarshalIndent(payload, "", "  ")
			text = "```json\n" + string(raw) + "\n```"
		}
	default:
		text = fmt.Sprintf("Webhook (%s):\n```json\n%s\n```", format, truncateStr(fmt.Sprintf("%v", payload), 500))
	}

	if text == "" {
		w.WriteHeader(200)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
	}

	// Find target channel:
	// 1. Explicit ?channel= param
	// 2. First channel owned by this bot
	// 3. Auto-create "alerts" as last resort
	var ch *store.Channel
	if channelName != "" {
		ch, err = s.ChannelByName(bot.ID, channelName)
	} else {
		ch, err = s.FirstChannelByBot(bot.ID)
	}
	if err != nil {
		if channelName == "" {
			channelName = "alerts"
		}
		ch, err = s.CreateChannel(bot.ID, channelName, "Webhook alerts")
		if err != nil {
			slog.Error("webhook channel create failed", "channel", channelName, "error", err)
			w.WriteHeader(200)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "error": "channel error"})
			return
		}
		_ = s.Subscribe(ch.ID, 1)
		slog.Info("webhook auto-created channel", "channel", channelName, "bot", bot.Name)
	}

	// Add ACK buttons for alert formats
	markup := ""
	if format != "raw" {
		markup = `{"inline_keyboard":[[{"text":"✓ ACK","callback_data":"ack"},{"text":"⏸ Mute 1h","callback_data":"mute"},{"text":"✓ Resolved","callback_data":"resolved"}]]}`
	}

	// Send message to channel
	msg, err := s.SaveChannelMessage(ch.ID, text, markup, "", "")
	if err != nil {
		slog.Error("webhook save failed", "error", err)
		w.WriteHeader(500)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error": "save failed"})
		return
	}

	// Push to subscribers via WebSocket (reuse existing push logic)
	if msg != nil {
		h.pushChannelMessage(s, ch, bot, msg)
	}

	metrics.WebhooksReceived.WithLabelValues(format).Inc()
	slog.Info("webhook received", "format", format, "channel", channelName, "bot", bot.Name)
	w.WriteHeader(200)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ── Helpers ──

func getStr(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// extractText tries to find a human-readable text field in webhook payload.
// Covers: Uptime Kuma (msg), generic webhooks (message, text).
func extractText(p map[string]interface{}) string {
	for _, key := range []string{"msg", "message", "text"} {
		if v := getStr(p, key); v != "" {
			return v
		}
	}
	return ""
}

func truncateStr(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "..."
}
