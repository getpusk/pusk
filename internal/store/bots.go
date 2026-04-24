// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package store

import "fmt"

// Bot represents a registered bot.
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
	rows, err := s.db.Query("SELECT id, token, name, COALESCE(webhook_url,''), COALESCE(icon_url,'') FROM bots ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var bots []Bot
	for rows.Next() {
		var b Bot
		if err := rows.Scan(&b.ID, &b.Token, &b.Name, &b.WebhookURL, &b.IconURL); err != nil {
			continue
		}
		bots = append(bots, b)
	}
	return bots, nil
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

// RenameBot updates the name of a bot.
func (s *Store) RenameBot(botID int64, name string) error {
	_, err := s.db.Exec("UPDATE bots SET name=? WHERE id=?", name, botID)
	return err
}

// DeleteBot removes a bot if it has no channels assigned.
func (s *Store) DeleteBot(botID int64) error {
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM channels WHERE bot_id=?", botID).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("bot has %d channel(s) — reassign them first", count)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM callback_queue WHERE bot_id=?", botID); err != nil {
		return fmt.Errorf("delete callbacks: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM files WHERE bot_id=?", botID); err != nil {
		return fmt.Errorf("delete files: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM messages WHERE chat_id IN (SELECT id FROM chats WHERE bot_id=?)", botID); err != nil {
		return fmt.Errorf("delete messages: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM chats WHERE bot_id=?", botID); err != nil {
		return fmt.Errorf("delete chats: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM bots WHERE id=?", botID); err != nil {
		return fmt.Errorf("delete bot: %w", err)
	}
	return tx.Commit()
}
