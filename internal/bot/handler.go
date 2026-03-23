// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package bot

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pusk-platform/pusk/internal/auth"
	"github.com/pusk-platform/pusk/internal/metrics"
	"github.com/pusk-platform/pusk/internal/notify"
	"github.com/pusk-platform/pusk/internal/org"
	"github.com/pusk-platform/pusk/internal/store"
	"github.com/pusk-platform/pusk/internal/ws"
)

// Handler implements Telegram-compatible Bot API endpoints
type Handler struct {
	orgs      *org.Manager
	store     *store.Store // default org store (backwards compat)
	hub       *ws.Hub
	push      *notify.PushService
	relay     *RelayHub
	updates   *UpdateQueue
	jwt       *auth.JWTService
	debounce  *Debouncer
	templates *TemplateEngine
	filesDir  string
}

func NewHandler(orgs *org.Manager, defaultStore *store.Store, hub *ws.Hub, push *notify.PushService, jwtSvc *auth.JWTService, filesDir string) *Handler {
	os.MkdirAll(filesDir, 0755)

	// Webhook debounce: PUSK_WEBHOOK_DEBOUNCE env (default 10s, "0" to disable)
	var deb *Debouncer
	if debStr := os.Getenv("PUSK_WEBHOOK_DEBOUNCE"); debStr == "0" {
		slog.Info("webhook debounce disabled")
	} else {
		window := 10 * time.Second
		if debStr != "" {
			if d, err := time.ParseDuration(debStr); err == nil {
				window = d
			} else {
				slog.Warn("invalid PUSK_WEBHOOK_DEBOUNCE, using default", "value", debStr, "error", err)
			}
		}
		deb = NewDebouncer(window)
		slog.Info("webhook debounce enabled", "window", window)
	}

	return &Handler{orgs: orgs, store: defaultStore, hub: hub, push: push, jwt: jwtSvc, debounce: deb, relay: NewRelayHub(), updates: NewUpdateQueue(), templates: NewTemplateEngine(), filesDir: filesDir}
}

// storeForJWT resolves org store from JWT token string
func (h *Handler) storeForJWT(tokenStr string) *store.Store {
	if h.jwt != nil && h.orgs != nil && tokenStr != "" {
		if claims, err := h.jwt.Validate(tokenStr); err == nil && claims.OrgID != "" {
			if s, err := h.orgs.Get(claims.OrgID); err == nil {
				return s
			}
		}
	}
	return h.store
}

// storeForRequest returns the Store for the bot's org based on token
func (h *Handler) storeForRequest(r *http.Request) *store.Store {
	token := r.Header.Get("X-Bot-Token")
	if h.orgs != nil && token != "" {
		if s, _, err := h.orgs.GetByToken(token); err == nil {
			return s
		}
	}
	return h.store
}

// Relay returns the relay hub for use by client API
func (h *Handler) Relay() *RelayHub { return h.relay }

// Updates returns the update queue for use by client API
func (h *Handler) Updates() *UpdateQueue { return h.updates }

