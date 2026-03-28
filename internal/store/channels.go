// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package store

import (
	"log/slog"
	"time"
)

// Channel represents a broadcast channel.
type Channel struct {
	ID          int64  `json:"id"`
	BotID       int64  `json:"bot_id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ChannelMessage represents a message in a channel.
type ChannelMessage struct {
	ID          int64  `json:"message_id"`
	ChannelID   int64  `json:"channel_id"`
	Sender      string `json:"sender"`
	SenderName  string `json:"sender_name,omitempty"`
	Text        string `json:"text,omitempty"`
	ReplyMarkup string `json:"reply_markup,omitempty"`
	ReplyTo     int64  `json:"reply_to,omitempty"`
	FileID      string `json:"file_id,omitempty"`
	FileType    string `json:"file_type,omitempty"`
	CreatedAt   string `json:"date"`
	EditedAt    string `json:"edited_at,omitempty"`
}

func (s *Store) CreateChannel(botID int64, name, description string) (*Channel, error) {
	res, err := s.db.Exec("INSERT INTO channels (bot_id, name, description) VALUES (?, ?, ?)",
		botID, name, description)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &Channel{ID: id, BotID: botID, Name: name, Description: description}, nil
}

func (s *Store) ChannelByName(botID int64, name string) (*Channel, error) {
	ch := &Channel{}
	err := s.db.QueryRow("SELECT id, bot_id, name, COALESCE(description,'') FROM channels WHERE bot_id=? AND name=?",
		botID, name).Scan(&ch.ID, &ch.BotID, &ch.Name, &ch.Description)
	return ch, err
}

func (s *Store) ChannelByID(id int64) (*Channel, error) {
	ch := &Channel{}
	err := s.db.QueryRow("SELECT id, bot_id, name, COALESCE(description,'') FROM channels WHERE id=?",
		id).Scan(&ch.ID, &ch.BotID, &ch.Name, &ch.Description)
	return ch, err
}

func (s *Store) ListChannels() ([]Channel, error) {
	rows, err := s.db.Query("SELECT id, bot_id, name, COALESCE(description,'') FROM channels")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var chs []Channel
	for rows.Next() {
		var ch Channel
		if err := rows.Scan(&ch.ID, &ch.BotID, &ch.Name, &ch.Description); err != nil {
			continue
		}
		chs = append(chs, ch)
	}
	return chs, nil
}

func (s *Store) Subscribe(channelID, userID int64) error {
	_, err := s.db.Exec("INSERT OR IGNORE INTO channel_subscribers (channel_id, user_id) VALUES (?, ?)",
		channelID, userID)
	return err
}

func (s *Store) Unsubscribe(channelID, userID int64) error {
	_, err := s.db.Exec("DELETE FROM channel_subscribers WHERE channel_id=? AND user_id=?",
		channelID, userID)
	return err
}

func (s *Store) ChannelSubscribers(channelID int64) ([]int64, error) {
	rows, err := s.db.Query("SELECT user_id FROM channel_subscribers WHERE channel_id=?", channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (s *Store) UserSubscriptions(userID int64) ([]Channel, error) {
	rows, err := s.db.Query(`SELECT c.id, c.bot_id, c.name, COALESCE(c.description,'')
		FROM channels c JOIN channel_subscribers cs ON c.id = cs.channel_id
		WHERE cs.user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var chs []Channel
	for rows.Next() {
		var ch Channel
		if err := rows.Scan(&ch.ID, &ch.BotID, &ch.Name, &ch.Description); err != nil {
			continue
		}
		chs = append(chs, ch)
	}
	return chs, nil
}

func (s *Store) IsSubscribed(channelID, userID int64) bool {
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM channel_subscribers WHERE channel_id=? AND user_id=?",
		channelID, userID).Scan(&count)
	return count > 0
}

func (s *Store) SaveChannelMessage(channelID int64, text, replyMarkup, fileID, fileType string) (*ChannelMessage, error) {
	return s.SaveChannelMessageFrom(channelID, "bot", "", text, replyMarkup, fileID, fileType)
}

