// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package store

import (
	"fmt"
	"time"
)

// File represents an uploaded file.
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

func (s *Store) CreateFileToken(token string, userID int64, ttl time.Duration) error {
	expires := time.Now().Add(ttl).UTC().Format(time.RFC3339)
	_, err := s.db.Exec("INSERT INTO file_tokens (token, user_id, expires_at) VALUES (?, ?, ?)", token, userID, expires)
	return err
}

func (s *Store) ValidateFileToken(token string) (int64, error) {
	var userID int64
	var expiresAt string
	err := s.db.QueryRow("SELECT user_id, expires_at FROM file_tokens WHERE token=?", token).Scan(&userID, &expiresAt)
	if err != nil {
		return 0, fmt.Errorf("invalid file token")
	}
	expires, _ := time.Parse(time.RFC3339, expiresAt)
	if time.Now().After(expires) {
		s.db.Exec("DELETE FROM file_tokens WHERE token=?", token)
		return 0, fmt.Errorf("file token expired")
	}
	// Consume token (single-use)
	s.db.Exec("DELETE FROM file_tokens WHERE token=?", token)
	return userID, nil
}

func (s *Store) CleanExpiredFileTokens() {
	s.db.Exec("DELETE FROM file_tokens WHERE expires_at < ?", Now())
}

func (s *Store) TotalFileSize() int64 {
	var total int64
	s.db.QueryRow("SELECT COALESCE(SUM(size), 0) FROM files").Scan(&total)
	return total
}
