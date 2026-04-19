# Changelog

## v0.7.8 (2026-04-19)

### Security
- **P0**: Remove username enumeration oracle — `/api/my-orgs` no longer called from landing page, rate-limited on server
- **P1**: Fix JSON injection in admin error responses — use `jsonErr()` helper instead of raw string concatenation
- **P1**: Fix nil-pointer panic in org registration — proper error checks after store/auth/token operations
- **P1**: Fix ACK matching on long messages — check only last 80 chars for ACK/Resolved/Muted markers

### Cleanup
- Remove 142 lines of dead format functions from `webhook.go` (replaced by template engine)
- Fix `time.Tick` goroutine leak in `route.go` — use `time.NewTicker`
- Close HTTP response body in push notification error path
- Rewrite `templates_test.go` to test via template engine API

### Docs
- Replace landing screenshot with real workflow GIF (create org → login → alerts → ACK → settings)
- Add screencast recording spec for reproducible README assets

## v0.7.7 (2026-04-19)

### Fixes
- Fix org modal reset and cross-org isolation (#82)
- Push HTTPS check for secure contexts (#82)
- Clipboard copy fallback for HTTP contexts (#81)
- Push notifications in Firefox — permission flow fix (#80)
- Demo visibility, org modal reset, auth UX improvements (#79)

## v0.7.6 (2026-04-18)

### Fixes
- Multi-device message delivery reliability (#78)
- Login UX cleanup — consistent error messages (#78)

## v0.7.5 (2026-04-17)

### Fixes
- Healthcheck flag, migration versioning, metrics endpoint (#77)

## v0.7.4 (2026-04-16)

### Features
- Bot selector for channels, admin bypass fix (#76)
- Admin API documentation (#76)

## v0.7.3 (2026-04-15)

### Security
- Self-hosted security hardening (#75)

## v0.7.2 (2026-04-14)

### Fixes
- Scrub webhook URL from logs (#68)
- Return 500 on save failure (#68)

## v0.7.1 (2026-04-10)

### Features
- Sign Docker images with cosign (keyless Sigstore) (#65)
- Exclude demo code from release binary via build tags (#62)

### CI
- Bump Go 1.26.0 → 1.26.2 (security fixes) (#64)
- Patch Alpine CVEs (#63)

## v0.7.0 (2026-04-03)

### Features
- **Cross-org push navigation** — click a push notification to jump directly into the alert channel, even from a different org
- **Unicode usernames** — Cyrillic and other non-Latin characters in usernames
- **Bot tag in sidebar** — bot name shown next to channel when multiple bots exist
- **Push vibration** — haptic feedback on incoming push notifications (Android)
- **Deploy pipeline** — lint + test gate before every deployment

### Fixes
- Push notifications now include org parameter for reliable cross-org routing
- localStorage split-brain resolved — all keys use consistent `pusk_` prefix
- Service Worker cache bumped (v75) to ensure clients pick up JS updates
- golangci-lint v2 config migration (`exclude-dirs` → `linters.exclusions.paths`)

### CI/CD
- Lint + test gate enforced in deploy script
- Playwright bumped to 1.59.0
- CI validates push notification properties (vibrate, requireInteraction)

---

## v0.6.1 (2026-03-28)

### Features
- **Multi-use invite links** — 7-day TTL, up to 50 uses, revokable
- **Online/away dots** on message avatars
- **Smart invite form** — auto-detects whether user needs login or registration
- **Server-side org discovery** — find all orgs where your account exists
- **Push deep links** — notifications link directly to channels and chats
- **Org stats API** — online count, channel stats, Prometheus gauges
- **Owner protection** — org owner cannot be deleted or demoted
- **Channel rename** — admins can rename channels (double-click header or long press on mobile)
- **Compact online status** on mobile (2● 1○)
- **localStorage migration** — seamless upgrade from old key format to `pusk_` prefix

### Fixes
- Push subscriptions stored per-device (max 5), refresh on app open
- Safe Service Worker update strategy — no auto-reload, user-triggered
- Org isolation: mention lists, read receipts, FAB permissions scoped to current org
- Token expiry shows login form with saved org (not guest fallback)
- Push delivery logging, stacking, requireInteraction for alerts
- Footer shows per-org online count instead of global
- Invite link shows registration even when logged into another org

### Security
- Cross-org isolation tests (mentions, read receipts, file access)
- Token cleanup on org switch
- Push disabled in demo org

### CI/CD
- Full release pipeline: Alpine + RED OS + Astra Linux images
- Cosign keyless signing (binary + SBOM)
- SBOM generation (SPDX-JSON)
- Trivy image scanning (HIGH/CRITICAL)
- Security regression tests
- golangci-lint with errcheck + staticcheck + nilerr

---

## v0.6.0 (2026-03-25)

### Features
- **ES modules** — frontend split from monolith into 9 JS modules
- **Service Worker** — app shell caching, offline indicator, update notification
- **Infinite scroll** for message history
- **File upload** — photos, videos, documents in channels (thumbnails 320px)
- **@mentions** with autocomplete and push notifications
- **Read receipts** — see who read the last message in channels
- **Away status** — idle detection + typing indicator
- **Alert filters** — filter alerts by status, precise timestamps
- **Elapsed time badges** on firing alerts
- **Reply to messages** in channels with quote display
- **Pin messages** — shows who pinned, unpin button
- **Context menu** — edit, delete, copy, reply on messages
- **Onboarding wizard** — 3-step setup for new organizations
- **Org switcher** with unread counters
- **getUpdates long polling** — drop-in migration path for Telegram polling bots
- **Telegram-native URLs** — `/botTOKEN/method` routing for full Bot API compatibility
- **Custom Go templates** for webhook message formatting
- **Webhook debounce** — deduplicates Alertmanager bursts
- **Prometheus /metrics** — HTTP, WebSocket, message counters
- **Structured logging** (slog) with request logger middleware
- **File tokens** + message retention + storage quota
- **ACK → Alertmanager Silence** API integration
- **/test-push** button in Settings
- **Clickable URLs** and /commands in messages
- **i18n** — full bilingual UI (Russian / English)
- **Double-back-to-exit** on mobile PWA
- **Loading skeleton** for initial app load

### Security
- JWT revocation on password change and user deletion
- XSS escaping hardening, PIN lockout (5 attempts)
- SSRF webhook validation (DNS resolve + IsPrivate check)
- Per-org file directories
- Rate limiting on send/upload endpoints
- Content-Security-Policy headers
- Admin auth on /admin/ endpoints

### CI/CD
- Dependabot for Go modules, GitHub Actions, npm, Docker
- All GitHub Actions pinned by commit SHA
- CodeQL scheduled analysis
- OpenSSF Scorecard
- Integration tests: Bot API, webhooks, multi-tenant, auth flows
- Frontend integrity checks in CI

---

## v0.5.0 (2026-03-20)

### Features
- **Channel replies** — subscribers can write in channels (team coordination)
- **ACK/Mute/Resolved buttons** on webhook alerts
- **Webhook endpoint** — native Alertmanager, Zabbix, Grafana, Uptime Kuma support
- **Smart raw parser** — extracts msg/message/text from any webhook payload
- **sendMessage → channel fallback** — negative chat_id sends to channel (Telegram convention)
- **Invite links** — one-time codes with 24h TTL
- **Multi-tenant** — SQLite per organization, isolated data
- **Org registration** — create org with system bot + #general channel
- **Webhook Relay** — localhost bots via WebSocket, no ngrok needed
- **Landing page** — split view with live demo chat
- **PWA install prompt** with app icon

### Security
- bcrypt password hashing
- Random JWT secret (persisted)
- Rate limiting (20 auth/min per IP)
- IDOR protection on all chat endpoints
- SSRF: URL parse + DNS resolve + IsPrivate/IsLoopback
- XSS: markdown links http/https only
- File serving requires JWT
- WebSocket origin validation
- Graceful shutdown (SIGTERM → 5s grace)

### Integrations
- Alertmanager, Zabbix, Grafana, Uptime Kuma
- Any system with Telegram notification type (negative chat_id)
- Generic webhook (format=raw)

### CI/CD
- CodeQL SAST analysis
- govulncheck
- Trivy filesystem + Docker image scan
- Integration tests (Bot API, webhooks, multi-tenant, auth)
- OpenSSF Scorecard
- 34 E2E Playwright tests

---

## v0.4.0 — v0.1.0

See [git history](https://github.com/getpusk/pusk/commits/main) for earlier releases.
