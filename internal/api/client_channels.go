// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package api

import (
	"bytes"
	crand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pusk-platform/pusk/internal/notify"
	"github.com/pusk-platform/pusk/internal/store"
	"github.com/pusk-platform/pusk/internal/ws"
)

func (a *ClientAPI) ackChannelMessage(w http.ResponseWriter, r *http.Request) {
	channelID, _ := strconv.ParseInt(r.PathValue("channelID"), 10, 64)
	s := a.db(r)

	var req struct {
		MessageID int64  `json:"message_id"`
		Action    string `json:"action"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	claims := ClaimsFromCtx(r.Context())
	username := ""
	if claims != nil {
		username = claims.Username
	}

	msgs, _ := s.ChannelMessages(channelID, 200)
	for _, m := range msgs {
		if m.ID == req.MessageID {
			now := time.Now().Format("15:04")
			var status string
			switch req.Action {
			case "ack":
				status = fmt.Sprintf("\n\n**ACK**: @%s в %s", username, now)
			case "mute":
				status = fmt.Sprintf("\n\n**Muted 1h**: @%s в %s", username, now)
			case "resolved":
				status = fmt.Sprintf("\n\n**Resolved**: @%s в %s", username, now)
			}
			newText := m.Text + status
			s.UpdateChannelMessageText(m.ID, newText, "")

			// Alertmanager silence on ACK/mute
			amURL := os.Getenv("PUSK_ALERTMANAGER_URL")
			if amURL != "" && (req.Action == "ack" || req.Action == "mute") {
				go createAlertmanagerSilence(amURL, username, m.Text)
			}

			json.NewEncoder(w).Encode(map[string]bool{"ok": true})
			return
		}
	}
	jsonErr(w, "message not found", 404)
}

func (a *ClientAPI) listChannels(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromCtx(r.Context())
	channels, err := a.db(r).ListChannels()
	if err != nil {
		jsonErr(w, "internal error", 500)
		return
	}
	type channelInfo struct {
		ID              int64  `json:"id"`
		Name            string `json:"name"`
		Description     string `json:"description,omitempty"`
		Subscribed      bool   `json:"subscribed"`
		Unread          int    `json:"unread"`
		PinnedMessageID int64  `json:"pinned_message_id"`
	}
	s := a.db(r)
	result := make([]channelInfo, 0, len(channels))
	for _, ch := range channels {
		result = append(result, channelInfo{
			ID: ch.ID, Name: ch.Name, Description: ch.Description, PinnedMessageID: s.GetPinnedMessage(ch.ID),
			Subscribed: s.IsSubscribed(ch.ID, userID),
			Unread:     s.UnreadCount(ch.ID, userID),
		})
	}
	json.NewEncoder(w).Encode(result)
}

func (a *ClientAPI) subscribe(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromCtx(r.Context())
	channelID, _ := strconv.ParseInt(r.PathValue("channelID"), 10, 64)
	if err := a.db(r).Subscribe(channelID, userID); err != nil {
		jsonErr(w, "internal error", 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (a *ClientAPI) unsubscribe(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromCtx(r.Context())
	channelID, _ := strconv.ParseInt(r.PathValue("channelID"), 10, 64)
	if err := a.db(r).Unsubscribe(channelID, userID); err != nil {
		jsonErr(w, "internal error", 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (a *ClientAPI) channelMessages(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromCtx(r.Context())
	channelID, _ := strconv.ParseInt(r.PathValue("channelID"), 10, 64)
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		limit, _ = strconv.Atoi(l)
	}
	if limit > 200 {
		limit = 200
	}
	s := a.db(r)
	msgs, err := s.ChannelMessages(channelID, limit)
	if err != nil {
		jsonErr(w, "internal error", 500)
		return
	}
	if msgs == nil {
		msgs = []store.ChannelMessage{}
	}
	if len(msgs) > 0 {
		s.MarkChannelRead(channelID, userID, msgs[0].ID)
	}
	json.NewEncoder(w).Encode(msgs)
}

func (a *ClientAPI) sendToChannel(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromCtx(r.Context())
	channelID, _ := strconv.ParseInt(r.PathValue("channelID"), 10, 64)
	s := a.db(r)

	if !s.IsSubscribed(channelID, userID) {
		jsonErr(w, "not subscribed", 403)
		return
	}

	var req struct {
		Text    string `json:"text"`
		ReplyTo int64  `json:"reply_to"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Text == "" {
		jsonErr(w, "text required", 400)
		return
	}

	claims := ClaimsFromCtx(r.Context())
	username := ""
	if claims != nil {
		username = claims.Username
	}

	msg, err := s.SaveChannelMessageFrom(channelID, "user", username, req.Text, "", "", "")
	if msg != nil && req.ReplyTo > 0 {
		s.SetChannelMessageReplyTo(msg.ID, req.ReplyTo)
		msg.ReplyTo = req.ReplyTo
	}
	if err != nil {
		jsonErr(w, "internal error", 500)
		return
	}
	if msg != nil {
		s.MarkChannelRead(channelID, userID, msg.ID)
	}

	ch, _ := s.ChannelByID(channelID)
	if ch != nil {
		orgID := ""
		if claims != nil {
			orgID = claims.OrgID
		}
		subs, _ := s.ChannelSubscribers(ch.ID)
		payload, _ := json.Marshal(map[string]interface{}{
			"message":      msg,
			"channel_name": ch.Name,
			"sender_name":  username,
		})
		for _, uid := range subs {
			key := orgID + ":" + fmt.Sprintf("%d", uid)
			a.hub.SendToUser(key, ws.Event{Type: "channel_message", ChatID: ch.ID, Payload: payload})
		}

		// @mentions: push notification to mentioned users
		if strings.Contains(req.Text, "@") {
			users, _ := s.ListUsers()
			for _, u := range users {
				if strings.Contains(req.Text, "@"+u.Username) && u.ID != userID {
					mentionPayload, _ := json.Marshal(map[string]interface{}{
						"type":    "mention",
						"channel": ch.Name,
						"from":    username,
						"text":    req.Text,
					})
					key := orgID + ":" + fmt.Sprintf("%d", u.ID)
					a.hub.SendToUser(key, ws.Event{Type: "mention", ChatID: ch.ID, Payload: mentionPayload})
					a.push.SendToUser(s, u.ID, notify.PushPayload{
						Title: "#" + ch.Name + " — @" + u.Username,
						Body:  username + ": " + truncateText(req.Text, 80),
						Tag:   "mention-" + ch.Name,
						URL:   "/",
					})
				}
			}
		}
	}

	json.NewEncoder(w).Encode(msg)
}

func (a *ClientAPI) pushSubscribe(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromCtx(r.Context())
	var req struct {
		Endpoint string `json:"endpoint"`
		Keys     struct {
			P256dh string `json:"p256dh"`
			Auth   string `json:"auth"`
		} `json:"keys"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if err := a.db(r).SavePushSubscription(userID, req.Endpoint, req.Keys.P256dh, req.Keys.Auth); err != nil {
		jsonErr(w, "internal error", 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (a *ClientAPI) vapidKey(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"key": a.vapidPub})
}

func (a *ClientAPI) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := a.db(r).ListUsers()
	if err != nil {
		jsonErr(w, "internal error", 500)
		return
	}
	json.NewEncoder(w).Encode(users)
}

func (a *ClientAPI) setUserRole(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromCtx(r.Context())
	s := a.db(r)
	if !s.IsAdmin(userID) {
		jsonErr(w, "admin only", 403)
		return
	}
	targetID, _ := strconv.ParseInt(r.PathValue("userID"), 10, 64)
	var req struct {
		Role string `json:"role"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Role != "admin" && req.Role != "member" {
		jsonErr(w, "role must be admin or member", 400)
		return
	}
	s.SetUserRole(targetID, req.Role)
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (a *ClientAPI) deleteUser(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromCtx(r.Context())
	s := a.db(r)
	if !s.IsAdmin(userID) {
		jsonErr(w, "admin only", 403)
		return
	}
	targetID, _ := strconv.ParseInt(r.PathValue("userID"), 10, 64)
	if targetID == userID {
		jsonErr(w, "cannot delete yourself", 400)
		return
	}
	if targetID == 1 {
		jsonErr(w, "cannot delete primary admin", 400)
		return
	}
	s.DeleteUser(targetID)
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (a *ClientAPI) editChannelMessage(w http.ResponseWriter, r *http.Request) {
	channelID, _ := strconv.ParseInt(r.PathValue("channelID"), 10, 64)
	msgID, _ := strconv.ParseInt(r.PathValue("msgID"), 10, 64)
	s := a.db(r)

	msg, err := s.GetChannelMessage(msgID)
	if err != nil {
		jsonErr(w, "not found", 404)
		return
	}

	claims := ClaimsFromCtx(r.Context())
	if claims == nil || msg.SenderName != claims.Username {
		jsonErr(w, "can only edit own messages", 403)
		return
	}

	var req struct {
		Text string `json:"text"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	s.UpdateChannelMessageText(msgID, req.Text, "")

	updated, _ := s.GetChannelMessage(msgID)
	if updated != nil {
		orgID := ""
		if claims != nil {
			orgID = claims.OrgID
		}
		subs, _ := s.ChannelSubscribers(channelID)
		payload, _ := json.Marshal(updated)
		for _, uid := range subs {
			key := orgID + ":" + fmt.Sprintf("%d", uid)
			a.hub.SendToUser(key, ws.Event{Type: "channel_message_edit", ChatID: channelID, Payload: payload})
		}
	}

	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (a *ClientAPI) deleteChannelMessage(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromCtx(r.Context())
	msgID, _ := strconv.ParseInt(r.PathValue("msgID"), 10, 64)
	s := a.db(r)

	msg, err := s.GetChannelMessage(msgID)
	if err != nil {
		jsonErr(w, "not found", 404)
		return
	}

	claims := ClaimsFromCtx(r.Context())
	isAuthor := claims != nil && msg.SenderName == claims.Username
	isAdmin := s.IsAdmin(userID)
	if !isAuthor && !isAdmin {
		jsonErr(w, "forbidden", 403)
		return
	}

	orgID := ""
	if claims != nil {
		orgID = claims.OrgID
	}
	subs, _ := s.ChannelSubscribers(msg.ChannelID)
	payload, _ := json.Marshal(map[string]int64{"message_id": msgID})
	for _, uid := range subs {
		key := orgID + ":" + fmt.Sprintf("%d", uid)
		a.hub.SendToUser(key, ws.Event{Type: "channel_message_delete", ChatID: msg.ChannelID, Payload: payload})
	}

	s.DeleteChannelMessage(msgID)
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (a *ClientAPI) onlineUsers(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromCtx(r.Context())
	prefix := claims.OrgID + ":"
	var users []int64
	for _, key := range a.hub.OnlineKeys() {
		if strings.HasPrefix(key, prefix) {
			parts := strings.SplitN(key, ":", 2)
			if len(parts) == 2 {
				if uid, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
					users = append(users, uid)
				}
			}
		}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"online": len(users), "user_ids": users})
}

func (a *ClientAPI) pinMessage(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromCtx(r.Context())
	channelID, _ := strconv.ParseInt(r.PathValue("channelID"), 10, 64)
	s := a.db(r)
	if !s.IsAdmin(userID) {
		jsonErr(w, "admin only", 403)
		return
	}
	var req struct {
		MessageID int64 `json:"message_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	s.PinMessage(channelID, req.MessageID)
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func truncateText(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func createAlertmanagerSilence(amURL, username, alertText string) {
	re := regexp.MustCompile("`([^`]+)`")
	matches := re.FindStringSubmatch(alertText)
	alertname := "unknown"
	if len(matches) > 1 {
		alertname = matches[1]
	}

	now := time.Now()
	silence := map[string]interface{}{
		"matchers": []map[string]string{
			{"name": "alertname", "value": alertname, "isRegex": "false"},
		},
		"startsAt":  now.Format(time.RFC3339),
		"endsAt":    now.Add(1 * time.Hour).Format(time.RFC3339),
		"createdBy": username,
		"comment":   "ACK'd via Pusk",
	}

	data, _ := json.Marshal(silence)
	resp, err := http.Post(amURL+"/api/v2/silences", "application/json", bytes.NewReader(data))
	if err != nil {
		slog.Warn("alertmanager silence failed", "error", err)
		return
	}
	resp.Body.Close()
	slog.Info("alertmanager silence created", "alertname", alertname, "by", username, "status", resp.StatusCode)
}

func randID() string {
	b := make([]byte, 16)
	crand.Read(b)
	return hex.EncodeToString(b)
}

func (a *ClientAPI) uploadToChannel(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromCtx(r.Context())
	channelID, _ := strconv.ParseInt(r.PathValue("channelID"), 10, 64)
	s := a.db(r)

	if !s.IsSubscribed(channelID, userID) {
		jsonErr(w, "not subscribed", 403)
		return
	}

	r.ParseMultipartForm(10 << 20) // 10MB max

	file, header, err := r.FormFile("file")
	if err != nil {
		jsonErr(w, "file required", 400)
		return
	}
	defer file.Close()

	// Determine file type from content-type
	ct := header.Header.Get("Content-Type")
	fileType := "document"
	if strings.HasPrefix(ct, "image/") {
		fileType = "photo"
	}
	if strings.HasPrefix(ct, "video/") {
		fileType = "video"
	}
	if strings.HasPrefix(ct, "audio/") {
		fileType = "voice"
	}

	// Save file in per-org directory
	fileID := randID()
	ext := filepath.Ext(header.Filename)
	orgID := ""
	if claims := ClaimsFromCtx(r.Context()); claims != nil {
		orgID = claims.OrgID
	}
	if orgID == "" {
		orgID = "default"
	}
	orgDir := filepath.Join("data/files", orgID)
	os.MkdirAll(orgDir, 0755)
	localPath := filepath.Join(orgDir, fileID+ext)

	dst, err := os.Create(localPath)
	if err != nil {
		jsonErr(w, "cannot save file", 500)
		return
	}
	size, _ := io.Copy(dst, file)
	dst.Close()

	// Save file record
	s.SaveFile(fileID, 0, header.Filename, ct, size, localPath)

	// Get username
	claims := ClaimsFromCtx(r.Context())
	username := ""
	if claims != nil {
		username = claims.Username
	}

	caption := r.FormValue("caption")
	if caption == "" {
		caption = header.Filename
	}

	// Save channel message with file
	msg, _ := s.SaveChannelMessageFrom(channelID, "user", username, caption, "", fileID, fileType)

	if msg != nil {
		s.MarkChannelRead(channelID, userID, msg.ID)
	}

	// WS broadcast
	ch, _ := s.ChannelByID(channelID)
	if ch != nil {
		orgID := ""
		if claims != nil {
			orgID = claims.OrgID
		}
		subs, _ := s.ChannelSubscribers(ch.ID)
		payload, _ := json.Marshal(map[string]interface{}{
			"message":      msg,
			"channel_name": ch.Name,
			"sender_name":  username,
		})
		for _, uid := range subs {
			key := orgID + ":" + fmt.Sprintf("%d", uid)
			a.hub.SendToUser(key, ws.Event{Type: "channel_message", ChatID: ch.ID, Payload: payload})
		}
	}

	json.NewEncoder(w).Encode(msg)
}
