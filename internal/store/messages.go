// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package store

import "time"

// Message represents a chat message.
type Message struct {
	ID          int64  `json:"message_id"`
	ChatID      int64  `json:"chat_id"`
	Sender      string `json:"sender"`
	Text        string `json:"text,omitempty"`
	ReplyMarkup string `json:"reply_markup,omitempty"`
	FileID      string `json:"file_id,omitempty"`
	FileType    string `json:"file_type,omitempty"`
	CreatedAt   string `json:"date"`
}

func (s *Store) SaveMessage(chatID int64, sender, text, replyMarkup, fileID, fileType string) (*Message, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		"INSERT INTO messages (chat_id, sender, text, reply_markup, file_id, file_type, created_at) VALUES (?,?,?,?,?,?,?)",
		chatID, sender, text, replyMarkup, fileID, fileType, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &Message{ID: id, ChatID: chatID, Sender: sender, Text: text,
		ReplyMarkup: replyMarkup, FileID: fileID, FileType: fileType, CreatedAt: now}, nil
}

func (s *Store) GetMessage(id int64) (*Message, error) {
	m := &Message{}
	err := s.db.QueryRow(
		"SELECT id, chat_id, sender, COALESCE(text,''), COALESCE(reply_markup,''), COALESCE(file_id,''), COALESCE(file_type,''), created_at FROM messages WHERE id=?", id).
		Scan(&m.ID, &m.ChatID, &m.Sender, &m.Text, &m.ReplyMarkup, &m.FileID, &m.FileType, &m.CreatedAt)
	return m, err
}

func (s *Store) UpdateMessageText(id int64, text, replyMarkup string) error {
	_, err := s.db.Exec("UPDATE messages SET text=?, reply_markup=? WHERE id=?", text, replyMarkup, id)
	return err
}

func (s *Store) DeleteMessage(id int64) error {
	_, err := s.db.Exec("DELETE FROM messages WHERE id=?", id)
	return err
}

func (s *Store) ChatMessages(chatID int64, limit int) ([]Message, error) {
	rows, err := s.db.Query(
		"SELECT id, chat_id, sender, COALESCE(text,''), COALESCE(reply_markup,''), COALESCE(file_id,''), COALESCE(file_type,''), created_at FROM messages WHERE chat_id=? ORDER BY created_at DESC LIMIT ?",
		chatID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ChatID, &m.Sender, &m.Text, &m.ReplyMarkup, &m.FileID, &m.FileType, &m.CreatedAt); err != nil {
			continue
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}
