// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package store

// Chat represents a private chat between a user and a bot.
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
	defer func() { _ = rows.Close() }()
	var chats []Chat
	for rows.Next() {
		var c Chat
		if err := rows.Scan(&c.ID, &c.UserID, &c.BotID); err != nil {
			continue
		}
		chats = append(chats, c)
	}
	return chats, nil
}

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
