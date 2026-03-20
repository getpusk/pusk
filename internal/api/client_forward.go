// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/pusk-platform/pusk/internal/bot"
	"github.com/pusk-platform/pusk/internal/store"
)

func (a *ClientAPI) forwardToBot(s *store.Store, chatID, userID int64, msg *store.Message) {
	botID, err := s.ChatBotID(chatID)
	if err != nil || botID == 0 {
		slog.Warn("no bot for chat", "chat_id", chatID)
		return
	}

	b, err := s.BotByID(botID)
	if err != nil {
		slog.Warn("bot not found", "bot_id", botID)
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

	if a.relay != nil && a.relay.Send(botID, update) {
		slog.Info("relay forwarded", "bot", b.Name, "transport", "ws")
		return
	}

	if b.WebhookURL == "" || bot.IsLocalURL(b.WebhookURL) {
		slog.Warn("bot unreachable", "bot", b.Name, "reason", "no webhook and not connected via relay")
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

	if a.relay != nil && a.relay.Send(botID, update) {
		return
	}

	if b.WebhookURL == "" || bot.IsLocalURL(b.WebhookURL) {
		return
	}
	sendWebhook(b.WebhookURL, update)
}

func sendWebhook(url string, payload interface{}) {
	data, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		slog.Error("webhook send failed", "url", url, "error", err)
		return
	}
	resp.Body.Close()
	slog.Info("webhook sent", "url", url, "status", resp.StatusCode)
}
