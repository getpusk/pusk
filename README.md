[![License: BSL-1.1](https://img.shields.io/badge/License-BSL--1.1-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](https://go.dev)

<video src="https://github.com/getpusk/pusk/raw/main/.github/assets/demo.webm" width="960" autoplay loop muted></video>

> **Drop-in Telegram Bot API replacement. Your bots keep working — on your server.**
>
> Alerts, coordination, inline keyboards. One binary, zero config.

**22 MB** binary | **12 MB** RAM | **1s** startup | **SQLite** storage

## Features

- **Telegram Bot API compatible** — sendMessage, editMessageText, deleteMessage, answerCallbackQuery, sendPhoto/Video/Voice/Document, setWebhook
- **InlineKeyboardMarkup** — interactive buttons with callback data
- **PWA client** — dark theme, mobile-ready, WebSocket push
- **File handling** — upload/download photos, voice, video, documents
- **SQLite** — zero-config database, no PostgreSQL/MySQL needed
- **Single binary** — `go build` and run, or use Docker

## Quick Start

```bash
git clone https://github.com/getpusk/pusk.git
cd pusk
go build -o pusk ./cmd/pusk/
./pusk
# Server running at :8443
```

## Usage

### 1. Register a bot

```bash
curl -X POST http://localhost:8443/admin/bots \
  -H "Content-Type: application/json" \
  -d '{"token":"my-secret-token","name":"MyBot"}'
```

### 2. Set webhook

```bash
curl -X POST http://localhost:8443/bot/my-secret-token/setWebhook \
  -H "Content-Type: application/json" \
  -d '{"url":"http://my-bot-server:3000/webhook"}'
```

### 3. Send message with buttons

```bash
curl -X POST http://localhost:8443/bot/my-secret-token/sendMessage \
  -H "Content-Type: application/json" \
  -d '{"chat_id":1,"text":"Choose:","reply_markup":{"inline_keyboard":[[{"text":"Status","callback_data":"status"},{"text":"Restart","callback_data":"restart"}]]}}'
```

### 4. Open PWA

Navigate to `http://localhost:8443` — register, pick a bot, chat.

## Bot API Compatibility

| Telegram Method | Pusk | Notes |
|----------------|------|-------|
| sendMessage | Yes | + InlineKeyboardMarkup |
| editMessageText | Yes | + update keyboard |
| deleteMessage | Yes | |
| answerCallbackQuery | Yes | |
| sendPhoto | Yes | multipart upload |
| sendVideo | Yes | multipart upload |
| sendVoice | Yes | multipart upload |
| sendDocument | Yes | multipart upload |
| setWebhook | Yes | |
| getMe | Yes | |

## Configuration

| Env Variable | Default | Description |
|-------------|---------|-------------|
| PUSK_ADDR | :8443 | Listen address |
| PUSK_DB | data/pusk.db | SQLite database path |
| PUSK_ADMIN_TOKEN | _(empty)_ | Admin API auth token |

## Architecture

```
pusk (16 MB binary)
+-- Bot API (/bot/<token>/<method>)  <- Telegram-compatible
+-- Client API (/api/*)              <- PWA backend
+-- WebSocket (/api/ws)              <- real-time push
+-- File server (/file/<id>)         <- media files
+-- PWA (/)                          <- built-in web client
+-- SQLite (data/pusk.db)            <- zero-config storage
```

## Migrating from Telegram

Replace the base URL in your bot:

```python
# Before (Telegram)
API_URL = "https://api.telegram.org"

# After (Pusk)
API_URL = "https://your-server:8443"
```

The JSON format for sendMessage, InlineKeyboardMarkup, CallbackQuery is identical.

## Migration from Telegram

### Python (aiogram)
```diff
- bot = Bot(token="YOUR_TOKEN")
+ bot = Bot(token="YOUR_TOKEN", base_url="https://your-pusk:8443/bot")
```

### Python (python-telegram-bot)
```diff
  app = Application.builder().token(TOKEN)
+     .base_url("https://your-pusk:8443/bot")
      .build()
```

### Node.js (Telegraf)
```diff
  const bot = new Telegraf(TOKEN);
+ bot.telegram.options.apiRoot = "https://your-pusk:8443";
```

See [examples/](examples/) for complete working bots.

## Live Demo

Try it: [getpusk.ru](https://getpusk.ru) — click "Demo", no registration needed.

## Integrations

### Uptime Kuma
1. Notifications → Add → Type: **Webhook**
2. URL: `https://your-pusk/hook/BOT-TOKEN?format=raw&channel=alerts`
3. Method: POST, Content-Type: application/json

### Alertmanager
```yaml
receivers:
  - name: pusk
    webhook_configs:
      - url: 'https://your-pusk/hook/BOT-TOKEN?format=alertmanager'
```

### Zabbix
1. Administration → Media types → Create: **Webhook**
2. URL: `https://your-pusk/hook/BOT-TOKEN?format=zabbix`
3. Parameters: `{ALERT.SUBJECT}`, `{ALERT.MESSAGE}`, `{EVENT.SEVERITY}`

### Grafana
1. Alerting → Contact points → New → Type: **Webhook**
2. URL: `https://your-pusk/hook/BOT-TOKEN?format=grafana`

### Any system with Telegram support
If the system has a built-in "Telegram" notification type:
1. Bot Token: your Pusk bot token
2. Chat ID: use **negative channel ID** (e.g., `-2` for channel with ID 2)
3. API URL: `https://your-pusk` (if the system supports custom base URL)

### Generic webhook
```bash
curl -X POST https://your-pusk/hook/BOT-TOKEN?format=raw \
  -H 'Content-Type: application/json' \
  -d '{"status":"down","name":"my-service"}'
```

## License

BSL 1.1 - Copyright (c) 2026 Volkov Pavel | DevITWay

See [LICENSE](LICENSE) for details.
