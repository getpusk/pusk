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
	res, err := s.db.Exec("UPDATE invites SET used=TRUE WHERE code=? AND used=FALSE AND expires_at > ?", code, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("invite error")
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		// Check why: not found, already used, or expired
		var used bool
		var expiresAt string
		err := s.db.QueryRow("SELECT used, expires_at FROM invites WHERE code=?", code).Scan(&used, &expiresAt)
		if err != nil {
			return fmt.Errorf("invite not found")
		}
		if used {
			return fmt.Errorf("invite already used")
		}
		return fmt.Errorf("invite expired")
	}
	return nil
}
