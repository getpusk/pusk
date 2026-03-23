// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/pusk-platform/pusk/internal/api"
	"github.com/pusk-platform/pusk/internal/auth"
	"github.com/pusk-platform/pusk/internal/bot"
	_ "github.com/pusk-platform/pusk/internal/metrics"
	"github.com/pusk-platform/pusk/internal/notify"
	"github.com/pusk-platform/pusk/internal/org"
	"github.com/pusk-platform/pusk/internal/store"
	"github.com/pusk-platform/pusk/internal/ws"
)

func main() {
	// Structured logging
	var handler slog.Handler
	if os.Getenv("PUSK_LOG_FORMAT") == "json" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	}
	slog.SetDefault(slog.New(handler))

	addr := flag.String("addr", ":8443", "listen address")
	filesDir := flag.String("files", "data/files", "uploaded files directory")
	staticDir := flag.String("static", "web/static", "PWA static files")
	flag.Parse()

	if v := os.Getenv("PUSK_ADDR"); v != "" {
		*addr = v
	}
	os.MkdirAll("data", 0755)
	os.MkdirAll(*filesDir, 0755)

	// Multi-tenant org manager
	orgs, err := org.NewManager("data")
	if err != nil {
		slog.Error("failed to init org manager", "error", err)
		os.Exit(1)
	}
	defer orgs.Close()

	// Default org store (backwards compatible)
	db, err := orgs.Get("default")
	if err != nil {
		slog.Error("failed to init default org", "error", err)
		os.Exit(1)
	}

	// Demo mode: create guest user + DemoBot on first start
	if os.Getenv("PUSK_NO_DEMO") == "" {
		initDemo(db)
	}

	hub := ws.NewHub()

	// Push notifications (optional — set VAPID env vars to enable)
	vapidPub := os.Getenv("VAPID_PUBLIC_KEY")
	vapidPriv := os.Getenv("VAPID_PRIVATE_KEY")
	vapidEmail := os.Getenv("VAPID_EMAIL")
	push := notify.NewPushService(vapidPub, vapidPriv, vapidEmail)
	if vapidPub != "" {
		slog.Info("push notifications configured", "provider", "VAPID")
	}

	// JWT auth
	jwtSecret := os.Getenv("PUSK_JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = loadOrGenerateSecret("data/jwt.secret")
	}
	jwtSvc := auth.NewJWTService(jwtSecret, 168) // 7 days

	mux := http.NewServeMux()

	// Bot API (Telegram-compatible)
	botHandler := bot.NewHandler(orgs, db, hub, push, jwtSvc, *filesDir)
	botHandler.Route(mux)

	// Client API (for PWA)
	clientAPI := api.NewClientAPI(orgs, db, hub, push, botHandler.Relay(), botHandler.Updates(), vapidPub, jwtSvc)
	clientAPI.Route(mux)

	// Invite redirect → PWA with invite param
	mux.HandleFunc("GET /invite/", func(w http.ResponseWriter, r *http.Request) {
		code := strings.TrimPrefix(r.URL.Path, "/invite/")
		org := r.URL.Query().Get("org")
		target := "/?invite=" + code
		if org != "" {
			target += "&org=" + org
		}
		http.Redirect(w, r, target, http.StatusFound)
	})

	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		if host != "127.0.0.1" && host != "::1" && !strings.HasPrefix(host, "10.") && !strings.HasPrefix(host, "192.168.") {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		promhttp.Handler().ServeHTTP(w, r)
	})
	// Static files (PWA)
	mux.Handle("GET /", http.FileServer(http.Dir(*staticDir)))

	// Auth helper: ADMIN_TOKEN (global) or JWT (org user)
	adminToken := os.Getenv("PUSK_ADMIN_TOKEN")
	getOrgStore := func(r *http.Request) (*store.Store, bool) {
		authHeader := r.Header.Get("Authorization")
		// Try ADMIN_TOKEN first (global admin → default org)
		if adminToken != "" && strings.TrimPrefix(authHeader, "Bearer ") == adminToken {
			return db, true
		}
		// Try JWT (org user → their org store)
		if jwtSvc != nil && authHeader != "" {
			if claims, err := jwtSvc.Validate(authHeader); err == nil {
				if s, err := orgs.Get(claims.OrgID); err == nil {
					return s, true
				}
			}
		}
		return nil, false
	}

	// requireAdmin checks that the request is from admin token or a JWT user with admin role
	requireAdmin := func(r *http.Request, s *store.Store) bool {
		authHeader := r.Header.Get("Authorization")
		if adminToken != "" && strings.TrimPrefix(authHeader, "Bearer ") == adminToken {
			return true // global admin token
		}
		// JWT user — verify admin role
		if jwtSvc != nil && authHeader != "" {
			if claims, err := jwtSvc.Validate(authHeader); err == nil {
				return s.IsAdmin(claims.UserID)
			}
		}
		return false
	}

	// Admin: register bot
	mux.HandleFunc("POST /admin/bots", func(w http.ResponseWriter, r *http.Request) {
		s, ok := getOrgStore(r)
		if !ok {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if !requireAdmin(r, s) {
			http.Error(w, `{"error":"admin only"}`, http.StatusForbidden)
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
		b, err := s.CreateBot(req.Token, req.Name)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		// Register token globally for Bot API routing
		claims, _ := jwtSvc.Validate(r.Header.Get("Authorization"))
		if claims != nil {
			orgs.RegisterToken(req.Token, claims.OrgID)
		} else {
			orgs.RegisterToken(req.Token, "default")
		}
		slog.Info("bot registered", "bot", b.Name, "token_prefix", b.Token[:8])
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(b)
	})

	mux.HandleFunc("POST /admin/channel", func(w http.ResponseWriter, r *http.Request) {
		s, ok := getOrgStore(r)
		if !ok {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if !requireAdmin(r, s) {
			http.Error(w, `{"error":"admin only"}`, http.StatusForbidden)
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
		bots, _ := s.ListBots()
		if len(bots) == 0 {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "create a bot first"})
			return
		}
		ch, err := s.CreateChannel(bots[0].ID, req.Name, req.Description)
		if err != nil {
			errMsg := "Channel already exists / Канал уже существует"
			if !strings.Contains(err.Error(), "UNIQUE") {
				errMsg = err.Error()
			}
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": errMsg})
			return
		}
		slog.Info("channel created", "channel", ch.Name)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "result": ch})
	})

	mux.HandleFunc("DELETE /admin/channel/{channelID}", func(w http.ResponseWriter, r *http.Request) {
		s, ok := getOrgStore(r)
		if !ok {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if !requireAdmin(r, s) {
			http.Error(w, `{"error":"admin only"}`, http.StatusForbidden)
			return
		}
		channelID, _ := strconv.ParseInt(r.PathValue("channelID"), 10, 64)
		if err := s.DeleteChannel(channelID); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
			return
		}
		slog.Info("channel deleted", "channel_id", channelID)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	})

	// Org registration
	orgRL := api.NewRateLimiter(10, time.Minute) // 10 org registrations per minute per IP
	mux.HandleFunc("POST /api/org/register", api.RateLimit(orgRL, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Slug     string `json:"slug"`
			Name     string `json:"name"`
			Username string `json:"username"`
			Pin      string `json:"pin"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, 400)
			return
		}
		if req.Slug == "" || req.Username == "" || req.Pin == "" {
			http.Error(w, `{"error":"slug, username and pin required"}`, 400)
			return
		}
		if len(req.Pin) < 6 {
			http.Error(w, `{"error":"password must be at least 6 characters"}`, 400)
			return
		}
		if err := orgs.Register(req.Slug, req.Name, req.Username, req.Pin); err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, 400)
			return
		}
		// Generate JWT for the new admin
		s, _ := orgs.Get(req.Slug)
		user, _ := s.AuthUser(req.Username, req.Pin)
		tok, _ := jwtSvc.Generate(user.ID, req.Slug, req.Username)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":       true,
			"org":      req.Slug,
			"token":    tok,
			"user_id":  user.ID,
			"username": req.Username,
		})
		slog.Info("org registered", "slug", req.Slug, "admin", req.Username)
	}))

	slog.Info("server starting", "addr", *addr)
	slog.Info("routes",
		"bot_api", "POST /bot{token}/sendMessage",
		"client_api", "GET /api/health",
		"pwa", "GET /",
		"admin", "POST /admin/bots",
	)

	srv := &http.Server{Addr: *addr, Handler: api.RequestLogger(bot.TelegramCompat(mux))}

	// Graceful shutdown on SIGTERM/SIGINT
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
		sig := <-sigCh
		slog.Info("shutting down", "signal", sig.String())

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped gracefully")
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
		slog.Error("failed to generate JWT secret", "error", err)
		os.Exit(1)
	}
	secret := hex.EncodeToString(b)
	os.WriteFile(path, []byte(secret+"\n"), 0600)
	slog.Info("JWT secret generated", "path", path)
	return secret
}
