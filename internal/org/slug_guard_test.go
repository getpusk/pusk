// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package org

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

var traversalSlugs = []string{
	"..", ".", "../evil", "../../etc", "x/../..", "a/b", `a\b`, "/abs", "%2e%2e", "-", "--", "",
}

func TestValidSlug(t *testing.T) {
	for _, s := range []string{"ab", "a1", "my-team", "team-42", "default", "abcdefghijklmnopqrstuvwxyz012345"} {
		if err := validSlug(s); err != nil {
			t.Errorf("validSlug(%q) = %v, want nil", s, err)
		}
	}
	bad := append([]string{"a", "A-Upper", "my org", "---", "abcdefghijklmnopqrstuvwxyz0123456", "ab\n"}, traversalSlugs...)
	for _, s := range bad {
		if err := validSlug(s); err == nil {
			t.Errorf("validSlug(%q) = nil, want error", s)
		}
	}
}

// newManagerWithMaster writes master.json and a canary file into a fresh data dir
// before opening the manager, so tests can check nothing escapes data/orgs/.
func newManagerWithMaster(t *testing.T, orgs []Org) (*Manager, string) {
	t.Helper()
	data := t.TempDir()
	if err := os.WriteFile(filepath.Join(data, "canary"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if orgs != nil {
		raw, _ := json.Marshal(orgs)
		if err := os.WriteFile(filepath.Join(data, "master.json"), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	m, err := NewManager(data)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Close() })
	return m, data
}

func assertDataRootUntouched(t *testing.T, data string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(data, "canary")); err != nil {
		t.Fatalf("data dir content removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(data, "pusk.db")); err == nil {
		t.Fatal("pusk.db created in data root")
	}
	ents, _ := os.ReadDir(filepath.Join(data, "orgs"))
	for _, e := range ents {
		if e.Name() != "default" && e.Name() != "acme" {
			t.Errorf("unexpected entry orgs/%s", e.Name())
		}
	}
}

func TestTraversalSlugsRejected(t *testing.T) {
	m, data := newManagerWithMaster(t, nil)
	for _, s := range traversalSlugs {
		if err := m.Register(s, "n", "admin", "pass1234", true); err == nil {
			t.Errorf("Register(%q) accepted", s)
		}
		if _, err := m.Get(s); err == nil {
			t.Errorf("Get(%q) opened a store", s)
		}
		if err := m.DeleteOrg(s); err == nil {
			t.Errorf("DeleteOrg(%q) succeeded", s)
		}
	}
	assertDataRootUntouched(t, data)

	// Positive control: a valid org still goes through the full lifecycle.
	if err := m.Register("acme", "Acme", "admin", "pass1234", true); err != nil {
		t.Fatalf("Register(acme): %v", err)
	}
	if _, err := m.Get("acme"); err != nil {
		t.Fatalf("Get(acme): %v", err)
	}
	if _, err := os.Stat(filepath.Join(data, "orgs", "acme", "pusk.db")); err != nil {
		t.Fatalf("acme db: %v", err)
	}
	if err := m.DeleteOrg("acme"); err != nil {
		t.Fatalf("DeleteOrg(acme): %v", err)
	}
	if _, err := os.Stat(filepath.Join(data, "orgs", "acme")); !os.IsNotExist(err) {
		t.Fatalf("acme dir still present after delete: %v", err)
	}
}

// A hand-edited or corrupted master.json must not let Get/DeleteOrg reach
// outside data/orgs/, and must not silently drop entries from the registry.
func TestTamperedMasterJSON(t *testing.T) {
	m, data := newManagerWithMaster(t, []Org{
		{Slug: "default", Name: "Default"},
		{Slug: "..", Name: "dotdot"},
		{Slug: "../outside", Name: "outside"},
		{Slug: "acme", Name: "Acme"},
	})

	for _, s := range []string{"..", "../outside"} {
		if _, err := m.Get(s); err == nil {
			t.Errorf("Get(%q) opened a store", s)
		}
		if err := m.DeleteOrg(s); err == nil {
			t.Errorf("DeleteOrg(%q) succeeded", s)
		}
	}
	if _, err := os.Stat(filepath.Join(data, "orgs", "outside")); err == nil {
		t.Error("orgs/outside created for slug ../outside")
	}

	// Valid entries from the same file keep working.
	if _, err := m.Get("acme"); err != nil {
		t.Fatalf("Get(acme): %v", err)
	}
	assertDataRootUntouched(t, data)

	// Invalid entries stay in the registry (reported, not deleted), so the
	// operator can fix master.json by hand without losing the record.
	if err := m.Register("beta", "Beta", "admin", "pass1234", true); err != nil {
		t.Fatalf("Register(beta): %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(data, "master.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved []Org
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, o := range saved {
		have[o.Slug] = true
	}
	for _, s := range []string{"default", "..", "../outside", "acme", "beta"} {
		if !have[s] {
			t.Errorf("master.json lost entry %q after save", s)
		}
	}
}
