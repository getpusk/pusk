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

// broadcastChannel sends a WS event to all subscribers of a channel.
func (a *ClientAPI) broadcastChannel(s *store.Store, channelID int64, orgID, evType string, payload []byte, excludeUserID ...int64) {
	subs, _ := s.ChannelSubscribers(channelID)
	for _, uid := range subs {
		if len(excludeUserID) > 0 && uid == excludeUserID[0] {
			continue
		}
		key := orgID + ":" + fmt.Sprintf("%d", uid)
		a.hub.SendToUser(key, ws.Event{Type: evType, ChatID: channelID, Payload: payload})
	}
}

func (a *ClientAPI) channelReaders(w http.ResponseWriter, r *http.Request) {
	channelID, _ := strconv.ParseInt(r.PathValue("channelID"), 10, 64)
	s := a.db(r)
	readers, err := s.ChannelReadersJoin(channelID)
	if err != nil {
		jsonErr(w, "internal error", 500)
		return
	}
	if readers == nil {
		readers = []store.ChannelReader{}
	}
	_ = json.NewEncoder(w).Encode(readers)
}

func (a *ClientAPI) ackChannelMessage(w http.ResponseWriter, r *http.Request) {
	channelID, _ := strconv.ParseInt(r.PathValue("channelID"), 10, 64)
	s := a.db(r)

	var req struct {
		MessageID int64  `json:"message_id"`
		Action    string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, "invalid request body", 400)
		return
	}

	claims := ClaimsFromCtx(r.Context())
	username := ""
	if claims != nil {
		username = claims.Username
	}

	msg, err := s.GetChannelMessage(req.MessageID)
	if err != nil || msg.ChannelID != channelID {
		jsonErr(w, "message not found", 404)
		return
	}
	// EDGE-9: reject unknown ACK actions
	switch req.Action {
	case "ack", "mute", "resolved":
		// valid
	default:
		jsonErr(w, "invalid action", 400)
		return
	}
	// BUG-11: prevent re-ACK on already ACK'd messages
	// Check last 80 chars only — ACK status is always appended at the end
	tail := msg.Text
	if len(tail) > 80 {
		tail = tail[len(tail)-80:]
	}
	if strings.Contains(tail, "**ACK**") || strings.Contains(tail, "**Resolved**") || strings.Contains(tail, "**Muted") {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "already": true})
		return
	}
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
	newText := msg.Text + status
	_ = s.UpdateChannelMessageText(msg.ID, newText, "")

	// Alertmanager silence on ACK/mute
	amURL := os.Getenv("PUSK_ALERTMANAGER_URL")
	if amURL != "" && (req.Action == "ack" || req.Action == "mute") {
		go createAlertmanagerSilence(amURL, username, msg.Text)
	}

	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (a *ClientAPI) listChannels(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromCtx(r.Context())
	s := a.db(r)
	result, err := s.ListChannelsForUser(userID)
	if err != nil {
		jsonErr(w, "internal error", 500)
		return
	}
	if result == nil {
		result = []store.ChannelInfo{}
	}
	_ = json.NewEncoder(w).Encode(result)
}

