// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the MIT License. See LICENSE file for details.
package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS bots (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			token      TEXT UNIQUE NOT NULL,
			name       TEXT NOT NULL,
			webhook_url TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS users (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			username   TEXT UNIQUE NOT NULL,
			pin        TEXT NOT NULL,
			display_name TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS chats (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id    INTEGER NOT NULL REFERENCES users(id),
			bot_id     INTEGER NOT NULL REFERENCES bots(id),
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, bot_id)
		);

		CREATE TABLE IF NOT EXISTS messages (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			chat_id    INTEGER NOT NULL REFERENCES chats(id),
			sender     TEXT NOT NULL,
			text       TEXT,
			reply_markup TEXT,
			file_id    TEXT,
			file_type  TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS files (
			id         TEXT PRIMARY KEY,
			bot_id     INTEGER NOT NULL REFERENCES bots(id),
			filename   TEXT,
			mime_type  TEXT,
			size       INTEGER,
			path       TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS callback_queue (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			bot_id     INTEGER NOT NULL REFERENCES bots(id),
			chat_id    INTEGER NOT NULL,
			user_id    INTEGER NOT NULL,
			data       TEXT NOT NULL,
			message_id INTEGER,
			answered   BOOLEAN DEFAULT FALSE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_messages_chat ON messages(chat_id, created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_chats_user ON chats(user_id);
		CREATE INDEX IF NOT EXISTS idx_chats_bot ON chats(bot_id);
	`)
	return err
}

// ── Bots ──

type Bot struct {
	ID         int64  `json:"id"`
	Token      string `json:"token"`
	Name       string `json:"name"`
	WebhookURL string `json:"webhook_url,omitempty"`
}

func (s *Store) CreateBot(token, name string) (*Bot, error) {
	res, err := s.db.Exec("INSERT INTO bots (token, name) VALUES (?, ?)", token, name)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &Bot{ID: id, Token: token, Name: name}, nil
}

func (s *Store) BotByToken(token string) (*Bot, error) {
	b := &Bot{}
	err := s.db.QueryRow("SELECT id, token, name, COALESCE(webhook_url,'') FROM bots WHERE token=?", token).
		Scan(&b.ID, &b.Token, &b.Name, &b.WebhookURL)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (s *Store) SetWebhook(botID int64, url string) error {
	_, err := s.db.Exec("UPDATE bots SET webhook_url=? WHERE id=?", url, botID)
	return err
}

func (s *Store) ListBots() ([]Bot, error) {
	rows, err := s.db.Query("SELECT id, token, name, COALESCE(webhook_url,'') FROM bots")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var bots []Bot
	for rows.Next() {
		var b Bot
		rows.Scan(&b.ID, &b.Token, &b.Name, &b.WebhookURL)
		bots = append(bots, b)
	}
	return bots, nil
}

// ── Users ──

type User struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name,omitempty"`
}

func (s *Store) CreateUser(username, pin, displayName string) (*User, error) {
	res, err := s.db.Exec("INSERT INTO users (username, pin, display_name) VALUES (?, ?, ?)",
		username, pin, displayName)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &User{ID: id, Username: username, DisplayName: displayName}, nil
}

func (s *Store) AuthUser(username, pin string) (*User, error) {
	u := &User{}
	err := s.db.QueryRow("SELECT id, username, COALESCE(display_name,'') FROM users WHERE username=? AND pin=?",
		username, pin).Scan(&u.ID, &u.Username, &u.DisplayName)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// ── Chats ──

type Chat struct {
	ID     int64 `json:"id"`
	UserID int64 `json:"user_id"`
	BotID  int64 `json:"bot_id"`
}

func (s *Store) GetOrCreateChat(userID, botID int64) (*Chat, error) {
	c := &Chat{}
	err := s.db.QueryRow("SELECT id, user_id, bot_id FROM chats WHERE user_id=? AND bot_id=?",
		userID, botID).Scan(&c.ID, &c.UserID, &c.BotID)
	if err == nil {
		return c, nil
	}
	res, err := s.db.Exec("INSERT INTO chats (user_id, bot_id) VALUES (?, ?)", userID, botID)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &Chat{ID: id, UserID: userID, BotID: botID}, nil
}

func (s *Store) UserChats(userID int64) ([]Chat, error) {
	rows, err := s.db.Query("SELECT c.id, c.user_id, c.bot_id FROM chats c WHERE c.user_id=?", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var chats []Chat
	for rows.Next() {
		var c Chat
		rows.Scan(&c.ID, &c.UserID, &c.BotID)
		chats = append(chats, c)
	}
	return chats, nil
}

// ── Messages ──

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
		rows.Scan(&m.ID, &m.ChatID, &m.Sender, &m.Text, &m.ReplyMarkup, &m.FileID, &m.FileType, &m.CreatedAt)
		msgs = append(msgs, m)
	}
	return msgs, nil
}

// ── Files ──

type File struct {
	ID       string `json:"file_id"`
	BotID    int64  `json:"bot_id"`
	Filename string `json:"filename"`
	MimeType string `json:"mime_type"`
	Size     int64  `json:"file_size"`
	Path     string `json:"-"`
}

func (s *Store) SaveFile(id string, botID int64, filename, mimeType string, size int64, path string) error {
	_, err := s.db.Exec("INSERT INTO files (id, bot_id, filename, mime_type, size, path) VALUES (?,?,?,?,?,?)",
		id, botID, filename, mimeType, size, path)
	return err
}

func (s *Store) GetFile(id string) (*File, error) {
	f := &File{}
	err := s.db.QueryRow("SELECT id, bot_id, COALESCE(filename,''), COALESCE(mime_type,''), size, path FROM files WHERE id=?", id).
		Scan(&f.ID, &f.BotID, &f.Filename, &f.MimeType, &f.Size, &f.Path)
	return f, err
}
