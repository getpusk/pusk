// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package store

import (
	"strings"
	"testing"
)

// TestPragmasApplied asserts that the DSN pragmas actually take effect on a
// file-backed database: WAL journal, foreign keys on, and a non-zero busy
// timeout. This is the regression guard for the silently-ignored mattn-style
// DSN keys (issue #146).
func TestPragmasApplied(t *testing.T) {
	s, err := New(t.TempDir() + "/pragma.db")
	if err != nil {
		t.Fatalf("New(file): %v", err)
	}
	defer func() { _ = s.Close() }()

	var mode string
	if err := s.DB().QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if !strings.EqualFold(mode, "wal") {
		t.Errorf("journal_mode = %q, want wal", mode)
	}

	var fk int
	if err := s.DB().QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("read foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys = %d, want 1", fk)
	}

	var busy int
	if err := s.DB().QueryRow("PRAGMA busy_timeout").Scan(&busy); err != nil {
		t.Fatalf("read busy_timeout: %v", err)
	}
	if busy != 5000 {
		t.Errorf("busy_timeout = %d, want 5000", busy)
	}
}

// TestForeignKeysEnforced proves foreign key constraints are enforced at
// runtime: inserting a child row whose parents do not exist must fail. Before
// #146 the REFERENCES clauses were inert and this insert silently succeeded.
func TestForeignKeysEnforced(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.DB().Exec(
		"INSERT INTO chats (user_id, bot_id) VALUES (999999, 888888)",
	); err == nil {
		t.Fatal("expected FK violation inserting chat with missing user/bot, got nil")
	}
}
