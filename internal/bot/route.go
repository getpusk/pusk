// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package bot

import (
	"net/http"
	"strings"
)

// Route registers Bot API endpoints with token extraction middleware
func (h *Handler) Route(mux *http.ServeMux) {
	// Bot API uses /bot<token>/method format
	// Go 1.22 can't do /bot{token}/method, so we use a prefix handler
	mux.HandleFunc("POST /bot/", h.dispatch)
	mux.HandleFunc("GET /file/{fileID}", h.serveFile)
}

func (h *Handler) dispatch(w http.ResponseWriter, r *http.Request) {
	// Path: /bot/<token>/<method>
	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/bot/"), "/", 2)
	if len(parts) != 2 {
		jsonResp(w, 400, APIResponse{OK: false, Error: "invalid path, use /bot/<token>/<method>"})
		return
	}

	token := parts[0]
	method := parts[1]

	// Inject token into request for authBot
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
	case "createChannel":
		h.createChannel(w, r)
	case "sendChannel":
		h.sendChannel(w, r)
	default:
		jsonResp(w, 400, APIResponse{OK: false, Error: "unknown method: " + method})
	}
}
