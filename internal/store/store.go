// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package store

import (
	"database/sql"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

// Now returns current time in RFC3339 format
func Now() string { return time.Now().UTC().Format(time.RFC3339) }

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
			icon_url   TEXT,
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

		CREATE TABLE IF NOT EXISTS channels (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			bot_id      INTEGER NOT NULL REFERENCES bots(id),
			name        TEXT NOT NULL,
			description TEXT,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(bot_id, name)
		);

		CREATE TABLE IF NOT EXISTS channel_subscribers (
			channel_id  INTEGER REFERENCES channels(id),
			user_id     INTEGER REFERENCES users(id),
			subscribed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (channel_id, user_id)
		);

		CREATE TABLE IF NOT EXISTS channel_messages (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			channel_id   INTEGER NOT NULL REFERENCES channels(id),
			text         TEXT,
			reply_markup TEXT,
			file_id      TEXT,
			file_type    TEXT,
			created_at   DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_channel_msgs ON channel_messages(channel_id, created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_channel_subs ON channel_subscribers(user_id);

		CREATE TABLE IF NOT EXISTS push_subscriptions (
			id       INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id  INTEGER NOT NULL REFERENCES users(id),
			endpoint TEXT NOT NULL UNIQUE,
			p256dh   TEXT NOT NULL,
			auth     TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS invites (
			code       TEXT PRIMARY KEY,
			used       BOOLEAN DEFAULT FALSE,
			expires_at DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		return err
	}
	// Migrations
	s.db.Exec("ALTER TABLE channel_messages ADD COLUMN sender TEXT DEFAULT 'bot'")
	s.db.Exec("CREATE TABLE IF NOT EXISTS channel_reads (channel_id INTEGER, user_id INTEGER, last_read_id INTEGER DEFAULT 0, PRIMARY KEY(channel_id, user_id))")
	s.db.Exec("ALTER TABLE channel_messages ADD COLUMN sender_name TEXT DEFAULT ''")
	s.db.Exec("ALTER TABLE channel_messages ADD COLUMN reply_to INTEGER DEFAULT 0")
	s.db.Exec("ALTER TABLE users ADD COLUMN role TEXT DEFAULT 'member'")
	s.db.Exec("ALTER TABLE channel_messages ADD COLUMN edited_at TEXT DEFAULT ''")
	return nil
}

// ── Bots ──

type Bot struct {
	ID         int64  `json:"id"`
	Token      string `json:"token"`
	Name       string `json:"name"`
	WebhookURL string `json:"webhook_url,omitempty"`
	IconURL    string `json:"icon_url,omitempty"`
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
	err := s.db.QueryRow("SELECT id, token, name, COALESCE(webhook_url,''), COALESCE(icon_url,'') FROM bots WHERE token=?", token).
		Scan(&b.ID, &b.Token, &b.Name, &b.WebhookURL, &b.IconURL)
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
	rows, err := s.db.Query("SELECT id, token, name, COALESCE(webhook_url,''), COALESCE(icon_url,'') FROM bots")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var bots []Bot
	for rows.Next() {
		var b Bot
		rows.Scan(&b.ID, &b.Token, &b.Name, &b.WebhookURL, &b.IconURL)
		bots = append(bots, b)
	}
	return bots, nil
}

// ── Users ──

type User struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name,omitempty"`
	Role        string `json:"role"`
}

func (s *Store) CreateUser(username, pin, displayName string) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(pin), bcrypt.DefaultCost) // cost 10, ~0.3s
	if err != nil {
		return nil, fmt.Errorf("hash pin: %w", err)
	}
	res, err := s.db.Exec("INSERT INTO users (username, pin, display_name) VALUES (?, ?, ?)",
		username, string(hash), displayName)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &User{ID: id, Username: username, DisplayName: displayName}, nil
}

