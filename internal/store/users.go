// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package store

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// User represents a registered user.
type User struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name,omitempty"`
	Role        string `json:"role"`
}

func (s *Store) CreateUser(username, pin, displayName string) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(pin), bcrypt.DefaultCost)
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

func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query("SELECT id, username, COALESCE(display_name,''), COALESCE(role,'member') FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role); err != nil {
			continue
		}
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
	//nolint:errcheck // returns false on error — non-admin is safe default
	s.db.QueryRow("SELECT COALESCE(role,'member') FROM users WHERE id=?", userID).Scan(&role)
	return role == "admin"
}

func (s *Store) DeleteUser(userID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	cascade := []string{
		"DELETE FROM channel_subscribers WHERE user_id=?",
		"DELETE FROM channel_reads WHERE user_id=?",
		"DELETE FROM push_subscriptions WHERE user_id=?",
		"DELETE FROM messages WHERE chat_id IN (SELECT id FROM chats WHERE user_id=?)",
		"DELETE FROM chats WHERE user_id=?",
		"DELETE FROM users WHERE id=?",
	}
	for _, stmt := range cascade {
		if _, err := tx.Exec(stmt, userID); err != nil {
			return fmt.Errorf("delete user %d: %w", userID, err)
		}
	}
	return tx.Commit()
}

// UserExists returns true if a user with the given ID exists.
func (s *Store) UserExists(userID int64) bool {
	var count int
	//nolint:errcheck // returns false on error — safe default
	s.db.QueryRow("SELECT COUNT(*) FROM users WHERE id=?", userID).Scan(&count)
	return count > 0
}

// ResetPassword updates the password (pin) for a user by username.
func (s *Store) ResetPassword(username, newPin string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPin), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash error")
	}
	res, err := s.db.Exec("UPDATE users SET pin=? WHERE username=?", string(hash), username)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

// GetUserByID returns a user by ID.
func (s *Store) GetUserByID(userID int64) (*User, error) {
	u := &User{}
	err := s.db.QueryRow("SELECT id, username, COALESCE(display_name,''), COALESCE(role,'member') FROM users WHERE id=?",
		userID).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role)
	if err != nil {
		return nil, err
	}
	return u, nil
}
