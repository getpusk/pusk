// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"syscall"
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
	a.updates.Push(botID, bot.Update{Message: msgPayload})
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
	a.updates.Push(botID, bot.Update{Callback: cbPayload})
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

// webhookClient is the shared client for outbound webhook delivery. Its dialer
// validates the actual IP being connected to — on the initial request and on
// every redirect hop — so a host that resolves (now, or after a DNS rebind
// between IsLocalURL's check and the connection) to a loopback/private/
// link-local address is refused at connect time. This is the authoritative
// SSRF gate; bot.IsLocalURL is only a fast pre-filter.
var webhookClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: 10 * time.Second,
			Control: func(_, address string, _ syscall.RawConn) error {
				host, _, err := net.SplitHostPort(address)
				if err != nil {
					return err
				}
				ip := net.ParseIP(host)
				if ip == nil || bot.IsBlockedIP(ip) {
					return fmt.Errorf("refusing webhook connection to internal address %s", address)
				}
				return nil
			},
		}).DialContext,
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	},
	CheckRedirect: func(_ *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("stopped after 5 redirects")
		}
		return nil
	},
}

func sendWebhook(url string, payload interface{}) {
	data, _ := json.Marshal(payload)
	//nolint:gosec // G704: admin-configured URL; webhookClient's dialer blocks internal IPs (SSRF)
	resp, err := webhookClient.Post(url, "application/json", bytes.NewReader(data)) // #nosec G704
	if err != nil {
		slog.Error("webhook send failed", "url", url, "error", err)
		return
	}
	_ = resp.Body.Close()
	slog.Info("webhook sent", "url", url, "status", resp.StatusCode)
}