func (s *Store) AuthUser(username, pin string) (*User, error) {
	u := &User{}
	var storedHash string
	err := s.db.QueryRow("SELECT id, username, COALESCE(display_name,''), pin FROM users WHERE username=?",
		username).Scan(&u.ID, &u.Username, &u.DisplayName, &storedHash)
	if err != nil {
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(pin)); err != nil {
		return nil, fmt.Errorf("invalid pin")
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

func (s *Store) SetChannelMessageReplyTo(id, replyTo int64) {
	s.db.Exec("UPDATE channel_messages SET reply_to=? WHERE id=?", replyTo, id)
}

func (s *Store) UpdateChannelMessageText(id int64, text, replyMarkup string) error {
	_, err := s.db.Exec("UPDATE channel_messages SET text=?, reply_markup=?, edited_at=? WHERE id=?", text, replyMarkup, Now(), id)
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

// ── Lookups ──

func (s *Store) ChatUserID(chatID int64) (int64, error) {
	var userID int64
	err := s.db.QueryRow("SELECT user_id FROM chats WHERE id=?", chatID).Scan(&userID)
	return userID, err
}

func (s *Store) ChatBotID(chatID int64) (int64, error) {
	var botID int64
	err := s.db.QueryRow("SELECT bot_id FROM chats WHERE id=?", chatID).Scan(&botID)
	return botID, err
}

func (s *Store) BotByID(id int64) (*Bot, error) {
	b := &Bot{}
	err := s.db.QueryRow("SELECT id, token, name, COALESCE(webhook_url,''), COALESCE(icon_url,'') FROM bots WHERE id=?", id).
		Scan(&b.ID, &b.Token, &b.Name, &b.WebhookURL, &b.IconURL)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// ── Channels ──

type Channel struct {
	ID          int64  `json:"id"`
	BotID       int64  `json:"bot_id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

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
		rows.Scan(&ch.ID, &ch.BotID, &ch.Name, &ch.Description)
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
		rows.Scan(&id)
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
		rows.Scan(&ch.ID, &ch.BotID, &ch.Name, &ch.Description)
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
		rows.Scan(&m.ID, &m.ChannelID, &m.Sender, &m.SenderName, &m.Text, &m.ReplyMarkup, &m.ReplyTo, &m.FileID, &m.FileType, &m.CreatedAt, &m.EditedAt)
		msgs = append(msgs, m)
	}
	return msgs, nil
}

// ── Push Subscriptions ──

type PushSubscription struct {
	ID       int64  `json:"id"`
	UserID   int64  `json:"user_id"`
	Endpoint string `json:"endpoint"`
	P256dh   string `json:"p256dh"`
	Auth     string `json:"auth"`
}

func (s *Store) SavePushSubscription(userID int64, endpoint, p256dh, auth string) error {
	_, err := s.db.Exec(`INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth) VALUES (?, ?, ?, ?)
		ON CONFLICT(endpoint) DO UPDATE SET user_id=?, p256dh=?, auth=?`,
		userID, endpoint, p256dh, auth, userID, p256dh, auth)
	return err
}

func (s *Store) UserPushSubscriptions(userID int64) ([]PushSubscription, error) {
	rows, err := s.db.Query("SELECT id, user_id, endpoint, p256dh, auth FROM push_subscriptions WHERE user_id=?", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var subs []PushSubscription
	for rows.Next() {
		var s PushSubscription
		rows.Scan(&s.ID, &s.UserID, &s.Endpoint, &s.P256dh, &s.Auth)
		subs = append(subs, s)
	}
	return subs, nil
}

func (s *Store) DeletePushSubscription(endpoint string) error {
	_, err := s.db.Exec("DELETE FROM push_subscriptions WHERE endpoint=?", endpoint)
	return err
}

// ── Invites ──

func (s *Store) CreateInvite(code string, ttl time.Duration) error {
	expires := time.Now().Add(ttl).UTC().Format(time.RFC3339)
	_, err := s.db.Exec("INSERT INTO invites (code, expires_at) VALUES (?, ?)", code, expires)
	return err
}

// ── Channel Reads ──

func (s *Store) MarkChannelRead(channelID, userID, lastMsgID int64) {
	s.db.Exec("INSERT INTO channel_reads (channel_id, user_id, last_read_id) VALUES (?,?,?) ON CONFLICT(channel_id,user_id) DO UPDATE SET last_read_id=?",
		channelID, userID, lastMsgID, lastMsgID)
}

func (s *Store) UnreadCount(channelID, userID int64) int {
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM channel_messages WHERE channel_id=? AND id > COALESCE((SELECT last_read_id FROM channel_reads WHERE channel_id=? AND user_id=?), 0)",
		channelID, channelID, userID).Scan(&count)
	return count
}

// ── Users & Roles ──

func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query("SELECT id, username, COALESCE(display_name,''), COALESCE(role,'member') FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role)
		users = append(users, u)
	}
	return users, nil
}

func (s *Store) SetUserRole(userID int64, role string) error {
	_, err := s.db.Exec("UPDATE users SET role=? WHERE id=?", role, userID)
	return err
}

func (s *Store) IsAdmin(userID int64) bool {
	var role string
	s.db.QueryRow("SELECT COALESCE(role,'member') FROM users WHERE id=?", userID).Scan(&role)
	return role == "admin" || userID == 1
}

func (s *Store) DeleteChannelMessage(id int64) error {
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

func (s *Store) GetChannelMessage(id int64) (*ChannelMessage, error) {
	m := &ChannelMessage{}
	err := s.db.QueryRow("SELECT id, channel_id, COALESCE(sender,'bot'), COALESCE(sender_name,''), COALESCE(text,''), COALESCE(edited_at,''), created_at FROM channel_messages WHERE id=?", id).
		Scan(&m.ID, &m.ChannelID, &m.Sender, &m.SenderName, &m.Text, &m.EditedAt, &m.CreatedAt)
	return m, err
}

func (s *Store) UseInvite(code string) error {
	var used bool
	var expiresAt string
	err := s.db.QueryRow("SELECT used, expires_at FROM invites WHERE code=?", code).Scan(&used, &expiresAt)
	if err != nil {
		return fmt.Errorf("invite not found")
	}
	if used {
		return fmt.Errorf("invite already used")
	}
	expires, _ := time.Parse(time.RFC3339, expiresAt)
	if time.Now().After(expires) {
		return fmt.Errorf("invite expired")
	}
	_, err = s.db.Exec("UPDATE invites SET used=TRUE WHERE code=?", code)
	return err
}
