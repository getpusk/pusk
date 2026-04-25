// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/pusk-platform/pusk/internal/api"
	"github.com/pusk-platform/pusk/internal/auth"
	"github.com/pusk-platform/pusk/internal/bot"
	_ "github.com/pusk-platform/pusk/internal/metrics"
	"github.com/pusk-platform/pusk/internal/notify"
	"github.com/pusk-platform/pusk/internal/org"
	"github.com/pusk-platform/pusk/internal/ws"
)

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; connect-src 'self' wss:; font-src 'self'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		if r.Header.Get("X-Forwarded-Proto") == "https" {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

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
	healthcheck := flag.Bool("healthcheck", false, "run health check and exit")
	flag.Parse()

	if *healthcheck {
		a := ":8443"
		if v := os.Getenv("PUSK_ADDR"); v != "" {
			a = v
		}
		host := "localhost"
		port := strings.TrimPrefix(a, ":")
		if strings.Contains(a, ":") && !strings.HasPrefix(a, ":") {
			host = a[:strings.LastIndex(a, ":")]
			port = a[strings.LastIndex(a, ":")+1:]
		}
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 3*time.Second) // #nosec G704 -- healthcheck dials own server
		if err != nil {
			os.Exit(1)
		}
		_ = conn.Close()
		os.Exit(0)
	}

	if v := os.Getenv("PUSK_ADDR"); v != "" {
		*addr = v
	}
	_ = os.MkdirAll("data", 0o750)
	_ = os.MkdirAll(*filesDir, 0o750)

	// Multi-tenant org manager
	orgs, err := org.NewManager("data")
	if err != nil {
		slog.Error("failed to init org manager", "error", err)
		os.Exit(1)
	}
	defer orgs.Close()

	// Org creation limit (default: 1 user-created org; 0 = unlimited)
	maxOrgs := 1
	if v := os.Getenv("PUSK_MAX_ORGS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			maxOrgs = n
		}
	}
	orgs.MaxOrgs = maxOrgs
	slog.Info("org limit configured", "max_orgs", maxOrgs)

	// Default org store (backwards compatible)
	db, err := orgs.Get("default")
	if err != nil {
		slog.Error("failed to init default org", "error", err)
		os.Exit(1)
	}

	// Demo mode: create guest user + DemoBot (opt-in: PUSK_DEMO=1)
	if os.Getenv("PUSK_DEMO") == "1" {
		slog.Warn("DEMO MODE ACTIVE — disable in production by removing PUSK_DEMO=1")
		initDemo(db)
	}

	hub := ws.NewHub()

	// Push notifications (optional — set VAPID env vars or auto-generate)
	vapidPub := os.Getenv("VAPID_PUBLIC_KEY")
	vapidPriv := os.Getenv("VAPID_PRIVATE_KEY")
	vapidEmail := os.Getenv("VAPID_EMAIL")
	if vapidPub == "" || vapidPriv == "" {
		vapidPub, vapidPriv = loadOrGenerateVAPID("data/vapid.pub", "data/vapid.key")
	}
	if vapidEmail == "" {
		vapidEmail = "admin@localhost"
	}
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

	adminToken := os.Getenv("PUSK_ADMIN_TOKEN")
	if adminToken == "" {
		slog.Warn("PUSK_ADMIN_TOKEN not set — org creation is unprotected, set it for production use")
	}

	mux := http.NewServeMux()

	// Bot API (Telegram-compatible)
	botHandler := bot.NewHandler(orgs, db, hub, push, jwtSvc, *filesDir)
	botHandler.Route(mux)

	// Client API (for PWA)
	clientAPI := api.NewClientAPI(orgs, db, hub, push, botHandler.Relay(), botHandler.Updates(), vapidPub, jwtSvc)
	if os.Getenv("PUSK_OPEN_USER_REGISTRATION") == "false" {
		clientAPI.OpenUserReg = false
		slog.Info("user self-registration disabled, invite required")
	}
	if os.Getenv("PUSK_DEMO") == "1" {
		clientAPI.DemoMode = true
	}
	clientAPI.Route(mux)

	// Admin API (admin endpoints + org registration)
	adminAPI := api.NewAdminAPI(orgs, db, jwtSvc, adminToken)
	if os.Getenv("PUSK_DEMO") == "1" {
		adminAPI.DemoMode = true
	}
	adminAPI.Route(mux)

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
		if host != "127.0.0.1" && host != "::1" && !strings.HasPrefix(host, "10.") && !strings.HasPrefix(host, "192.168.") && !strings.HasPrefix(host, "172.") && !strings.HasPrefix(host, "100.") {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		promhttp.Handler().ServeHTTP(w, r)
	})
	// Static files (PWA) — no HTTP cache for JS/CSS, SW manages caching
	fs := http.FileServer(http.Dir(*staticDir))
	mux.Handle("GET /", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if strings.HasSuffix(p, ".js") || strings.HasSuffix(p, ".css") {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			r.Header.Del("If-Modified-Since")
			r.Header.Del("If-None-Match")
		}
		fs.ServeHTTP(w, r)
	}))

	// Message retention cleanup (default 30 days, PUSK_MSG_RETENTION_DAYS env)
	retentionDays := 30
	if v := os.Getenv("PUSK_MSG_RETENTION_DAYS"); v != "" {
		if d, err := strconv.Atoi(v); err == nil && d > 0 {
			retentionDays = d
		}
	}
	if os.Getenv("PUSK_MSG_RETENTION_DAYS") != "0" {
		go func() {
			ticker := time.NewTicker(1 * time.Hour)
			defer ticker.Stop()
			for range ticker.C {
				cutoff := time.Now().AddDate(0, 0, -retentionDays).UTC().Format(time.RFC3339)
				for _, o := range orgs.List() {
					if s, err := orgs.Get(o.Slug); err == nil {
						s.CleanOldChannelMessages(cutoff)
						s.CleanExpiredFileTokens()
					}
				}
				slog.Info("retention cleanup done", "retention_days", retentionDays)
			}
		}()
		slog.Info("message retention enabled", "days", retentionDays)
	}

	slog.Info("server starting", "addr", *addr)
	slog.Info("routes",
		"bot_api", "POST /bot{token}/sendMessage",
		"client_api", "GET /api/health",
		"pwa", "GET /",
		"admin", "POST /admin/bots",
	)

	srv := &http.Server{
		Addr:              *addr,
		Handler:           securityHeaders(api.RequestLogger(bot.TelegramCompat(mux))),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Graceful shutdown on SIGTERM/SIGINT
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
		sig := <-sigCh
		slog.Info("shutting down", "signal", sig.String())

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped gracefully")
}

