// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package store

import (
	"fmt"
	"time"
)

func (s *Store) CreateInvite(code string, ttl time.Duration) error {
	expires := time.Now().Add(ttl).UTC().Format(time.RFC3339)
	_, err := s.db.Exec("INSERT INTO invites (code, expires_at) VALUES (?, ?)", code, expires)
	return err
}

func (s *Store) UseInvite(code string) error {
	res, err := s.db.Exec(
		"UPDATE invites SET uses=uses+1 WHERE code=? AND uses<COALESCE(max_uses,50) AND expires_at > ?",
		code, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("invite error")
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		var uses, maxUses int
		var expiresAt string
		err := s.db.QueryRow("SELECT COALESCE(uses,0), COALESCE(max_uses,50), expires_at FROM invites WHERE code=?", code).Scan(&uses, &maxUses, &expiresAt)
		if err != nil {
			return fmt.Errorf("invite not found")
		}
		if uses >= maxUses {
			return fmt.Errorf("invite limit reached")
		}
		return fmt.Errorf("invite expired")
	}
	return nil
}

// ValidateInvite checks if an invite code is valid without consuming it.
func (s *Store) ValidateInvite(code string) error {
	var uses, maxUses int
	var expiresAt string
	err := s.db.QueryRow("SELECT COALESCE(uses,0), COALESCE(max_uses,50), expires_at FROM invites WHERE code=?", code).Scan(&uses, &maxUses, &expiresAt)
	if err != nil {
		return fmt.Errorf("invite not found")
	}
	if uses >= maxUses {
		return fmt.Errorf("invite limit reached")
	}
	expTime, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil || time.Now().UTC().After(expTime) {
		return fmt.Errorf("invite expired")
	}
	return nil
}

// RevokeInvite deletes an invite code.
func (s *Store) RevokeInvite(code string) error {
	_, err := s.db.Exec("DELETE FROM invites WHERE code=?", code)
	return err
}

// ActiveInvite returns the most recent non-expired invite code, or empty string.
func (s *Store) ActiveInvite() (string, error) {
	var code string
	err := s.db.QueryRow(
		"SELECT code FROM invites WHERE COALESCE(uses,0)<COALESCE(max_uses,50) AND expires_at > ? ORDER BY created_at DESC LIMIT 1",
		time.Now().UTC().Format(time.RFC3339)).Scan(&code)
	if err != nil {
		return "", nil // no active invite
	}
	return code, nil
}
