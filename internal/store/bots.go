// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package store

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
	rows, err := s.db.Query("SELECT id, token, name, COALESCE(webhook_url,''), COALESCE(icon_url,'') FROM bots")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
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