func loadOrGenerateVAPID(pubPath, keyPath string) (string, string) {
	//nolint:gosec // G304: fixed config paths
	pubData, err1 := os.ReadFile(pubPath) // #nosec G304
	//nolint:gosec // G304: fixed config paths
	keyData, err2 := os.ReadFile(keyPath) // #nosec G304
	if err1 == nil && err2 == nil {
		pub := strings.TrimSpace(string(pubData))
		key := strings.TrimSpace(string(keyData))
		if pub != "" && key != "" {
			slog.Info("VAPID keys loaded from disk", "pub_path", pubPath)
			return pub, key
		}
	}
	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		slog.Warn("failed to generate VAPID keys, push disabled", "error", err)
		return "", ""
	}
	_ = os.WriteFile(pubPath, []byte(pub+"\n"), 0o600)
	_ = os.WriteFile(keyPath, []byte(priv+"\n"), 0o600)
	slog.Info("VAPID keys auto-generated", "pub_path", pubPath, "key_path", keyPath)
	return pub, priv
}

func loadOrGenerateSecret(path string) string {
	//nolint:gosec // G304: fixed config path from CLI flag
	data, err := os.ReadFile(path) // #nosec G304
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
	_ = os.WriteFile(path, []byte(secret+"\n"), 0o600)
	slog.Info("JWT secret generated", "path", path)
	return secret
}
