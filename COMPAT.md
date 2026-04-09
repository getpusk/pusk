# Pusk Bot API compatibility with Telegram Bot API

This document describes which Telegram Bot API methods are supported by Pusk.

**Legend:** Full = complete implementation, Partial = basic support with limitations, — = not implemented

## Supported Methods

### Getting Updates

| Method | Status | Notes |
|--------|--------|-------|
| getUpdates | Full | Long polling, monotonic update_id, offset support |
| setWebhook | Full | URL + secret_token validation |
| deleteWebhook | Full | |
| getWebhookInfo | Full | |

### Available Methods

| Method | Status | Notes |
|--------|--------|-------|
| getMe | Full | Returns bot id, name, username |
| sendMessage | Full | parse_mode: HTML, Markdown. reply_to_message_id |
| editMessageText | Full | Real-time via WebSocket |
| deleteMessage | Full | |
| answerCallbackQuery | Full | |
| sendPhoto | Full | Thumbnails 320px, file storage |
| sendDocument | Full | |
| sendVoice | Full | |
| sendVideo | Full | |

### Not Implemented

These Telegram Bot API methods are **not supported**:

| Category | Methods | Priority |
|----------|---------|----------|
| Messages | forwardMessage, copyMessage, sendAnimation, sendAudio, sendVideoNote, sendMediaGroup, sendLocation, sendVenue, sendContact, sendPoll, sendDice | Low |
| Editing | editMessageCaption, editMessageMedia, editMessageReplyMarkup, stopPoll | Low |
| Stickers | All sticker methods | — |
| Inline mode | answerInlineQuery, all inline methods | — |
| Payments | All payment methods | — |
| Games | All game methods | — |
| Chat management | getChat, getChatAdministrators, getChatMemberCount, getChatMember, banChatMember, unbanChatMember, leaveChat, setChatTitle, setChatDescription, pinChatMessage, unpinChatMessage | Medium |
| Bot commands | setMyCommands, getMyCommands, deleteMyCommands | Low |
| User | getUserProfilePhotos | Low |
| Files | getFile (direct download via /file/{id} instead) | Partial |
| Callbacks | setCallbackAnswer with show_alert | Low |

## Pusk-Specific Extensions

These methods are **not part of Telegram Bot API** but are available in Pusk:

| Method | Description |
|--------|-------------|
| relay | WebSocket relay for real-time bot communication |
| createChannel | Create a new channel in the org |
| sendChannel | Send message to a channel by name |

## Webhook Format

### Incoming Webhooks (Alertmanager, Zabbix, Grafana)

| Source | Endpoint | Status |
|--------|----------|--------|
| Alertmanager | POST /hook/{bot_token} | Full |
| Zabbix | POST /hook/{bot_token} | Full |
| Grafana | POST /hook/{bot_token} | Full |
| Custom JSON | POST /hook/{bot_token} | Full |
| Go templates | Custom formatting per bot | Full |

### Webhook Dispatch

| Feature | Status | Notes |
|---------|--------|-------|
| secret_token validation | Full | X-Telegram-Bot-Api-Secret-Token header |
| Retry on failure | — | Single attempt only |
| Update delivery guarantee | Partial | At-most-once via webhook, at-least-once via getUpdates |

## Wire Format Compatibility

| Feature | Status | Notes |
|---------|--------|-------|
| Bot API URL format `/bot{token}/{method}` | Full | |
| JSON request body | Full | |
| Multipart form-data (file upload) | Full | sendPhoto, sendDocument, sendVoice, sendVideo |
| `application/x-www-form-urlencoded` | Full | |
| Response format `{"ok": true, "result": ...}` | Full | |
| Error format `{"ok": false, "error_code": N}` | Full | |

## Known Differences from Telegram

| Behavior | Telegram | Pusk |
|----------|----------|------|
| update_id | Sequential per bot | Monotonic (UnixMilli), shared across bots per org |
| File storage | Telegram CDN | Local filesystem |
| File URLs | `https://api.telegram.org/file/bot{token}/{path}` | `https://{host}/file/{id}?token={jwt}` |
| Chat IDs | Negative for groups | Positive integers (channel IDs) |
| User IDs | Telegram user IDs | Local sequential IDs per org |
| Bot creation | @BotFather | Admin API (`POST /admin/bots`) |
| Rate limits | 30 msg/sec per bot | Configurable per instance |
