# Changelog

## v0.5.0 (2026-03-20)

### Features
- **Channel replies** — subscribers can write in channels (team coordination)
- **ACK/Mute/Resolved buttons** on webhook alerts
- **Webhook endpoint** — native Alertmanager, Zabbix, Grafana, Uptime Kuma support (`POST /hook/{token}?format=`)
- **Smart raw parser** — extracts `msg`/`message`/`text` from any webhook payload
- **sendMessage → channel fallback** — negative `chat_id` sends to channel (Telegram convention)
- **Invite links** — one-time codes with 24h TTL, UI button in Settings
- **Multi-tenant** — SQLite per organization, isolated data
- **Org registration** — create org with system bot + #general channel
- **Webhook Relay** — localhost bots via WebSocket, no ngrok needed
- **Landing page** — split view with live demo chat
- **PWA install prompt** with app icon
- **AI DemoBot** — Groq-powered demo bot responds in real-time

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
- 27 security issues found, 20+ fixed

### CI/CD
- CodeQL SAST analysis
- govulncheck
- Trivy filesystem + Docker image scan
- Integration tests (Bot API, webhooks, multi-tenant, auth)
- OpenSSF Scorecard
- 34 E2E Playwright tests + 32 QA Panel checks

### Integrations
- Alertmanager (`format=alertmanager`)
- Zabbix (`format=zabbix`)
- Grafana (`format=grafana`)
- Uptime Kuma (smart raw with `msg` field extraction)
- Any system with Telegram notification type (negative `chat_id`)
- Generic webhook (`format=raw`)

## v0.4.0 — v0.1.0
See git history for earlier releases.
