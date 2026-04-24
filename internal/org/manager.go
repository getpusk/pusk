// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package org

import (
	crand "crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"

	"github.com/pusk-platform/pusk/internal/metrics"
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
	MaxOrgs  int     // 0 = unlimited, default 1
}

func NewManager(dataDir string) (*Manager, error) {
	dir := filepath.Join(dataDir, "orgs")
	_ = os.MkdirAll(dir, 0o750)

	// Global token registry
	tokDB, err := sql.Open("sqlite", filepath.Join(dataDir, "tokens.db")+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("open tokens db: %w", err)
	}
	_, _ = tokDB.Exec(`CREATE TABLE IF NOT EXISTS tokens (
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
		_ = json.Unmarshal(data, &m.orgs)
	}

	// Ensure "default" org exists
	if !m.hasOrg("default") {
		m.orgs = append(m.orgs, Org{Slug: "default", Name: "Default", Created: store.Now()})
		_ = m.save()
	}

	slog.Info("orgs loaded", "count", len(m.orgs))
	metrics.OrgsTotal.Set(float64(len(m.orgs)))
	return m, nil
}

// RegisterToken maps a bot token to an org in the global registry
func (m *Manager) RegisterToken(token, orgSlug string) {
	_, _ = m.tokDB.Exec("INSERT OR REPLACE INTO tokens (token, org) VALUES (?, ?)", token, orgSlug)
}

// UnregisterToken removes a bot token from the global registry.
func (m *Manager) UnregisterToken(token string) {
	_, _ = m.tokDB.Exec("DELETE FROM tokens WHERE token=?", token)
}

func (m *Manager) registerTokenLocked(token, orgSlug string) {
	_, _ = m.tokDB.Exec("INSERT OR REPLACE INTO tokens (token, org) VALUES (?, ?)", token, orgSlug)
}

// OrgByToken looks up which org a bot token belongs to
func (m *Manager) OrgByToken(token string) (string, error) {
	var slug string
	err := m.tokDB.QueryRow("SELECT org FROM tokens WHERE token=?", token).Scan(&slug)
	if err != nil {
		return "default", nil //nolint:nilerr // fallback to default org for backwards compat
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
	return os.WriteFile(m.masterFn, data, 0o600)
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
		s.OrgID = slug
		return s, nil
	}

	if !m.hasOrg(slug) {
		return nil, fmt.Errorf("org not found: %s", slug)
	}

	orgDir := filepath.Join(m.dir, filepath.Base(slug))
	_ = os.MkdirAll(orgDir, 0o750)
	dbPath := filepath.Join(orgDir, "pusk.db")

	s, err := store.New(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open org db %s: %w", slug, err)
	}
	s.OrgID = slug
	m.stores[slug] = s
	slog.Info("org loaded", "slug", slug)
	return s, nil
}

// Register creates a new organization with admin user.
// If bypassLimit is true, the org limit check is skipped (admin token callers).
func (m *Manager) Register(slug, name, adminUser, adminPin string, bypassLimit bool) error {
	// Validate slug: alphanumeric + hyphens, 2-32 chars, no leading dot/slash
	if len(slug) < 2 || len(slug) > 32 {
		return fmt.Errorf("slug must be 2-32 characters")
	}
	for _, c := range slug {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return fmt.Errorf("slug must contain only lowercase letters, digits and hyphens")
		}
	}
	// EDGE-4: slug must have at least one alphanumeric character
	hasAlphaNum := false
	for _, c := range slug {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			hasAlphaNum = true
			break
		}
	}
	if !hasAlphaNum {
		return fmt.Errorf("slug must contain at least one letter or digit")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Enforce org limit (default org doesn't count toward the limit)
	if !bypassLimit && m.MaxOrgs > 0 && m.userOrgCount() >= m.MaxOrgs {
		return fmt.Errorf("max organizations limit reached (%d)", m.MaxOrgs)
	}
	if bypassLimit && m.MaxOrgs > 0 && m.userOrgCount() >= m.MaxOrgs {
		slog.Warn("org limit bypassed by admin token", "current", m.userOrgCount(), "max", m.MaxOrgs, "slug", slug)
	}

	if m.hasOrg(slug) {
		return fmt.Errorf("org already exists: %s", slug)
	}

	// Create org directory and database
	orgDir := filepath.Join(m.dir, filepath.Base(slug))
	_ = os.MkdirAll(orgDir, 0o750)
	dbPath := filepath.Join(orgDir, "pusk.db")

	s, err := store.New(dbPath)
	if err != nil {
		return fmt.Errorf("create org db: %w", err)
	}
	s.OrgID = slug
	m.stores[slug] = s

	// Create admin user
	admin, err := s.CreateUser(adminUser, adminPin, adminUser)
	if err != nil {
		return fmt.Errorf("create admin: %w", err)
	}
	_ = s.SetUserRole(admin.ID, "admin")

	// Create default system bot (needed for channels)
	tokenBytes := make([]byte, 16)
	crand.Read(tokenBytes)
	botToken := hex.EncodeToString(tokenBytes)
	sysBot, _ := s.CreateBot(botToken, name+" Bot")
	if sysBot != nil {
		m.registerTokenLocked(botToken, slug)

		// Create #general channel with welcome message
		ch, _ := s.CreateChannel(sysBot.ID, "general", "General channel")
		if ch != nil {
			_ = s.Subscribe(ch.ID, 1) // admin user_id = 1
			welcome := fmt.Sprintf("Welcome to **%s**! / Добро пожаловать в **%s**!\n\n"+
				"This is #general — your team's chat channel.\n"+
				"Это #general — канал для общения команды.\n\n"+
				"What you can do / Что можно делать:\n"+
				"• Send messages and reply / Писать и отвечать на сообщения\n"+
				"• @mention teammates — they get push / @упомянуть — придёт push\n"+
				"• Upload files and photos (📎) / Загрузить файлы и фото (📎)\n"+
				"• Create more channels / Создать ещё каналы\n\n"+
				"Connect monitoring → Settings ⚙️ → Webhook URL\n"+
				"Подключить мониторинг → Настройки ⚙️ → Webhook URL",
				name, name)
			_, _ = s.SaveChannelMessage(ch.ID, welcome, "", "", "")
		}

		// Welcome message from system bot
		admin, _ := s.AuthUser(adminUser, adminPin)
		if admin != nil {
			chat, _ := s.GetOrCreateChat(admin.ID, sysBot.ID)
			if chat != nil {
				welcome := fmt.Sprintf("Hi! I'm **%s Bot** — your system gateway.\n"+
					"Привет! Я **%s Bot** — системный бот.\n\n"+
					"Your bot token / Ваш bot token: `%s`\n\n"+
					"API: POST /bot/%s/sendMessage\n"+
					"Webhook: /hook/%s?format=alertmanager\n\n"+
					"Full docs / Документация: https://getpusk.ru",
					name, name, botToken, botToken, botToken)
				_, _ = s.SaveMessage(chat.ID, "bot", welcome, "", "", "")
			}
		}

		slog.Info("org system bot created", "bot", sysBot.Name, "token", botToken)
	}

	m.orgs = append(m.orgs, Org{
		Slug:    slug,
		Name:    name,
		Created: store.Now(),
	})

	// Use proper timestamp
	m.orgs[len(m.orgs)-1].Created = store.Now()

	_ = m.save()
	slog.Info("org registered", "slug", slug, "name", name)
	metrics.OrgsTotal.Set(float64(len(m.orgs)))
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

// userOrgCount returns the number of user-created orgs (excludes "default").
// Must be called with mu held.
func (m *Manager) userOrgCount() int {
	count := 0
	for _, o := range m.orgs {
		if o.Slug != "default" {
			count++
		}
	}
	return count
}

// CanCreateOrg reports whether a new org can be created within the limit.
func (m *Manager) CanCreateOrg() bool {
	if m.MaxOrgs <= 0 {
		return true
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.userOrgCount() < m.MaxOrgs
}

// Close closes all open stores

// DeleteOrg removes an org: closes store, removes from registry, deletes data directory.
// Protected: only "default" org cannot be deleted.
func (m *Manager) DeleteOrg(slug string) error {
	if slug == "default" {
		return fmt.Errorf("cannot delete default org")
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	// Close store if loaded
	if s, ok := m.stores[slug]; ok {
		_ = s.Close()
		delete(m.stores, slug)
	}

	// Remove from orgs list
	filtered := make([]Org, 0, len(m.orgs))
	found := false
	for _, o := range m.orgs {
		if o.Slug == slug {
			found = true
			continue
		}
		filtered = append(filtered, o)
	}
	if !found {
		return fmt.Errorf("org not found: %s", slug)
	}
	m.orgs = filtered
	_ = m.save()

	// Remove tokens for this org
	_, _ = m.tokDB.Exec("DELETE FROM tokens WHERE org = ?", slug)

	// Remove data directory
	orgDir := filepath.Join(m.dir, slug)
	if err := os.RemoveAll(orgDir); err != nil {
		slog.Warn("failed to remove org dir", "slug", slug, "err", err)
	}

	metrics.OrgsTotal.Set(float64(len(m.orgs)))
	slog.Info("org deleted", "slug", slug)
	return nil
}

func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for slug, s := range m.stores {
		_ = s.Close()
		slog.Info("org closed", "slug", slug)
	}
	if m.tokDB != nil {
		_ = m.tokDB.Close()
	}
}