func (a *ClientAPI) subscribe(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromCtx(r.Context())
	channelID, _ := strconv.ParseInt(r.PathValue("channelID"), 10, 64)
	if err := a.db(r).Subscribe(channelID, userID); err != nil {
		jsonErr(w, "internal error", 500)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (a *ClientAPI) unsubscribe(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromCtx(r.Context())
	channelID, _ := strconv.ParseInt(r.PathValue("channelID"), 10, 64)
	if err := a.db(r).Unsubscribe(channelID, userID); err != nil {
		jsonErr(w, "internal error", 500)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (a *ClientAPI) channelMessages(w http.ResponseWriter, r *http.Request) {
	channelID, _ := strconv.ParseInt(r.PathValue("channelID"), 10, 64)
	s := a.db(r)
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		limit, _ = strconv.Atoi(l)
	}
	if limit > 200 {
		limit = 200
	}
	msgs, err := s.ChannelMessages(channelID, limit)
	if err != nil {
		jsonErr(w, "internal error", 500)
		return
	}
	if msgs == nil {
		msgs = []store.ChannelMessage{}
	}
	_ = json.NewEncoder(w).Encode(msgs)
}

func (a *ClientAPI) markChannelRead(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromCtx(r.Context())
	channelID, _ := strconv.ParseInt(r.PathValue("channelID"), 10, 64)
	s := a.db(r)

	var req struct {
		LastReadID int64 `json:"last_read_id"`
	}
	// Body is optional — if empty or missing last_read_id, mark up to latest message
	_ = json.NewDecoder(r.Body).Decode(&req)

	if req.LastReadID == 0 {
		// Find the latest message in the channel
		msgs, err := s.ChannelMessages(channelID, 1)
		if err != nil || len(msgs) == 0 {
			_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
			return
		}
		req.LastReadID = msgs[0].ID
	}

	s.MarkChannelRead(channelID, userID, req.LastReadID)
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, "invalid request body", 400)
		return
	}
	if req.Text == "" {
		jsonErr(w, "text required", 400)
		return
	}
	// EDGE-10: message text length limit
	if len(req.Text) > 4096 {
		jsonErr(w, "text too long (max 4096 chars)", 400)
		return
	}

	claims := ClaimsFromCtx(r.Context())
	username := ""
	if claims != nil {
		username = claims.Username
	}

	// Use display_name as sender_name if available
	senderName := username
	if user, uerr := s.GetUserByID(userID); uerr == nil && user != nil && user.DisplayName != "" {
		senderName = user.DisplayName
	}

	msg, err := s.SaveChannelMessageFrom(channelID, "user", senderName, req.Text, "", "", "")
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
		payload, _ := json.Marshal(map[string]interface{}{
			"message":      msg,
			"channel_name": ch.Name,
			"sender_name":  senderName,
		})
		a.broadcastChannel(s, ch.ID, orgID, "channel_message", payload)

		// Push notification to offline channel subscribers (skip sender + online users)
		sentPush := map[int64]bool{}
		subs, _ := s.ChannelSubscribers(ch.ID)
		for _, uid := range subs {
			if uid == userID {
				continue
			}
			wsKey := orgID + ":" + fmt.Sprintf("%d", uid)
			if a.hub.IsConnected(wsKey) {
				ach := a.hub.GetActiveChannel(wsKey)
				if ach == ch.ID {
					continue // viewing this channel, gets WS event
				}
			}
			a.push.SendToUser(s, uid, notify.PushPayload{
				Title: "#" + ch.Name,
				Body:  senderName + ": " + req.Text[:min(len(req.Text), 100)],
				Tag:   fmt.Sprintf("ch-%d-%d", ch.ID, msg.ID),
				URL:   fmt.Sprintf("/?channel=%d&org=%s", ch.ID, orgID),
			})
			sentPush[uid] = true
		}

		// Reply push: notify the original message author (skip if already got channel push)
		if req.ReplyTo > 0 {
			origMsg, rerr := s.GetChannelMessage(req.ReplyTo)
			if rerr == nil && origMsg.Sender == "user" && origMsg.SenderName != "" {
				users, _ := s.ListUsers()
				for _, u := range users {
					if u.Username == origMsg.SenderName && u.ID != userID && !sentPush[u.ID] {
						a.push.SendToUser(s, u.ID, notify.PushPayload{
							Title: "#" + ch.Name + " \u2014 reply",
							Body:  senderName + ": " + req.Text[:min(len(req.Text), 100)],
							Tag:   fmt.Sprintf("reply-%d-%d", ch.ID, msg.ID),
							URL:   fmt.Sprintf("/?channel=%d&org=%s", ch.ID, orgID),
						})
						sentPush[u.ID] = true
						break
					}
				}
			}
		}

		// @mentions: push notification to mentioned users (skip if already got channel push)
		if strings.Contains(req.Text, "@") {
			users, _ := s.ListUsers()
			for _, u := range users {
				if strings.Contains(req.Text, "@"+u.Username) && u.ID != userID && !sentPush[u.ID] {
					mentionPayload, _ := json.Marshal(map[string]interface{}{
						"type":    "mention",
						"channel": ch.Name,
						"from":    senderName,
						"text":    req.Text,
					})
					key := orgID + ":" + fmt.Sprintf("%d", u.ID)
					a.hub.SendToUser(key, ws.Event{Type: "mention", ChatID: ch.ID, Payload: mentionPayload})
					a.push.SendToUser(s, u.ID, notify.PushPayload{
						Title: "#" + ch.Name + " — @" + u.Username,
						Body:  senderName + ": " + truncateText(req.Text, 80),
						Tag:   "mention-" + ch.Name,
						URL:   fmt.Sprintf("/?channel=%d&org=%s", ch.ID, orgID),
					})
				}
			}
		}
	}

	_ = json.NewEncoder(w).Encode(msg)
}

