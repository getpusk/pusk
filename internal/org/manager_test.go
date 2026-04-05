// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package org

import (
	"os"
	"testing"
)

func tempManager(t *testing.T) *Manager {
	t.Helper()
	dir, err := os.MkdirTemp("", "pusk-org-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	m, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Close() })
	return m
}

func TestNewManager_CreatesDefault(t *testing.T) {
	m := tempManager(t)
	orgs := m.List()
	if len(orgs) != 1 {
		t.Fatalf("orgs = %d, want 1", len(orgs))
	}
	if orgs[0].Slug != "default" {
		t.Fatalf("slug = %q, want default", orgs[0].Slug)
	}
}

func TestGet_Default(t *testing.T) {
	m := tempManager(t)
	s, err := m.Get("default")
	if err != nil {
		t.Fatal(err)
	}
	if s == nil {
		t.Fatal("expected non-nil store")
	}
	if s.OrgID != "default" {
		t.Fatalf("OrgID = %q, want default", s.OrgID)
	}
}

func TestGet_NotFound(t *testing.T) {
	m := tempManager(t)
	_, err := m.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent org")
	}
}

func TestGet_LazyInit(t *testing.T) {
	m := tempManager(t)
	// First call creates the store
	s1, err := m.Get("default")
	if err != nil {
		t.Fatal(err)
	}
	// Second call returns cached store
	s2, err := m.Get("default")
	if err != nil {
		t.Fatal(err)
	}
	if s1 != s2 {
		t.Fatal("expected same store instance on second call")
	}
}

func TestRegister_Success(t *testing.T) {
	m := tempManager(t)
	err := m.Register("myteam", "My Team", "admin", "pass1234")
	if err != nil {
		t.Fatal(err)
	}
	orgs := m.List()
	if len(orgs) != 2 {
		t.Fatalf("orgs = %d, want 2", len(orgs))
	}
	s, err := m.Get("myteam")
	if err != nil {
		t.Fatal(err)
	}
	if s.OrgID != "myteam" {
		t.Fatalf("OrgID = %q", s.OrgID)
	}
}

func TestRegister_CreatesAdminAndBot(t *testing.T) {
	m := tempManager(t)
	err := m.Register("neworg", "New Org", "admin", "pass1234")
	if err != nil {
		t.Fatal(err)
	}
	s, _ := m.Get("neworg")
	// Admin should be able to auth
	user, err := s.AuthUser("admin", "pass1234")
	if err != nil {
		t.Fatalf("admin auth failed: %v", err)
	}
	if user == nil {
		t.Fatal("expected non-nil user")
	}
}

func TestRegister_Duplicate(t *testing.T) {
	m := tempManager(t)
	_ = m.Register("dup", "Dup", "admin", "pass1234")
	err := m.Register("dup", "Dup2", "admin2", "pass5678")
	if err == nil {
		t.Fatal("expected error for duplicate slug")
	}
}

func TestRegister_InvalidSlugs(t *testing.T) {
	m := tempManager(t)
	invalids := []string{
		"a",       // too short
		"",        // empty
		"A-Upper", // uppercase
		"my org",  // space
		"---",     // no alphanumeric
		"../evil", // path traversal
	}
	for _, slug := range invalids {
		err := m.Register(slug, "Test", "admin", "pass")
		if err == nil {
			t.Errorf("expected error for slug %q", slug)
		}
	}
}

func TestRegister_ValidSlugs(t *testing.T) {
	m := tempManager(t)
	valids := []string{"ab", "my-team", "team-42", "a1"}
	for _, slug := range valids {
		err := m.Register(slug, "Test "+slug, "admin", "pass1234")
		if err != nil {
			t.Errorf("unexpected error for slug %q: %v", slug, err)
		}
	}
}

func TestTokenRegistry(t *testing.T) {
	m := tempManager(t)
	_ = m.Register("org1", "Org 1", "admin", "pass1234")
	m.RegisterToken("tok123", "org1")
	slug, err := m.OrgByToken("tok123")
	if err != nil {
		t.Fatal(err)
	}
	if slug != "org1" {
		t.Fatalf("slug = %q, want org1", slug)
	}
}

func TestOrgByToken_UnknownFallsToDefault(t *testing.T) {
	m := tempManager(t)
	slug, err := m.OrgByToken("unknown-token")
	if err != nil {
		t.Fatal(err)
	}
	if slug != "default" {
		t.Fatalf("slug = %q, want default (fallback)", slug)
	}
}

func TestGetByToken(t *testing.T) {
	m := tempManager(t)
	_ = m.Register("org2", "Org 2", "admin", "pass1234")
	m.RegisterToken("tok456", "org2")
	s, slug, err := m.GetByToken("tok456")
	if err != nil {
		t.Fatal(err)
	}
	if slug != "org2" {
		t.Fatalf("slug = %q", slug)
	}
	if s == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestGetByToken_Unknown(t *testing.T) {
	m := tempManager(t)
	// Unknown token should fallback to default org
	s, slug, err := m.GetByToken("nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if slug != "default" {
		t.Fatalf("slug = %q, want default", slug)
	}
	if s == nil {
		t.Fatal("expected non-nil store for default")
	}
}

func TestManagerClose(t *testing.T) {
	m := tempManager(t)
	_ = m.Register("close-test", "Close", "admin", "pass1234")
	_, _ = m.Get("close-test")
	m.Close() // should not panic
}

func TestRegisterToken_Overwrite(t *testing.T) {
	m := tempManager(t)
	_ = m.Register("org-a", "A", "admin", "pass1234")
	_ = m.Register("org-b", "B", "admin", "pass1234")
	m.RegisterToken("shared-tok", "org-a")
	m.RegisterToken("shared-tok", "org-b") // overwrite
	slug, _ := m.OrgByToken("shared-tok")
	if slug != "org-b" {
		t.Fatalf("slug = %q, want org-b after overwrite", slug)
	}
}
