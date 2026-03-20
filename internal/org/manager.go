// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package org

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"

	"github.com/pusk-platform/pusk/internal/store"
)

// Org represents a registered organization
type Org struct {
	Slug    string `json:"slug"`
	Name    string `json:"name"`
	Created string `json:"created"`
}

// Manager manages per-tenant SQLite databases
type Manager struct {
	mu       sync.RWMutex
	stores   map[string]*store.Store
	orgs     []Org
	dir      string  // data/orgs/
	masterFn string  // data/master.json
	tokDB    *sql.DB // global token→org mapping
}

func NewManager(dataDir string) (*Manager, error) {
	dir := filepath.Join(dataDir, "orgs")
	os.MkdirAll(dir, 0755)

	// Global token registry
	tokDB, err := sql.Open("sqlite", filepath.Join(dataDir, "tokens.db")+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("open tokens db: %w", err)
	}
	tokDB.Exec(`CREATE TABLE IF NOT EXISTS tokens (
		token TEXT PRIMARY KEY,
		org   TEXT NOT NULL
	)`)

	m := &Manager{
		stores:   make(map[string]*store.Store),
		dir:      dir,
		masterFn: filepath.Join(dataDir, "master.json"),
		tokDB:    tokDB,
	}

	// Load existing orgs
	if data, err := os.ReadFile(m.masterFn); err == nil {
		json.Unmarshal(data, &m.orgs)
	}

	// Ensure "default" org exists
	if !m.hasOrg("default") {
		m.orgs = append(m.orgs, Org{Slug: "default", Name: "Default", Created: store.Now()})
		m.save()
	}

	log.Printf("  Orgs:       %d registered", len(m.orgs))
	return m, nil
}

// RegisterToken maps a bot token to an org in the global registry
func (m *Manager) RegisterToken(token, orgSlug string) {
	m.tokDB.Exec("INSERT OR REPLACE INTO tokens (token, org) VALUES (?, ?)", token, orgSlug)
}

func (m *Manager) registerTokenLocked(token, orgSlug string) {
	m.tokDB.Exec("INSERT OR REPLACE INTO tokens (token, org) VALUES (?, ?)", token, orgSlug)
}

// OrgByToken looks up which org a bot token belongs to
func (m *Manager) OrgByToken(token string) (string, error) {
	var slug string
	err := m.tokDB.QueryRow("SELECT org FROM tokens WHERE token=?", token).Scan(&slug)
	if err != nil {
		return "default", nil // fallback to default for backwards compat
	}
	return slug, nil
}

// GetByToken returns the Store for the org that owns the given bot token
func (m *Manager) GetByToken(token string) (*store.Store, string, error) {
	slug, _ := m.OrgByToken(token)
	s, err := m.Get(slug)
	return s, slug, err
}

func (m *Manager) hasOrg(slug string) bool {
	for _, o := range m.orgs {
		if o.Slug == slug {
			return true
		}
	}
	return false
}

func (m *Manager) save() error {
	data, _ := json.MarshalIndent(m.orgs, "", "  ")
	return os.WriteFile(m.masterFn, data, 0600)
}

// Get returns the Store for an org, creating it lazily
func (m *Manager) Get(slug string) (*store.Store, error) {
	m.mu.RLock()
	s, ok := m.stores[slug]
	m.mu.RUnlock()
	if ok {
		return s, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if s, ok := m.stores[slug]; ok {
		return s, nil
	}

	if !m.hasOrg(slug) {
		return nil, fmt.Errorf("org not found: %s", slug)
	}

	orgDir := filepath.Join(m.dir, slug)
	os.MkdirAll(orgDir, 0755)
	dbPath := filepath.Join(orgDir, "pusk.db")

	s, err := store.New(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open org db %s: %w", slug, err)
	}
	m.stores[slug] = s
	log.Printf("[org] loaded: %s", slug)
	return s, nil
}

// Register creates a new organization with admin user
func (m *Manager) Register(slug, name, adminUser, adminPin string) error {
	// Validate slug: alphanumeric + hyphens, 2-32 chars, no leading dot/slash
	if len(slug) < 2 || len(slug) > 32 {
		return fmt.Errorf("slug must be 2-32 characters")
	}
	for _, c := range slug {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return fmt.Errorf("slug must contain only lowercase letters, digits and hyphens")
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.hasOrg(slug) {
		return fmt.Errorf("org already exists: %s", slug)
	}

	// Create org directory and database
	orgDir := filepath.Join(m.dir, slug)
	os.MkdirAll(orgDir, 0755)
	dbPath := filepath.Join(orgDir, "pusk.db")

	s, err := store.New(dbPath)
	if err != nil {
		return fmt.Errorf("create org db: %w", err)
	}
	m.stores[slug] = s

	// Create admin user
	admin, err := s.CreateUser(adminUser, adminPin, adminUser)
	if err != nil {
		return fmt.Errorf("create admin: %w", err)
	}
	s.SetUserRole(admin.ID, "admin")

	// Create default system bot (needed for channels)
	botToken := slug + "-system-" + fmt.Sprintf("%d", len(m.orgs))
	sysBot, _ := s.CreateBot(botToken, name+" Bot")
	if sysBot != nil {
		m.registerTokenLocked(botToken, slug)

		// Create #general channel
		ch, _ := s.CreateChannel(sysBot.ID, "general", "General channel")
		if ch != nil {
			s.Subscribe(ch.ID, 1) // admin user_id = 1
		}

		// Welcome message from system bot
		admin, _ := s.AuthUser(adminUser, adminPin)
		if admin != nil {
			chat, _ := s.GetOrCreateChat(admin.ID, sysBot.ID)
			if chat != nil {
				welcome := fmt.Sprintf("Добро пожаловать в **%s**!\n\nВаш бот-шлюз готов к работе. Отправьте первое сообщение через API:\n\n```\ncurl -X POST https://your-server/bot/%s/sendMessage \\\n  -H 'Content-Type: application/json' \\\n  -d '{\"chat_id\": %d, \"text\": \"Hello!\"}'\n```\n\nBot token: `%s`", name, botToken, chat.ID, botToken)
				s.SaveMessage(chat.ID, "bot", welcome, "", "", "")
			}
		}

		log.Printf("[org] system bot created: %s (token: %s)", sysBot.Name, botToken)
	}

	m.orgs = append(m.orgs, Org{
		Slug:    slug,
		Name:    name,
		Created: fmt.Sprintf("%v", os.Getenv("_")), // will be overwritten
	})

	// Use proper timestamp
	m.orgs[len(m.orgs)-1].Created = store.Now()

	m.save()
	log.Printf("[org] registered: %s (%s)", slug, name)
	return nil
}

// List returns all registered organizations
func (m *Manager) List() []Org {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Org, len(m.orgs))
	copy(out, m.orgs)
	return out
}

// Close closes all open stores
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for slug, s := range m.stores {
		s.Close()
		log.Printf("[org] closed: %s", slug)
	}
	if m.tokDB != nil {
		m.tokDB.Close()
	}
}