func (a *ClientAPI) pushUnsubscribe(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromCtx(r.Context())
	var req struct {
		Endpoint string `json:"endpoint"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, "invalid request body", 400)
		return
	}
	if req.Endpoint == "" {
		// Delete all push subscriptions for this user
		_ = a.db(r).DeleteAllPushSubscriptions(userID)
	} else {
		_ = a.db(r).DeletePushSubscription(req.Endpoint)
	}
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (a *ClientAPI) pushSubscribe(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromCtx(r.Context())
	if claims == nil || claims.OrgID == "" || claims.OrgID == "default" {
		jsonErr(w, "push not available in default org", 403)
		return
	}
	userID := UserIDFromCtx(r.Context())
	var req struct {
		Endpoint string `json:"endpoint"`
		Keys     struct {
			P256dh string `json:"p256dh"`
			Auth   string `json:"auth"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, "invalid request body", 400)
		return
	}
	if err := a.db(r).SavePushSubscription(userID, req.Endpoint, req.Keys.P256dh, req.Keys.Auth); err != nil {
		jsonErr(w, "internal error", 500)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (a *ClientAPI) vapidKey(w http.ResponseWriter, r *http.Request) {
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"key": a.vapidPub, "configured": a.vapidPub != ""})
}

func (a *ClientAPI) testPush(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromCtx(r.Context())
	claims := ClaimsFromCtx(r.Context())
	username := ""
	if claims != nil {
		username = claims.Username
	}
	if claims == nil || claims.OrgID == "" || claims.OrgID == "default" {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "push not available in default org"})
		return
	}
	s := a.db(r)
	subs, _ := s.UserPushSubscriptions(userID)
	if len(subs) == 0 {
		slog.Info("push test: no subscriptions", "user_id", userID, "username", username)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "no push subscription", "subscriptions": 0})
		return
	}
	slog.Info("push test", "user_id", userID, "username", username, "subscriptions", len(subs))
	a.push.SendToUser(s, userID, notify.PushPayload{
		Title: "Pusk Test",
		Body:  "Push works! / Push работает!",
		Tag:   "test-push",
		URL:   "/",
	})
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "subscriptions": len(subs)})
}

