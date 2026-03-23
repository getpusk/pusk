// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package bot

import (
	"net/http"
	"strings"
)

// Route registers Bot API endpoints with token extraction middleware
// TelegramCompat rewrites /bot{token}/method → /bot/{token}/method for Telegram-native clients
func TelegramCompat(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if strings.HasPrefix(p, "/bot") && !strings.HasPrefix(p, "/bot/") && len(p) > 4 {
			// /botTOKEN/method → /bot/TOKEN/method
			r.URL.Path = "/bot/" + p[4:]
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) Route(mux *http.ServeMux) {
	// Bot API: /bot/{token}/method (Telegram-native /bot{token}/ handled by TelegramCompat middleware)
	mux.HandleFunc("POST /bot/", h.dispatch)
	mux.HandleFunc("GET /bot/", h.dispatchGet)
	mux.HandleFunc("GET /file/{fileID}", h.serveFile)
	mux.HandleFunc("POST /hook/", h.dispatchHook)
}

// extractTokenMethod parses both /bot/{token}/{method} and /bot{token}/{method} (Telegram-native)
func extractTokenMethod(path string) (token, method string, ok bool) {
	// Try /bot/{token}/{method} first
	if strings.HasPrefix(path, "/bot/") {
		parts := strings.SplitN(strings.TrimPrefix(path, "/bot/"), "/", 2)
		if len(parts) == 2 {
			return parts[0], parts[1], true
		}
	}
	// Telegram-native: /bot{token}/{method}
	if strings.HasPrefix(path, "/bot") {
		rest := strings.TrimPrefix(path, "/bot")
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) == 2 && parts[0] != "" {
			return parts[0], parts[1], true
		}
	}
	return "", "", false
}

func (h *Handler) dispatchGet(w http.ResponseWriter, r *http.Request) {
	token, method, ok := extractTokenMethod(r.URL.Path)
	if !ok {
		jsonResp(w, 400, APIResponse{OK: false, Error: "invalid path"})
		return
	}
	r.Header.Set("X-Bot-Token", token)
	switch method {
	case "relay":
		h.relayWebSocket(w, r)
	case "getMe":
		h.getMe(w, r)
	case "getUpdates":
		h.getUpdates(w, r)
	case "deleteWebhook":
		h.deleteWebhook(w, r)
	case "getWebhookInfo":
		h.getWebhookInfo(w, r)
	default:
		jsonResp(w, 400, APIResponse{OK: false, Error: "unknown GET method: " + method})
	}
}

func (h *Handler) dispatchHook(w http.ResponseWriter, r *http.Request) {
	// Path: /hook/<token>
	token := strings.TrimPrefix(r.URL.Path, "/hook/")
	token = strings.TrimSuffix(token, "/")
	if token == "" {
		http.Error(w, "missing token", 400)
		return
	}
	r.Header.Set("X-Bot-Token", token)
	h.webhook(w, r)
}

func (h *Handler) dispatch(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB max for bot API
	token, method, ok := extractTokenMethod(r.URL.Path)
	if !ok {
		jsonResp(w, 400, APIResponse{OK: false, Error: "invalid path, use /bot/<token>/<method>"})
		return
	}
	r.Header.Set("X-Bot-Token", token)

	switch method {
	case "sendMessage":
		h.sendMessage(w, r)
	case "editMessageText":
		h.editMessageText(w, r)
	case "deleteMessage":
		h.deleteMessage(w, r)
	case "answerCallbackQuery":
		h.answerCallback(w, r)
	case "sendPhoto":
		h.sendFile("photo")(w, r)
	case "sendDocument":
		h.sendFile("document")(w, r)
	case "sendVoice":
		h.sendFile("voice")(w, r)
	case "sendVideo":
		h.sendFile("video")(w, r)
	case "setWebhook":
		h.setWebhook(w, r)
	case "getMe":
		h.getMe(w, r)
	case "getUpdates":
		h.getUpdates(w, r)
	case "deleteWebhook":
		h.deleteWebhook(w, r)
	case "getWebhookInfo":
		h.getWebhookInfo(w, r)
	case "createChannel":
		h.createChannel(w, r)
	case "sendChannel":
		h.sendChannel(w, r)
	default:
		jsonResp(w, 400, APIResponse{OK: false, Error: "unknown method: " + method})
	}
}