func (s *Store) SaveChannelMessageFrom(channelID int64, sender, senderName, text, replyMarkup, fileID, fileType string) (*ChannelMessage, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		"INSERT INTO channel_messages (channel_id, sender, sender_name, text, reply_markup, file_id, file_type, created_at) VALUES (?,?,?,?,?,?,?,?)",
		channelID, sender, senderName, text, replyMarkup, fileID, fileType, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &ChannelMessage{ID: id, ChannelID: channelID, Sender: sender, SenderName: senderName,
		Text: text, ReplyMarkup: replyMarkup, FileID: fileID, FileType: fileType, CreatedAt: now}, nil
}

func (s *Store) ChannelMessages(channelID int64, limit int) ([]ChannelMessage, error) {
	rows, err := s.db.Query(
		"SELECT id, channel_id, COALESCE(sender,'bot'), COALESCE(sender_name,''), COALESCE(text,''), COALESCE(reply_markup,''), COALESCE(reply_to,0), COALESCE(file_id,''), COALESCE(file_type,''), created_at, COALESCE(edited_at,'') FROM channel_messages WHERE channel_id=? ORDER BY created_at DESC LIMIT ?",
		channelID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var msgs []ChannelMessage
	for rows.Next() {
		var m ChannelMessage
		if err := rows.Scan(&m.ID, &m.ChannelID, &m.Sender, &m.SenderName, &m.Text, &m.ReplyMarkup, &m.ReplyTo, &m.FileID, &m.FileType, &m.CreatedAt, &m.EditedAt); err != nil {
			continue
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}

func (s *Store) GetChannelMessage(id int64) (*ChannelMessage, error) {
	m := &ChannelMessage{}
	err := s.db.QueryRow("SELECT id, channel_id, COALESCE(sender,'bot'), COALESCE(sender_name,''), COALESCE(text,''), COALESCE(edited_at,''), created_at FROM channel_messages WHERE id=?", id).
		Scan(&m.ID, &m.ChannelID, &m.Sender, &m.SenderName, &m.Text, &m.EditedAt, &m.CreatedAt)
	return m, err
}

func (s *Store) SetChannelMessageReplyTo(id, replyTo int64) {
	s.db.Exec("UPDATE channel_messages SET reply_to=? WHERE id=?", replyTo, id)
}

func (s *Store) UpdateChannelMessageText(id int64, text, replyMarkup string) error {
	_, err := s.db.Exec("UPDATE channel_messages SET text=?, reply_markup=?, edited_at=? WHERE id=?", text, replyMarkup, Now(), id)
	return err
}

func (s *Store) DeleteChannelMessage(id int64) error {
	// Clear pin if this message was pinned
	s.db.Exec("UPDATE channels SET pinned_message_id=0 WHERE pinned_message_id=?", id)
	_, err := s.db.Exec("DELETE FROM channel_messages WHERE id=?", id)
	return err
}

func (s *Store) DeleteChannel(id int64) error {
	s.db.Exec("DELETE FROM channel_messages WHERE channel_id=?", id)
	s.db.Exec("DELETE FROM channel_subscribers WHERE channel_id=?", id)
	s.db.Exec("DELETE FROM channel_reads WHERE channel_id=?", id)
	_, err := s.db.Exec("DELETE FROM channels WHERE id=?", id)
	return err
}

func (s *Store) MarkChannelRead(channelID, userID, lastMsgID int64) {
	s.db.Exec("INSERT INTO channel_reads (channel_id, user_id, last_read_id) VALUES (?,?,?) ON CONFLICT(channel_id,user_id) DO UPDATE SET last_read_id=?",
		channelID, userID, lastMsgID, lastMsgID)
}

func (s *Store) GetLastRead(channelID, userID int64) int64 {
	var id int64
	s.db.QueryRow("SELECT COALESCE(last_read_id,0) FROM channel_reads WHERE channel_id=? AND user_id=?", channelID, userID).Scan(&id)
	return id
}

func (s *Store) UnreadCount(channelID, userID int64) int {
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM channel_messages WHERE channel_id=? AND id > COALESCE((SELECT last_read_id FROM channel_reads WHERE channel_id=? AND user_id=?), 0)",
		channelID, channelID, userID).Scan(&count)
	return count
}

func (s *Store) PinMessage(channelID, messageID int64) error {
	_, err := s.db.Exec("UPDATE channels SET pinned_message_id=? WHERE id=?", messageID, channelID)
	return err
}

func (s *Store) GetPinnedMessage(channelID int64) int64 {
	var id int64
	s.db.QueryRow("SELECT COALESCE(pinned_message_id,0) FROM channels WHERE id=?", channelID).Scan(&id)
	return id
}

func (s *Store) CleanOldChannelMessages(cutoff string) {
	res, _ := s.db.Exec("DELETE FROM channel_messages WHERE created_at < ?", cutoff)
	if res != nil {
		n, _ := res.RowsAffected()
		if n > 0 {
			slog.Info("cleaned old channel messages", "org", s.OrgID, "deleted", n)
		}
	}
}

// ChannelInfo is a pre-joined view used by ListChannelsForUser to avoid N+1 queries.
type ChannelInfo struct {
	ID              int64  `json:"id"`
	BotID           int64  `json:"bot_id"`
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	Subscribed      bool   `json:"subscribed"`
	Unread          int    `json:"unread"`
	PinnedMessageID int64  `json:"pinned_message_id"`
}

// ListChannelsForUser returns all channels with subscription/unread status in a single query.
func (s *Store) ListChannelsForUser(userID int64) ([]ChannelInfo, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.bot_id, c.name, COALESCE(c.description,''),
		       COALESCE(c.pinned_message_id, 0),
		       CASE WHEN cs.user_id IS NOT NULL THEN 1 ELSE 0 END AS subscribed,
		       CASE WHEN cs.user_id IS NOT NULL THEN
		         (SELECT COUNT(*) FROM channel_messages cm
		          WHERE cm.channel_id = c.id
		          AND cm.id > COALESCE((SELECT last_read_id FROM channel_reads cr
		              WHERE cr.channel_id = c.id AND cr.user_id = ?), 0))
		       ELSE 0 END AS unread
		FROM channels c
		LEFT JOIN channel_subscribers cs ON c.id = cs.channel_id AND cs.user_id = ?
		ORDER BY c.name`, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []ChannelInfo
	for rows.Next() {
		var ci ChannelInfo
		var sub int
		if err := rows.Scan(&ci.ID, &ci.BotID, &ci.Name, &ci.Description, &ci.PinnedMessageID, &sub, &ci.Unread); err != nil {
			continue
		}
		ci.Subscribed = sub == 1
		result = append(result, ci)
	}
	return result, nil
}

// ChannelReader represents a user who has read messages in a channel.
type ChannelReader struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	LastRead int64  `json:"last_read_id"`
}

// ChannelReadersJoin returns readers with usernames in a single JOIN query (replaces N+1).
func (s *Store) ChannelReadersJoin(channelID int64) ([]ChannelReader, error) {
	rows, err := s.db.Query(`
		SELECT cr.user_id, u.username, cr.last_read_id
		FROM channel_reads cr
		JOIN users u ON cr.user_id = u.id
		JOIN channel_subscribers cs ON cs.channel_id = cr.channel_id AND cs.user_id = cr.user_id
		WHERE cr.channel_id = ? AND cr.last_read_id > 0`, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var readers []ChannelReader
	for rows.Next() {
		var r ChannelReader
		if err := rows.Scan(&r.UserID, &r.Username, &r.LastRead); err != nil {
			continue
		}
		readers = append(readers, r)
	}
	return readers, nil
}

// RenameChannel updates the name of a channel.
func (s *Store) RenameChannel(channelID int64, name string) error {
	_, err := s.db.Exec("UPDATE channels SET name=? WHERE id=?", name, channelID)
	return err
}