func (a *ClientAPI) listUsers(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromCtx(r.Context())
	if claims != nil && (claims.OrgID == "" || claims.OrgID == "default") {
		_ = json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	users, err := a.db(r).ListUsers()
	if err != nil {
		jsonErr(w, "internal error", 500)
		return
	}
	_ = json.NewEncoder(w).Encode(users)
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, "invalid request body", 400)
		return
	}
	if req.Role != "admin" && req.Role != "member" {
		jsonErr(w, "role must be admin or member", 400)
		return
	}
	// BUG-2: prevent self-demotion
	if targetID == userID && req.Role == "member" {
		jsonErr(w, "cannot demote yourself", 400)
		return
	}
	// Protect primary admin (org creator) from demotion
	if targetID == 1 && req.Role == "member" {
		jsonErr(w, "cannot demote primary admin", 400)
		return
	}
	// BUG-2: prevent demoting the last admin
	if req.Role == "member" && s.IsAdmin(targetID) {
		users, _ := s.ListUsers()
		adminCount := 0
		for _, u := range users {
			if s.IsAdmin(u.ID) {
				adminCount++
			}
		}
		if adminCount <= 1 {
			jsonErr(w, "cannot demote the last admin", 400)
			return
		}
	}
	_ = s.SetUserRole(targetID, req.Role)
	// K14: notify target user about role change via WS
	claims := ClaimsFromCtx(r.Context())
	if claims != nil {
		key := claims.OrgID + ":" + strconv.FormatInt(targetID, 10)
		payload, _ := json.Marshal(map[string]string{"role": req.Role})
		a.hub.SendToUser(key, ws.Event{Type: "role_update", Payload: payload})
	}
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
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
	// BUG-12: return 404 for nonexistent users
	if !s.UserExists(targetID) {
		jsonErr(w, "user not found", 404)
		return
	}
	_ = s.DeleteUser(targetID)
	claims := ClaimsFromCtx(r.Context())
	if claims != nil {
		RevokeUser(claims.OrgID, targetID)
	}
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, "invalid request body", 400)
		return
	}
	_ = s.UpdateChannelMessageText(msgID, req.Text, "")

	updated, _ := s.GetChannelMessage(msgID)
	if updated != nil {
		orgID := ""
		if claims != nil {
			orgID = claims.OrgID
		}
		payload, _ := json.Marshal(updated)
		a.broadcastChannel(s, channelID, orgID, "channel_message_edit", payload)
	}

	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
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
	payload, _ := json.Marshal(map[string]int64{"message_id": msgID})
	a.broadcastChannel(s, msg.ChannelID, orgID, "channel_message_delete", payload)

	_ = s.DeleteChannelMessage(msgID)
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (a *ClientAPI) onlineUsers(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromCtx(r.Context())
	prefix := claims.OrgID + ":"
	s := a.db(r)
	type onlineUser struct {
		UserID   int64  `json:"user_id"`
		Username string `json:"username"`
		Status   string `json:"status"`
	}
	var users []onlineUser
	onlineCount := 0
	for _, key := range a.hub.OnlineKeys() {
		if strings.HasPrefix(key, prefix) {
			parts := strings.SplitN(key, ":", 2)
			if len(parts) == 2 {
				if uid, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
					st := a.hub.GetStatus(key)
					uname := ""
					if u, err := s.GetUserByID(uid); err == nil {
						uname = u.Username
					}
					users = append(users, onlineUser{UserID: uid, Username: uname, Status: st})
					if st == "online" {
						onlineCount++
					}
				}
			}
		}
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"online": onlineCount, "total_connected": len(users), "users": users})
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, "invalid request body", 400)
		return
	}
	_ = s.PinMessage(channelID, req.MessageID)
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
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
		"matchers": []map[string]interface{}{
			{"name": "alertname", "value": alertname, "isRegex": false},
		},
		"startsAt":  now.Format(time.RFC3339),
		"endsAt":    now.Add(1 * time.Hour).Format(time.RFC3339),
		"createdBy": username,
		"comment":   "ACK'd via Pusk",
	}

	data, _ := json.Marshal(silence)
	client := &http.Client{Timeout: 10 * time.Second}
	//nolint:gosec // G704: Alertmanager URL from server config
	resp, err := client.Post(amURL+"/api/v2/silences", "application/json", bytes.NewReader(data)) // #nosec G704
	if err != nil {
		slog.Warn("alertmanager silence failed", "error", err)
		return
	}
	_ = resp.Body.Close()
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

	//nolint:gosec // G120: bounded to 10MB
	_ = r.ParseMultipartForm(10 << 20) // #nosec G120 -- 10MB max

	file, header, err := r.FormFile("file")
	if err != nil {
		jsonErr(w, "file required", 400)
		return
	}
	defer func() { _ = file.Close() }()

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
	_ = os.MkdirAll(orgDir, 0o750)
	localPath := filepath.Join(orgDir, fileID+ext)

	//nolint:gosec // G703,G304: path from filepath.Join with server-generated UUID
	dst, err := os.Create(localPath) // #nosec G703 G304
	if err != nil {
		jsonErr(w, "cannot save file", 500)
		return
	}
	size, _ := io.Copy(dst, file)
	_ = dst.Close()

	// Check storage quota (default 1GB, PUSK_FILE_QUOTA_MB env)
	quotaMB := int64(1024)
	if v := os.Getenv("PUSK_FILE_QUOTA_MB"); v != "" {
		if q, err := strconv.ParseInt(v, 10, 64); err == nil && q > 0 {
			quotaMB = q
		}
	}
	if s.TotalFileSize()+size > quotaMB*1024*1024 {
		//nolint:gosec // G703: path from filepath.Join with server-generated UUID
		_ = os.Remove(localPath) // #nosec G703
		jsonErr(w, "storage quota exceeded", 400)
		return
	}

	// Save file record
	_ = s.SaveFile(fileID, 0, header.Filename, ct, size, localPath)

	// Get username and display_name
	claims := ClaimsFromCtx(r.Context())
	username := ""
	if claims != nil {
		username = claims.Username
	}
	uploadSenderName := username
	if upUser, uerr := s.GetUserByID(userID); uerr == nil && upUser != nil && upUser.DisplayName != "" {
		uploadSenderName = upUser.DisplayName
	}

	caption := r.FormValue("caption")
	if caption == "" {
		caption = header.Filename
	}

	// Save channel message with file
	msg, _ := s.SaveChannelMessageFrom(channelID, "user", uploadSenderName, caption, "", fileID, fileType)

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
		payload, _ := json.Marshal(map[string]interface{}{
			"message":      msg,
			"channel_name": ch.Name,
			"sender_name":  uploadSenderName,
		})
		a.broadcastChannel(s, ch.ID, orgID, "channel_message", payload)
	}

	_ = json.NewEncoder(w).Encode(msg)
}

func (a *ClientAPI) createFileToken(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromCtx(r.Context())
	b := make([]byte, 16)
	crand.Read(b)
	token := hex.EncodeToString(b)
	s := a.db(r)
	if err := s.CreateFileToken(token, userID, 5*time.Minute); err != nil {
		jsonErr(w, "internal error", 500)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
}
