// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Now returns current time in RFC3339 format.
func Now() string { return time.Now().UTC().Format(time.RFC3339) }

// Store wraps a per-org SQLite database.
type Store struct {
	OrgID string
	db    *sql.DB
}

// New opens (or creates) the SQLite database at path and runs migrations.
func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(4) // WAL mode allows concurrent reads; single writer enforced by WAL
	//nolint:errcheck // best-effort PRAGMA tuning — defaults are safe
	db.Exec("PRAGMA journal_size_limit = 67108864") // 64MB
	//nolint:errcheck // best-effort PRAGMA tuning
	db.Exec("PRAGMA synchronous = NORMAL")
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// Ping checks that the database is accessible.
func (s *Store) Ping() error {
	var n int
	return s.db.QueryRow("SELECT 1").Scan(&n)
}

func (s *Store) migrate() error {
	var version int
	//nolint:errcheck // defaults to 0 on fresh DB — triggers full migration
	s.db.QueryRow("PRAGMA user_version").Scan(&version)

	if version < 1 {
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
		if _, err := s.db.Exec("PRAGMA user_version = 1"); err != nil {
			return fmt.Errorf("set version 1: %w", err)
		}
	}

	if version < 2 {
		stmts := []string{
			"ALTER TABLE channel_messages ADD COLUMN sender TEXT DEFAULT 'bot'",
			"CREATE TABLE IF NOT EXISTS channel_reads (channel_id INTEGER, user_id INTEGER, last_read_id INTEGER DEFAULT 0, PRIMARY KEY(channel_id, user_id))",
			"ALTER TABLE channel_messages ADD COLUMN sender_name TEXT DEFAULT ''",
			"ALTER TABLE channel_messages ADD COLUMN reply_to INTEGER DEFAULT 0",
			"ALTER TABLE users ADD COLUMN role TEXT DEFAULT 'member'",
			"ALTER TABLE channel_messages ADD COLUMN edited_at TEXT DEFAULT ''",
			"ALTER TABLE channels ADD COLUMN pinned_message_id INTEGER DEFAULT 0",
		}
		for _, stmt := range stmts {
			if _, err := s.db.Exec(stmt); err != nil {
				return fmt.Errorf("migrate v2: %w", err)
			}
		}
		if _, err := s.db.Exec("PRAGMA user_version = 2"); err != nil {
			return fmt.Errorf("set version 2: %w", err)
		}
	}

	// v3: multi-use invites
	if version < 3 {
		//nolint:errcheck // ALTER TABLE ADD COLUMN is idempotent in SQLite — safe to ignore "duplicate column" error
		s.db.Exec("ALTER TABLE invites ADD COLUMN uses INTEGER DEFAULT 0")
		//nolint:errcheck // same as above
		s.db.Exec("ALTER TABLE invites ADD COLUMN max_uses INTEGER DEFAULT 50")
	}

	// v4: file tokens
	if version < 4 {
		if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS file_tokens (
			token TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			expires_at DATETIME NOT NULL
		)`); err != nil {
			return fmt.Errorf("migrate v3 create file_tokens: %w", err)
		}
		if _, err := s.db.Exec("PRAGMA user_version = 4"); err != nil {
			return fmt.Errorf("set version 3: %w", err)
		}
	}

	return nil
}

// DB returns the underlying sql.DB for direct queries.
func (s *Store) DB() *sql.DB { return s.db }
