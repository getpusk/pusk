[![License: MIT](https://img.shields.io/badge/License-BSL--1.1-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](https://go.dev)

> **Self-hosted Telegram-compatible bot platform.**
>
> One binary. Inline keyboards. Zero config.

**16 MB** binary | **< 50 MB** RAM | **1s** startup | **SQLite** storage

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

## Cloud

Managed version at [getpusk.com](https://getpusk.com)

## License

BSL 1.1 - Copyright (c) 2026 Volkov Pavel | DevITWay

See [LICENSE](LICENSE) for details.
