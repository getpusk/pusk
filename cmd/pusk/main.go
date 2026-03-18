// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/pusk-platform/pusk/internal/api"
	"github.com/pusk-platform/pusk/internal/auth"
	"github.com/pusk-platform/pusk/internal/bot"
	"github.com/pusk-platform/pusk/internal/notify"
	"github.com/pusk-platform/pusk/internal/store"
	"github.com/pusk-platform/pusk/internal/ws"
)

func main() {
	addr := flag.String("addr", ":8443", "listen address")
	dbPath := flag.String("db", "data/pusk.db", "SQLite database path")
	filesDir := flag.String("files", "data/files", "uploaded files directory")
	staticDir := flag.String("static", "web/static", "PWA static files")
	flag.Parse()

	if v := os.Getenv("PUSK_ADDR"); v != "" {
		*addr = v
	}
	if v := os.Getenv("PUSK_DB"); v != "" {
		*dbPath = v
	}

	os.MkdirAll("data", 0755)
	os.MkdirAll(*filesDir, 0755)

	db, err := store.New(*dbPath)
	if err != nil {
		log.Fatalf("Failed to init database: %v", err)
	}
	defer db.Close()

	// Demo mode: create guest user + DemoBot on first start
	if os.Getenv("PUSK_NO_DEMO") == "" {
		initDemo(db)
	}

	hub := ws.NewHub()

	// Push notifications (optional — set VAPID env vars to enable)
	vapidPub := os.Getenv("VAPID_PUBLIC_KEY")
	vapidPriv := os.Getenv("VAPID_PRIVATE_KEY")
	vapidEmail := os.Getenv("VAPID_EMAIL")
	push := notify.NewPushService(db, vapidPub, vapidPriv, vapidEmail)
	if vapidPub != "" {
		log.Printf("  Push:       VAPID configured")
	}

	// JWT auth
	jwtSecret := os.Getenv("PUSK_JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = loadOrGenerateSecret("data/jwt.secret")
	}
	jwtSvc := auth.NewJWTService(jwtSecret, 720) // 30 days

	mux := http.NewServeMux()

	// Bot API (Telegram-compatible)
	botHandler := bot.NewHandler(db, hub, push, *filesDir)
	botHandler.Route(mux)

	// Client API (for PWA)
	clientAPI := api.NewClientAPI(db, hub, push, botHandler.Relay(), vapidPub, jwtSvc)
	clientAPI.Route(mux)

	// Static files (PWA)
	mux.Handle("GET /", http.FileServer(http.Dir(*staticDir)))

	// Admin auth helper
	adminToken := os.Getenv("PUSK_ADMIN_TOKEN")
	checkAdmin := func(r *http.Request) bool {
		if adminToken == "" {
			return false
		}
		return strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ") == adminToken
	}

	// Admin: register bot
	mux.HandleFunc("POST /admin/bots", func(w http.ResponseWriter, r *http.Request) {
		if !checkAdmin(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var req struct {
			Token string `json:"token"`
			Name  string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		b, err := db.CreateBot(req.Token, req.Name)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		log.Printf("[admin] bot registered: %s (token: %s...)", b.Name, b.Token[:8])
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(b)
	})

	mux.HandleFunc("POST /admin/channel", func(w http.ResponseWriter, r *http.Request) {
		if !checkAdmin(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
			return
		}
		bots, _ := db.ListBots()
		if len(bots) == 0 {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "create a bot first"})
			return
		}
		ch, err := db.CreateChannel(bots[0].ID, req.Name, req.Description)
		if err != nil {
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
			return
		}
		log.Printf("[admin] channel created: %s", ch.Name)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "result": ch})
	})

	log.Printf("Pusk server starting on %s", *addr)
	log.Printf("  Bot API:    POST /bot{token}/sendMessage")
	log.Printf("  Client API: GET  /api/health")
	log.Printf("  PWA:        GET  /")
	log.Printf("  Admin:      POST /admin/bots")

	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func loadOrGenerateSecret(path string) string {
	data, err := os.ReadFile(path)
	if err == nil {
		s := strings.TrimSpace(string(data))
		if len(s) >= 32 {
			return s
		}
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		log.Fatalf("Failed to generate JWT secret: %v", err)
	}
	secret := hex.EncodeToString(b)
	os.WriteFile(path, []byte(secret+"\n"), 0600)
	log.Printf("  JWT:        generated new secret → %s", path)
	return secret
}