// relayWebSocket handles GET /bot/{token}/relay — bot connects here
// to receive Telegram-compatible Updates via WebSocket instead of HTTP webhook.
func (h *Handler) relayWebSocket(w http.ResponseWriter, r *http.Request) {
	bot, err := h.authBot(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	wsConn, err := relayUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	conn := ws.NewConn(wsConn, bot.ID)
	h.relay.Register(bot.ID, conn)

	// Send confirmation
	conn.Send([]byte(`{"type":"connected","bot_id":` + strconv.FormatInt(bot.ID, 10) + `}`))

	go conn.WritePump()

	// ReadPump blocks — unregister on disconnect
	conn.ReadPump(nil, nil)
	h.relay.Unregister(bot.ID, conn)
}

// ── Telegram-compatible types ──

type SendMessageRequest struct {
	ChatID      int64           `json:"chat_id"`
	Text        string          `json:"text"`
	ReplyMarkup json.RawMessage `json:"reply_markup,omitempty"`
	ParseMode   string          `json:"parse_mode,omitempty"`
}

type EditMessageRequest struct {
	ChatID      int64           `json:"chat_id"`
	MessageID   int64           `json:"message_id"`
	Text        string          `json:"text"`
	ReplyMarkup json.RawMessage `json:"reply_markup,omitempty"`
}

type DeleteMessageRequest struct {
	ChatID    int64 `json:"chat_id"`
	MessageID int64 `json:"message_id"`
}

type AnswerCallbackRequest struct {
	CallbackQueryID string `json:"callback_query_id"`
	Text            string `json:"text,omitempty"`
	ShowAlert       bool   `json:"show_alert,omitempty"`
}

type SetWebhookRequest struct {
	URL string `json:"url"`
}

type APIResponse struct {
	OK     bool        `json:"ok"`
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"description,omitempty"`
}

// ── Route ──

// ── Helpers ──

func (h *Handler) authBot(r *http.Request) (*store.Bot, error) {
	s := h.storeForRequest(r)
	token := r.Header.Get("X-Bot-Token")
	if token == "" {
		return nil, fmt.Errorf("missing token")
	}
	return s.BotByToken(token)
}

// db returns the store for the current request (set by storeForRequest)
func (h *Handler) db(r *http.Request) *store.Store {
	return h.storeForRequest(r)
}

func jsonResp(w http.ResponseWriter, status int, resp APIResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

func decodeBody(r *http.Request, v interface{}) error {
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "application/json") {
		return json.NewDecoder(r.Body).Decode(v)
	}
	// form-urlencoded fallback
	if err := r.ParseForm(); err != nil {
		return err
	}
	data, _ := json.Marshal(formToMap(r))
	return json.Unmarshal(data, v)
}

func formToMap(r *http.Request) map[string]interface{} {
	m := make(map[string]interface{})
	for k, v := range r.Form {
		if len(v) == 1 {
			// Try to parse as number
			if n, err := strconv.ParseInt(v[0], 10, 64); err == nil {
				m[k] = n
			} else {
				m[k] = v[0]
			}
		}
	}
	return m
}

func randID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ── Handlers ──

func (h *Handler) sendMessage(w http.ResponseWriter, r *http.Request) {
	bot, err := h.authBot(r)
	if err != nil {
		jsonResp(w, 401, APIResponse{OK: false, Error: "Unauthorized"})
		return
	}

	var req SendMessageRequest
	if err := decodeBody(r, &req); err != nil {
		jsonResp(w, 400, APIResponse{OK: false, Error: err.Error()})
		return
	}

	markup := ""
	if req.ReplyMarkup != nil {
		raw := string(req.ReplyMarkup)
		// PTB may send reply_markup as JSON string; unwrap if quoted
		if len(raw) > 1 && raw[0] == '"' {
			var unquoted string
			if json.Unmarshal(req.ReplyMarkup, &unquoted) == nil {
				raw = unquoted
			}
		}
		markup = raw
	}

	s := h.db(r)

	// Negative chat_id = channel (Telegram convention: channels have negative IDs)
	if req.ChatID < 0 {
		channelID := -req.ChatID
		ch, err := s.ChannelByID(channelID)
		if err != nil {
			jsonResp(w, 404, APIResponse{OK: false, Error: "channel not found"})
			return
		}
		chMsg, _ := s.SaveChannelMessage(ch.ID, req.Text, markup, "", "")
		if chMsg != nil {
			h.pushChannelMessage(s, ch, bot, chMsg)
		}
		metrics.MessagesSent.WithLabelValues("channel").Inc()
		jsonResp(w, 200, APIResponse{OK: true, Result: telegramChannelMessage(chMsg)})
		return
	}

	msg, err := s.SaveMessage(req.ChatID, "bot", req.Text, markup, "", "")
	if err != nil {
		// Fallback: try chat_id as channel_id
		ch, chErr := s.ChannelByID(req.ChatID)
		if chErr == nil {
			chMsg, _ := s.SaveChannelMessage(ch.ID, req.Text, markup, "", "")
			if chMsg != nil {
				h.pushChannelMessage(s, ch, bot, chMsg)
				metrics.MessagesSent.WithLabelValues("channel").Inc()
				jsonResp(w, 200, APIResponse{OK: true, Result: telegramChannelMessage(chMsg)})
				return
			}
		}
		jsonResp(w, 500, APIResponse{OK: false, Error: err.Error()})
		return
	}

	// Find user for this chat and push via WebSocket
	h.pushMessageToChat(s, req.ChatID, bot, msg)
	metrics.MessagesSent.WithLabelValues("chat").Inc()

	jsonResp(w, 200, APIResponse{OK: true, Result: telegramMessage(msg, bot)})
}

func (h *Handler) editMessageText(w http.ResponseWriter, r *http.Request) {
	bot, err := h.authBot(r)
	if err != nil {
		jsonResp(w, 401, APIResponse{OK: false, Error: "Unauthorized"})
		return
	}

	var req EditMessageRequest
	if err := decodeBody(r, &req); err != nil {
		jsonResp(w, 400, APIResponse{OK: false, Error: err.Error()})
		return
	}

	markup := ""
	if req.ReplyMarkup != nil {
		raw := string(req.ReplyMarkup)
		if len(raw) > 1 && raw[0] == '"' {
			var unquoted string
			if json.Unmarshal(req.ReplyMarkup, &unquoted) == nil {
				raw = unquoted
			}
		}
		markup = raw
	}

	if err := h.db(r).UpdateMessageText(req.MessageID, req.Text, markup); err != nil {
		jsonResp(w, 500, APIResponse{OK: false, Error: err.Error()})
		return
	}

	msg, _ := h.db(r).GetMessage(req.MessageID)
	h.pushEditToChat(h.db(r), req.ChatID, bot, msg)

	jsonResp(w, 200, APIResponse{OK: true, Result: telegramMessage(msg, bot)})
}

func (h *Handler) deleteMessage(w http.ResponseWriter, r *http.Request) {
	_, err := h.authBot(r)
	if err != nil {
		jsonResp(w, 401, APIResponse{OK: false, Error: "Unauthorized"})
		return
	}

	var req DeleteMessageRequest
	if err := decodeBody(r, &req); err != nil {
		jsonResp(w, 400, APIResponse{OK: false, Error: err.Error()})
		return
	}

	h.db(r).DeleteMessage(req.MessageID)
	jsonResp(w, 200, APIResponse{OK: true, Result: true})
}

func (h *Handler) answerCallback(w http.ResponseWriter, r *http.Request) {
	_, err := h.authBot(r)
	if err != nil {
		jsonResp(w, 401, APIResponse{OK: false, Error: "Unauthorized"})
		return
	}

	var req AnswerCallbackRequest
	decodeBody(r, &req)

	if req.Text != "" {
		// Parse callback_query_id (set to messageID string in pushCallbackToQueue)
		msgID, _ := strconv.ParseInt(req.CallbackQueryID, 10, 64)
		if msgID > 0 {
			s := h.db(r)
			if msg, err := s.GetMessage(msgID); err == nil && msg != nil {
				if userID, err := s.ChatUserID(msg.ChatID); err == nil && userID > 0 {
					key := s.OrgID + ":" + fmt.Sprintf("%d", userID)
					payload, _ := json.Marshal(map[string]interface{}{
						"text":       req.Text,
						"show_alert": req.ShowAlert,
					})
					h.hub.SendToUser(key, ws.Event{Type: "callback_answer", Payload: payload})
				}
			}
		}
	}

	jsonResp(w, 200, APIResponse{OK: true, Result: true})
}

func (h *Handler) setWebhook(w http.ResponseWriter, r *http.Request) {
	bot, err := h.authBot(r)
	if err != nil {
		jsonResp(w, 401, APIResponse{OK: false, Error: "Unauthorized"})
		return
	}

	var req SetWebhookRequest
	if err := decodeBody(r, &req); err != nil {
		jsonResp(w, 400, APIResponse{OK: false, Error: err.Error()})
		return
	}

	if req.URL != "" && IsLocalURL(req.URL) {
		jsonResp(w, 400, APIResponse{OK: false, Error: "local URLs not allowed for webhooks"})
		return
	}
	h.db(r).SetWebhook(bot.ID, req.URL)
	slog.Info("webhook set", "bot", bot.Name, "url", req.URL)
	jsonResp(w, 200, APIResponse{OK: true, Result: true})
}

func (h *Handler) getMe(w http.ResponseWriter, r *http.Request) {
	bot, err := h.authBot(r)
	if err != nil {
		jsonResp(w, 401, APIResponse{OK: false, Error: "Unauthorized"})
		return
	}
	jsonResp(w, 200, APIResponse{OK: true, Result: map[string]interface{}{
		"id":         bot.ID,
		"is_bot":     true,
		"username":   bot.Name,
		"first_name": bot.Name,
	}})
}

func (h *Handler) sendFile(fileType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bot, err := h.authBot(r)
		if err != nil {
			jsonResp(w, 401, APIResponse{OK: false, Error: "Unauthorized"})
			return
		}

		r.ParseMultipartForm(50 << 20) // 50MB max

		chatID, _ := strconv.ParseInt(r.FormValue("chat_id"), 10, 64)
		caption := r.FormValue("caption")

		file, header, err := r.FormFile(fileType)
		if err != nil {
			jsonResp(w, 400, APIResponse{OK: false, Error: "missing file field: " + fileType})
			return
		}
		defer file.Close()

		fileID := randID()
		ext := filepath.Ext(header.Filename)
		// Per-org file directory
		orgID := "default"
		if h.orgs != nil {
			token := r.Header.Get("X-Bot-Token")
			if _, slug, err := h.orgs.GetByToken(token); err == nil && slug != "" {
				orgID = slug
			}
		}
		orgDir := filepath.Join(h.filesDir, orgID)
		os.MkdirAll(orgDir, 0755)
		localPath := filepath.Join(orgDir, fileID+ext)

		dst, err := os.Create(localPath)
		if err != nil {
			jsonResp(w, 500, APIResponse{OK: false, Error: "cannot save file"})
			return
		}
		size, _ := io.Copy(dst, file)
		dst.Close()

		// Check storage quota (default 1GB, PUSK_FILE_QUOTA_MB env)
		quotaMB := int64(1024)
		if v := os.Getenv("PUSK_FILE_QUOTA_MB"); v != "" {
			if q, err := strconv.ParseInt(v, 10, 64); err == nil && q > 0 {
				quotaMB = q
			}
		}
		if h.db(r).TotalFileSize()+size > quotaMB*1024*1024 {
			os.Remove(localPath)
			jsonResp(w, 400, APIResponse{OK: false, Error: "storage quota exceeded"})
			return
		}

		h.db(r).SaveFile(fileID, bot.ID, header.Filename,
			header.Header.Get("Content-Type"), size, localPath)

		text := caption
		if text == "" {
			text = "[" + fileType + "]"
		}

		msg, _ := h.db(r).SaveMessage(chatID, "bot", text, "", fileID, fileType)
		h.pushMessageToChat(h.db(r), chatID, bot, msg)

		jsonResp(w, 200, APIResponse{OK: true, Result: telegramMessage(msg, bot)})
	}
}

func (h *Handler) serveFile(w http.ResponseWriter, r *http.Request) {
	// Auth: require JWT or file-token via Authorization header or ?token= query
	tokenStr := r.Header.Get("Authorization")
	if tokenStr == "" {
		tokenStr = r.URL.Query().Get("token")
	}
	if tokenStr == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	fileID := r.PathValue("fileID")

	// Try JWT first
	s := h.storeForJWT(tokenStr)
	f, err := s.GetFile(fileID)
	if err == nil {
		http.ServeFile(w, r, f.Path)
		return
	}

	// Try as file-token (opaque 32-hex-char token)
	if h.orgs != nil {
		for _, o := range h.orgs.List() {
			if os, err := h.orgs.Get(o.Slug); err == nil {
				if _, err := os.ValidateFileToken(tokenStr); err == nil {
					f, err := os.GetFile(fileID)
					if err == nil {
						http.ServeFile(w, r, f.Path)
						return
					}
				}
			}
		}
	}
	// Fallback: try default store file-token
	if h.store != nil {
		if _, err := h.store.ValidateFileToken(tokenStr); err == nil {
			f, err := h.store.GetFile(fileID)
			if err == nil {
				http.ServeFile(w, r, f.Path)
				return
			}
		}
	}

	http.NotFound(w, r)
}

// ── WebSocket push ──

func (h *Handler) pushMessageToChat(s *store.Store, chatID int64, bot *store.Bot, msg *store.Message) {
	userID, err := s.ChatUserID(chatID)
	if err != nil {
		slog.Warn("cannot find user for chat", "chat_id", chatID, "error", err)
		return
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"message":  msg,
		"bot_name": bot.Name,
	})
	key := s.OrgID + ":" + fmt.Sprintf("%d", userID)
	h.hub.SendToUser(key, ws.Event{Type: "new_message", ChatID: chatID, Payload: payload})
	// Push notification (use org store for subscription lookup)
	h.push.SendToUser(s, userID, notify.PushPayload{
		Title: bot.Name,
		Body:  truncate(msg.Text, 100),
		Tag:   "chat-" + fmt.Sprintf("%d", chatID),
		URL:   "/",
	})
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func (h *Handler) pushEditToChat(s *store.Store, chatID int64, bot *store.Bot, msg *store.Message) {
	userID, err := s.ChatUserID(chatID)
	if err != nil {
		return
	}
	key := s.OrgID + ":" + fmt.Sprintf("%d", userID)
	payload, _ := json.Marshal(msg)
	h.hub.SendToUser(key, ws.Event{Type: "edit_message", ChatID: chatID, Payload: payload})
}

func (h *Handler) pushChannelMessage(s *store.Store, ch *store.Channel, bot *store.Bot, msg *store.ChannelMessage) {
	subs, _ := s.ChannelSubscribers(ch.ID)
	payload, _ := json.Marshal(map[string]interface{}{
		"message":      msg,
		"channel_name": ch.Name,
		"bot_name":     bot.Name,
	})
	for _, userID := range subs {
		key := s.OrgID + ":" + fmt.Sprintf("%d", userID)
		h.hub.SendToUser(key, ws.Event{Type: "channel_message", ChatID: ch.ID, Payload: payload})
		h.push.SendToUser(s, userID, notify.PushPayload{
			Title: "#" + ch.Name,
			Body:  truncate(msg.Text, 100),
			Tag:   "channel-" + ch.Name,
			URL:   "/",
		})
	}
}

// ── Channel handlers ──

type CreateChannelRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type SendChannelRequest struct {
	Channel     string          `json:"channel"`
	Text        string          `json:"text"`
	ReplyMarkup json.RawMessage `json:"reply_markup,omitempty"`
}

func (h *Handler) createChannel(w http.ResponseWriter, r *http.Request) {
	bot, err := h.authBot(r)
	if err != nil {
		jsonResp(w, 401, APIResponse{OK: false, Error: "Unauthorized"})
		return
	}

	var req CreateChannelRequest
	if err := decodeBody(r, &req); err != nil {
		jsonResp(w, 400, APIResponse{OK: false, Error: err.Error()})
		return
	}

	ch, err := h.db(r).CreateChannel(bot.ID, req.Name, req.Description)
	if err != nil {
		jsonResp(w, 500, APIResponse{OK: false, Error: err.Error()})
		return
	}

	slog.Info("channel created", "channel", ch.Name, "bot", bot.Name)
	jsonResp(w, 200, APIResponse{OK: true, Result: ch})
}

func (h *Handler) sendChannel(w http.ResponseWriter, r *http.Request) {
	bot, err := h.authBot(r)
	if err != nil {
		jsonResp(w, 401, APIResponse{OK: false, Error: "Unauthorized"})
		return
	}

	var req SendChannelRequest
	if err := decodeBody(r, &req); err != nil {
		jsonResp(w, 400, APIResponse{OK: false, Error: err.Error()})
		return
	}

	ch, err := h.db(r).ChannelByName(bot.ID, req.Channel)
	if err != nil {
		jsonResp(w, 404, APIResponse{OK: false, Error: "channel not found: " + req.Channel})
		return
	}

	markup := ""
	if req.ReplyMarkup != nil {
		raw := string(req.ReplyMarkup)
		if len(raw) > 1 && raw[0] == '"' {
			var unquoted string
			if json.Unmarshal(req.ReplyMarkup, &unquoted) == nil {
				raw = unquoted
			}
		}
		markup = raw
	}

	msg, err := h.db(r).SaveChannelMessage(ch.ID, req.Text, markup, "", "")
	if err != nil {
		jsonResp(w, 500, APIResponse{OK: false, Error: err.Error()})
		return
	}

	// Push to all subscribers
	h.pushChannelMessage(h.db(r), ch, bot, msg)
	metrics.MessagesSent.WithLabelValues("channel").Inc()

	slog.Info("channel message sent", "channel", ch.Name)
	jsonResp(w, 200, APIResponse{OK: true, Result: telegramChannelMessage(msg)})
}

// ── Long Polling ──

func (h *Handler) getUpdates(w http.ResponseWriter, r *http.Request) {
	bot, err := h.authBot(r)
	if err != nil {
		jsonResp(w, 401, APIResponse{OK: false, Error: "Unauthorized"})
		return
	}

	var req struct {
		Offset  int64 `json:"offset"`
		Timeout int   `json:"timeout"`
	}

	// Support both JSON body and query params
	if r.Method == "POST" {
		decodeBody(r, &req)
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		req.Offset, _ = strconv.ParseInt(v, 10, 64)
	}
	if v := r.URL.Query().Get("timeout"); v != "" {
		req.Timeout, _ = strconv.Atoi(v)
	}
	if req.Timeout <= 0 {
		req.Timeout = 30
	}
	if req.Timeout > 50 {
		req.Timeout = 50
	}

	updates := h.updates.Poll(bot.ID, req.Offset, time.Duration(req.Timeout)*time.Second)
	if updates == nil {
		updates = []Update{}
	}
	if len(updates) > 0 {
		slog.Info("getUpdates delivering", "bot", bot.Name, "count", len(updates), "offset", req.Offset)
	}

	jsonResp(w, 200, APIResponse{OK: true, Result: updates})
}

func (h *Handler) deleteWebhook(w http.ResponseWriter, r *http.Request) {
	bot, err := h.authBot(r)
	if err != nil {
		jsonResp(w, 401, APIResponse{OK: false, Error: "Unauthorized"})
		return
	}
	h.db(r).SetWebhook(bot.ID, "")
	slog.Info("webhook deleted", "bot", bot.Name)
	jsonResp(w, 200, APIResponse{OK: true, Result: true})
}

func (h *Handler) getWebhookInfo(w http.ResponseWriter, r *http.Request) {
	bot, err := h.authBot(r)
	if err != nil {
		jsonResp(w, 401, APIResponse{OK: false, Error: "Unauthorized"})
		return
	}
	jsonResp(w, 200, APIResponse{OK: true, Result: map[string]interface{}{
		"url":                    bot.WebhookURL,
		"has_custom_certificate": false,
		"pending_update_count":   0,
	}})
}

// addFileFields adds file-related fields (photo/document/voice/video) to a Telegram-compatible message map.
func addFileFields(m map[string]interface{}, fileID, fileType string) {
	if fileID == "" {
		return
	}
	fileObj := []map[string]interface{}{{"file_id": fileID}}
	switch fileType {
	case "photo":
		m["photo"] = fileObj
	case "voice":
		m["voice"] = map[string]interface{}{"file_id": fileID}
	case "video":
		m["video"] = map[string]interface{}{"file_id": fileID}
	default:
		m["document"] = map[string]interface{}{"file_id": fileID}
	}
}

// telegramMessage converts a store.Message to Telegram-compatible format.
// PTB expects: message_id, chat{id,type}, from{id,is_bot,first_name}, date(unix), text, reply_markup, photo/document/voice/video.
func telegramMessage(msg *store.Message, bot *store.Bot) map[string]interface{} {
	t, _ := time.Parse(time.RFC3339, msg.CreatedAt)
	m := map[string]interface{}{
		"message_id": msg.ID,
		"chat":       map[string]interface{}{"id": msg.ChatID, "type": "private"},
		"date":       t.Unix(),
		"text":       msg.Text,
	}
	if msg.Sender == "bot" && bot != nil {
		m["from"] = map[string]interface{}{"id": bot.ID, "is_bot": true, "first_name": bot.Name}
	} else if msg.Sender == "bot" {
		m["from"] = map[string]interface{}{"id": 0, "is_bot": true, "first_name": "Bot"}
	} else {
		m["from"] = map[string]interface{}{"id": msg.ChatID, "is_bot": false, "first_name": "User"}
	}
	if msg.ReplyMarkup != "" {
		m["reply_markup"] = json.RawMessage(msg.ReplyMarkup)
	}
	addFileFields(m, msg.FileID, msg.FileType)
	return m
}

// telegramChannelMessage converts a store.ChannelMessage to Telegram-compatible format.
// Uses negative channel_id and "channel" chat type, matching Telegram convention.
func telegramChannelMessage(msg *store.ChannelMessage) map[string]interface{} {
	t, _ := time.Parse(time.RFC3339, msg.CreatedAt)
	m := map[string]interface{}{
		"message_id": msg.ID,
		"chat":       map[string]interface{}{"id": -msg.ChannelID, "type": "channel"},
		"from":       map[string]interface{}{"id": 0, "is_bot": true, "first_name": "Bot"},
		"date":       t.Unix(),
		"text":       msg.Text,
	}
	if msg.ReplyMarkup != "" {
		m["reply_markup"] = json.RawMessage(msg.ReplyMarkup)
	}
	addFileFields(m, msg.FileID, msg.FileType)
	return m
}
