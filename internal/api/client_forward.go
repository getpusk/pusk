// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pusk-platform/pusk/internal/bot"
	"github.com/pusk-platform/pusk/internal/store"
)

// pushToUpdateQueue pushes message update SYNCHRONOUSLY before async forwarding.
func (a *ClientAPI) pushToUpdateQueue(s *store.Store, chatID, userID int64, msg *store.Message) {
	if a.updates == nil {
		return
	}
	botID, err := s.ChatBotID(chatID)
	if err != nil || botID == 0 {
		return
	}
	ts := func() int64 { t, _ := time.Parse(time.RFC3339, msg.CreatedAt); return t.Unix() }()
	msgPayload := map[string]interface{}{
		"message_id": msg.ID,
		"chat":       map[string]interface{}{"id": chatID, "type": "private"},
		"from":       map[string]interface{}{"id": userID, "is_bot": false, "first_name": "User"},
		"text":       msg.Text,
		"date":       ts,
	}
	if strings.HasPrefix(msg.Text, "/") {
		cmd := strings.SplitN(msg.Text, " ", 2)[0]
		msgPayload["entities"] = []map[string]interface{}{
			{"type": "bot_command", "offset": 0, "length": len(cmd)},
		}
	}
	a.updates.Push(botID, bot.Update{UpdateID: msg.ID, Message: msgPayload})
}

// pushCallbackToQueue pushes callback update SYNCHRONOUSLY before async forwarding.
func (a *ClientAPI) pushCallbackToQueue(s *store.Store, chatID, userID int64, data string, messageID int64) {
	if a.updates == nil {
		return
	}
	botID, err := s.ChatBotID(chatID)
	if err != nil || botID == 0 {
		return
	}

	// Look up real bot info
	botObj, _ := s.BotByID(botID)
	var botFromID int64
	botFromName := "Bot"
	if botObj != nil {
		botFromID = botObj.ID
		botFromName = botObj.Name
	}

	// Look up original message for text and date
	msgText := ""
	var msgDate int64
	if origMsg, err := s.GetMessage(messageID); err == nil && origMsg != nil {
		msgText = origMsg.Text
		if t, err := time.Parse(time.RFC3339, origMsg.CreatedAt); err == nil {
			msgDate = t.Unix()
		}
	}
	if msgDate == 0 {
		msgDate = time.Now().Unix()
	}

	cbUpdateID := time.Now().UnixMilli()
	cbPayload := map[string]interface{}{
		"id":            strconv.FormatInt(messageID, 10),
		"from":          map[string]interface{}{"id": userID, "is_bot": false, "first_name": "User"},
		"chat_instance": strconv.FormatInt(chatID, 10),
		"data":          data,
		"message": map[string]interface{}{
			"message_id": messageID,
			"date":       msgDate,
			"chat":       map[string]interface{}{"id": chatID, "type": "private"},
			"from":       map[string]interface{}{"id": botFromID, "is_bot": true, "first_name": botFromName},
			"text":       msgText,
		},
	}
	a.updates.Push(botID, bot.Update{UpdateID: cbUpdateID, Callback: cbPayload})
}

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

	ts := func() int64 { t, _ := time.Parse(time.RFC3339, msg.CreatedAt); return t.Unix() }()
	msgPayload := map[string]interface{}{
		"message_id": msg.ID,
		"chat":       map[string]interface{}{"id": chatID, "type": "private"},
		"from":       map[string]interface{}{"id": userID, "is_bot": false, "first_name": "User"},
		"text":       msg.Text,
		"date":       ts,
	}
	// Add entities for bot commands (PTB requires them to match CommandHandler)
	if strings.HasPrefix(msg.Text, "/") {
		cmd := strings.SplitN(msg.Text, " ", 2)[0]
		msgPayload["entities"] = []map[string]interface{}{
			{"type": "bot_command", "offset": 0, "length": len(cmd)},
		}
	}

	update := map[string]interface{}{
		"update_id": msg.ID,
		"message":   msgPayload,
	}

	// Update queue push is now done synchronously in pushToUpdateQueue

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
	slog.Info("forwardCallback called", "chat_id", chatID, "user_id", userID, "data", data, "msg_id", messageID)
	botID, err := s.ChatBotID(chatID)
	if err != nil || botID == 0 {
		slog.Warn("forwardCallback: no bot", "chat_id", chatID, "err", err)
		return
	}

	slog.Info("forwardCallback: got botID", "bot_id", botID)
	b, err := s.BotByID(botID)
	if err != nil {
		slog.Warn("forwardCallback: bot not found", "bot_id", botID)
		return
	}
	slog.Info("forwardCallback: pushing", "bot", b.Name, "updates_nil", a.updates == nil)

	// Look up original message for text and date
	msgText := ""
	var msgDate int64
	if origMsg, err := s.GetMessage(messageID); err == nil && origMsg != nil {
		msgText = origMsg.Text
		if t, err := time.Parse(time.RFC3339, origMsg.CreatedAt); err == nil {
			msgDate = t.Unix()
		}
	}
	if msgDate == 0 {
		msgDate = time.Now().Unix()
	}

	cbPayload := map[string]interface{}{
		"id":            strconv.FormatInt(messageID, 10),
		"from":          map[string]interface{}{"id": userID, "is_bot": false, "first_name": "User"},
		"chat_instance": strconv.FormatInt(chatID, 10),
		"data":          data,
		"message": map[string]interface{}{
			"message_id": messageID,
			"date":       msgDate,
			"chat":       map[string]interface{}{"id": chatID, "type": "private"},
			"from":       map[string]interface{}{"id": b.ID, "is_bot": true, "first_name": b.Name},
			"text":       msgText,
		},
	}

	// Update queue push is now done synchronously in pushCallbackToQueue
	update := map[string]interface{}{
		"update_id":      messageID,
		"callback_query": cbPayload,
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
