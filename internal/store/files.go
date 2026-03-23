// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package store

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
