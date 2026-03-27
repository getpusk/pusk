[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![CI](https://github.com/getpusk/pusk/actions/workflows/ci.yml/badge.svg)](https://github.com/getpusk/pusk/actions/workflows/ci.yml)
[![Bot API](https://img.shields.io/badge/Telegram_Bot_API-13_methods-2CA5E0?logo=telegram)](https://core.telegram.org/bots/api)
[![SQLite](https://img.shields.io/badge/SQLite-per_tenant-003B57?logo=sqlite)](https://www.sqlite.org)
[![Go Report Card](https://goreportcard.com/badge/github.com/getpusk/pusk)](https://goreportcard.com/report/github.com/getpusk/pusk)

🌐 [Русский](README.md)

<img src=".github/assets/landing.png" alt="Pusk — interface" width="960" />

# Pusk — self-hosted alerts for ops teams

**Pusk** — self-hosted alert platform with team chat. Webhooks from any monitoring, one-click ACK, push to phone. Single binary, zero dependencies.

## Why?

**Problem:** alert fires. Who picked it up? Silence.
- Alerts drown in Telegram among memes and personal chats
- No acknowledgment (ACK) — unclear who is handling it
- No escalation — if on-call is asleep, the alert dies
- Data on third-party servers — compliance fails
- Telegram can be throttled or blocked

**Solution — Pusk:**
- Alerts from Grafana, Zabbix, Alertmanager, Uptime Kuma — into dedicated channels
- One-click ACK — automatic silence in Alertmanager
- Push notifications to phone even with browser closed
- Team chat built in — channels, @mentions, file uploads
- Telegram Bot API compatible — existing bots work with a one-line change

<img src=".github/assets/alerts.png" alt="Pusk — alert channel" width="960" />

## Who is it for

- **DevOps/SRE teams** — monitoring alerts + incident coordination
- **Companies with compliance needs** — data on your server, air-gapped, regulated environments
- **Anyone who needs control** — no third parties, works when Telegram is blocked

## Features

| Feature | Description |
|---------|-------------|
| **Alerts** | Webhooks from Alertmanager, Grafana, Zabbix, Uptime Kuma. Color indicators, ACK, automatic silence |
| **Push** | Web Push notifications to phone and desktop (even with browser closed) |
| **Bots** | 13 Telegram Bot API methods. Inline buttons, webhook, long polling |
| **Channels** | Team channels for communication. @mentions with push notifications |
| **Files** | Photos, videos, voice messages, documents — upload and view in chat |
| **Multi-tenant** | Separate organizations with data isolation |
| **Simple** | Single binary (22 MB), SQLite, ~2 MB RAM, 1-second startup |

## FAQ

<details>
<summary><b>Is this yet another messenger?</b></summary>
No. It is an alert platform with team chat. Closer to PagerDuty and Opsgenie than to Slack — but self-hosted and free.
</details>

<details>
<summary><b>Why not just use Telegram?</b></summary>
Telegram is a chat app. Pusk is for alerts. ACK, automatic Alertmanager silence, push even when Telegram is blocked. Team chat is a bonus, not the goal.
</details>

<details>
<summary><b>Do I need to install an app?</b></summary>
No. Pusk works in your browser — just open the link. You can add it to your home screen as a PWA icon, but it is optional. Chrome, Firefox, Edge supported.
</details>

<details>
<summary><b>How do phone notifications work?</b></summary>
Via Web Push — a browser standard, like Slack and Discord. Works even when the browser is closed. Great on Android, iOS with Safari 16.4+.
</details>

## Quick start

### Docker (recommended)

```bash
docker run -d --name pusk \
  -p 8443:8443 \
  -v pusk-data:/app/data \
  ghcr.io/getpusk/pusk:latest
```

Open `http://localhost:8443` — register and get started.

### First run

1. First user creates an **organization** — becomes admin
2. Go to Settings → **Invite** — copy the link and share with your team
3. Teammates follow the link, register — and they are in

> Assign at least 2 admins so you do not depend on a single person.

### Connect monitoring

#### Alertmanager

```yaml
receivers:
  - name: pusk
    webhook_configs:
      - url: 'https://your-pusk/hook/BOT-TOKEN?format=alertmanager'
```

#### Grafana

Alerting → Contact points → New → Type: **Webhook**
URL: `https://your-pusk/hook/BOT-TOKEN?format=grafana`

#### Zabbix

Administration → Media types → Create: **Webhook**
URL: `https://your-pusk/hook/BOT-TOKEN?format=zabbix`

#### Uptime Kuma

Notifications → Add → Type: **Webhook**
URL: `https://your-pusk/hook/BOT-TOKEN?format=raw&channel=alerts`

#### Any system with curl

```bash
curl -X POST https://your-pusk/hook/BOT-TOKEN?format=raw \
  -H 'Content-Type: application/json' \
  -d '{"status":"down","name":"my-service"}'
```

### From source

```bash
git clone https://github.com/getpusk/pusk.git
cd pusk
go build -o pusk ./cmd/pusk/
./pusk
```

### Docker Compose

```yaml
version: '3'
services:
  pusk:
    image: ghcr.io/getpusk/pusk:latest
    ports:
      - "8443:8443"
    volumes:
      - ./data:/app/data
    environment:
      - PUSK_ADMIN_TOKEN=your-secret
    restart: unless-stopped
```

## Migrating from Telegram

If your bot uses `sendMessage`, `editMessageText`, inline buttons, webhook — just change one line:

```python
# Python (aiogram)
bot = Bot(token="TOKEN", base_url="https://your-pusk:8443/bot")

# Python (python-telegram-bot)
app = Application.builder().token(TOKEN).base_url("https://your-pusk:8443/bot").build()
```

```javascript
// Node.js (Telegraf)
bot.telegram.options.apiRoot = "https://your-pusk:8443";
```

Supports 13 of 80+ Telegram Bot API methods — enough for alerts, notifications and simple bots.

## VPS installation

### Systemd

```bash
sudo tee /etc/systemd/system/pusk.service << EOF
[Unit]
Description=Pusk
After=network.target

[Service]
User=pusk
WorkingDirectory=/opt/pusk
ExecStart=/opt/pusk/pusk
Restart=always
Environment=PUSK_ADDR=:8443
Environment=PUSK_ADMIN_TOKEN=your-secret

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl enable --now pusk
```

### Reverse proxy (Caddy)

```
pusk.example.com {
    reverse_proxy localhost:8443
}
```

### Reverse proxy (Nginx)

```nginx
server {
    listen 443 ssl;
    server_name pusk.example.com;

    location / {
        proxy_pass http://127.0.0.1:8443;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PUSK_ADDR` | `:8443` | Server address |
| `PUSK_ADMIN_TOKEN` | — | Admin API token |
| `PUSK_DEMO` | — | `1` — enable demo mode |
| `PUSK_MSG_RETENTION_DAYS` | `30` | Auto-delete messages older than N days. `0` — keep all |
| `PUSK_FILE_QUOTA_MB` | `1024` | File storage limit per organization (MB) |
| `PUSK_WEBHOOK_DEBOUNCE` | `10s` | Deduplicate identical webhooks. `0` — disable |
| `PUSK_ALERTMANAGER_URL` | — | Alertmanager URL for auto-silence on ACK |
| `VAPID_PUBLIC_KEY` | — | VAPID key for Web Push |
| `VAPID_PRIVATE_KEY` | — | VAPID private key |
| `VAPID_EMAIL` | — | Email for push service |

## Backup

All data is in the `data/` directory:

```bash
# Hot backup
sqlite3 data/orgs/default/pusk.db ".backup backup.db"

# Full backup
tar czf pusk-backup-$(date +%Y%m%d).tar.gz data/
```

## Architecture

```
pusk (22 MB)
├── Bot API    — /bot/<token>/<method>  (Telegram compatible)
├── Client API — /api/*                 (PWA backend)
├── WebSocket  — /api/ws                (real-time)
├── Files      — /file/<id>             (media)
├── PWA        — /                      (web client)
└── SQLite     — data/orgs/*/pusk.db    (database)
```

## Demo

Try it: [getpusk.ru](https://getpusk.ru) — click "Demo", no registration.

## Security

- CSP headers, bcrypt hashing, JWT with 7-day TTL
- Rate limiting on auth, registration, messaging
- SSRF protection for webhook URLs
- Multi-tenant with data isolation (separate SQLite per organization)

## License

BSL 1.1 — Copyright (c) 2026 Volkov Pavel | DevITWay

See [LICENSE](LICENSE) for details.
