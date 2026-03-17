// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package bot

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pusk-platform/pusk/internal/store"
	"github.com/pusk-platform/pusk/internal/ws"
)

// Handler implements Telegram-compatible Bot API endpoints
type Handler struct {
	store    *store.Store
	hub      *ws.Hub
	filesDir string
}

func NewHandler(s *store.Store, hub *ws.Hub, filesDir string) *Handler {
	os.MkdirAll(filesDir, 0755)
	return &Handler{store: s, hub: hub, filesDir: filesDir}
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
	token := r.Header.Get("X-Bot-Token")
	if token == "" {
		return nil, fmt.Errorf("missing token")
	}
	return h.store.BotByToken(token)
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
		markup = string(req.ReplyMarkup)
	}

	msg, err := h.store.SaveMessage(req.ChatID, "bot", req.Text, markup, "", "")
	if err != nil {
		jsonResp(w, 500, APIResponse{OK: false, Error: err.Error()})
		return
	}

	// Find user for this chat and push via WebSocket
	h.pushMessageToChat(req.ChatID, bot, msg)

	jsonResp(w, 200, APIResponse{OK: true, Result: msg})
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
		markup = string(req.ReplyMarkup)
	}

	if err := h.store.UpdateMessageText(req.MessageID, req.Text, markup); err != nil {
		jsonResp(w, 500, APIResponse{OK: false, Error: err.Error()})
		return
	}

	msg, _ := h.store.GetMessage(req.MessageID)
	h.pushEditToChat(req.ChatID, bot, msg)

	jsonResp(w, 200, APIResponse{OK: true, Result: msg})
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

	h.store.DeleteMessage(req.MessageID)
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
	// In real impl: send notification to user via WS
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

	h.store.SetWebhook(bot.ID, req.URL)
	log.Printf("[bot] webhook set for %s: %s", bot.Name, req.URL)
	jsonResp(w, 200, APIResponse{OK: true, Result: true})
}

func (h *Handler) getMe(w http.ResponseWriter, r *http.Request) {
	bot, err := h.authBot(r)
	if err != nil {
		jsonResp(w, 401, APIResponse{OK: false, Error: "Unauthorized"})
		return
	}
	jsonResp(w, 200, APIResponse{OK: true, Result: map[string]interface{}{
		"id":       bot.ID,
		"is_bot":   true,
		"username": bot.Name,
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
		localPath := filepath.Join(h.filesDir, fileID+ext)

		dst, err := os.Create(localPath)
		if err != nil {
			jsonResp(w, 500, APIResponse{OK: false, Error: "cannot save file"})
			return
		}
		size, _ := io.Copy(dst, file)
		dst.Close()

		h.store.SaveFile(fileID, bot.ID, header.Filename,
			header.Header.Get("Content-Type"), size, localPath)

		text := caption
		if text == "" {
			text = "[" + fileType + "]"
		}

		msg, _ := h.store.SaveMessage(chatID, "bot", text, "", fileID, fileType)
		h.pushMessageToChat(chatID, bot, msg)

		jsonResp(w, 200, APIResponse{OK: true, Result: msg})
	}
}

func (h *Handler) serveFile(w http.ResponseWriter, r *http.Request) {
	fileID := r.PathValue("fileID")
	f, err := h.store.GetFile(fileID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, f.Path)
}

// ── WebSocket push ──

func (h *Handler) pushMessageToChat(chatID int64, bot *store.Bot, msg *store.Message) {
	userID, err := h.store.ChatUserID(chatID)
	if err != nil {
		log.Printf("[ws] cannot find user for chat %d: %v", chatID, err)
		return
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"message":  msg,
		"bot_name": bot.Name,
	})
	h.hub.SendToUser(userID, ws.Event{Type: "new_message", ChatID: chatID, Payload: payload})
}

func (h *Handler) pushEditToChat(chatID int64, bot *store.Bot, msg *store.Message) {
	userID, err := h.store.ChatUserID(chatID)
	if err != nil {
		return
	}
	payload, _ := json.Marshal(msg)
	h.hub.SendToUser(userID, ws.Event{Type: "edit_message", ChatID: chatID, Payload: payload})
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

	ch, err := h.store.CreateChannel(bot.ID, req.Name, req.Description)
	if err != nil {
		jsonResp(w, 500, APIResponse{OK: false, Error: err.Error()})
		return
	}

	log.Printf("[channel] created: %s (bot: %s)", ch.Name, bot.Name)
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

	ch, err := h.store.ChannelByName(bot.ID, req.Channel)
	if err != nil {
		jsonResp(w, 404, APIResponse{OK: false, Error: "channel not found: " + req.Channel})
		return
	}

	markup := ""
	if req.ReplyMarkup != nil {
		markup = string(req.ReplyMarkup)
	}

	msg, err := h.store.SaveChannelMessage(ch.ID, req.Text, markup, "", "")
	if err != nil {
		jsonResp(w, 500, APIResponse{OK: false, Error: err.Error()})
		return
	}

	// Push to all subscribers via WebSocket
	subs, _ := h.store.ChannelSubscribers(ch.ID)
	payload, _ := json.Marshal(map[string]interface{}{
		"message":      msg,
		"channel_name": ch.Name,
		"bot_name":     bot.Name,
	})
	for _, userID := range subs {
		h.hub.SendToUser(userID, ws.Event{Type: "channel_message", ChatID: ch.ID, Payload: payload})
	}

	log.Printf("[channel] %s: sent to %d subscribers", ch.Name, len(subs))
	jsonResp(w, 200, APIResponse{OK: true, Result: msg})
}
