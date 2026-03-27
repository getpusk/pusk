[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![CI](https://github.com/getpusk/pusk/actions/workflows/ci.yml/badge.svg)](https://github.com/getpusk/pusk/actions/workflows/ci.yml)
[![Bot API](https://img.shields.io/badge/Telegram_Bot_API-13_methods-2CA5E0?logo=telegram)](https://core.telegram.org/bots/api)
[![SQLite](https://img.shields.io/badge/SQLite-per_tenant-003B57?logo=sqlite)](https://www.sqlite.org)
[![Go Report Card](https://goreportcard.com/badge/github.com/getpusk/pusk)](https://goreportcard.com/report/github.com/getpusk/pusk)

🌐 [Русский](README.md)


<img src=".github/assets/landing.png" alt="Pusk — интерфейс" width="960" />

# Pusk — свой мессенджер для алертов и команды

**Pusk** — это self-hosted платформа для оповещений и командного общения. Один бинарник, встроенный веб-клиент, без внешних зависимостей.

## Зачем это нужно?

**Проблема:** ваш мониторинг (Grafana, Zabbix, Uptime Kuma, Alertmanager) шлёт алерты в Telegram. Но:
- Telegram могут замедлить или заблокировать
- Алерты тонут среди личных чатов
- Нет контроля — данные на чужих серверах
- Нужен бот? Придётся зависеть от Telegram API

**Решение — Pusk:**
- Работает на вашем сервере — никаких внешних зависимостей
- Алерты приходят в отдельные каналы с push-уведомлениями
- Подтверждение алертов (ACK) одной кнопкой прямо в чате
- Команда общается здесь же — каналы, @упоминания, загрузка файлов
- Совместим с Telegram Bot API — существующие боты работают с заменой одной строки

<img src=".github/assets/alerts.png" alt="Pusk — канал алертов" width="960" />

## Кому подходит

- **DevOps/SRE-команды** — алерты из мониторинга + координация инцидентов
- **Малые компании** — корпоративный мессенджер без Slack/Teams
- **Те, кому важен контроль** — данные на своём сервере, никаких третьих сторон

## Что умеет

| Возможность | Описание |
|-------------|----------|
| **Алерты** | Webhook из Alertmanager, Grafana, Zabbix, Uptime Kuma. Цветовые индикаторы, ACK, автоматический silence |
| **Каналы** | Командные каналы для общения. @упоминания с push-уведомлениями |
| **Push** | Web Push уведомления на телефон и десктоп (даже при закрытом браузере) |
| **Боты** | 13 методов Telegram Bot API. Inline-кнопки, webhook, long polling |
| **Файлы** | Фото, видео, голосовые, документы — загрузка и просмотр в чате |
| **Мультитенант** | Отдельные организации с изоляцией данных |
| **Простота** | Один бинарник (22 МБ), SQLite, ~2 МБ RAM, запуск за 1 секунду |

## Быстрый старт

### Docker (рекомендуется)

```bash
docker run -d --name pusk \
  -p 8443:8443 \
  -v pusk-data:/app/data \
  ghcr.io/getpusk/pusk:latest
```

Open `http://localhost:8443` — register and get started.

### First Run

1. First user creates an **organization** — becomes admin
2. Go to Settings → **Invite** — copy the link and share with your team
3. Teammates follow the link, register — and they are in

> Assign at least 2 admins so you do not depend on a single person.

### Из исходников

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

## Подключение мониторинга

### Alertmanager

```yaml
receivers:
  - name: pusk
    webhook_configs:
      - url: 'https://your-pusk/hook/BOT-TOKEN?format=alertmanager'
```

### Grafana

Alerting → Contact points → New → Type: **Webhook**
URL: `https://your-pusk/hook/BOT-TOKEN?format=grafana`

### Zabbix

Administration → Media types → Create: **Webhook**
URL: `https://your-pusk/hook/BOT-TOKEN?format=zabbix`

### Uptime Kuma

Notifications → Add → Type: **Webhook**
URL: `https://your-pusk/hook/BOT-TOKEN?format=raw&channel=alerts`

### Любая система с curl

```bash
curl -X POST https://your-pusk/hook/BOT-TOKEN?format=raw \
  -H 'Content-Type: application/json' \
  -d '{"status":"down","name":"my-service"}'
```

## Миграция с Telegram

Если ваш бот использует `sendMessage`, `editMessageText`, inline-кнопки, webhook — достаточно поменять одну строку:

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

Поддерживается 13 из 80+ методов Telegram Bot API — достаточно для алертов, уведомлений и простых ботов.

## Установка на VPS

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

### Обратный прокси (Caddy)

```
pusk.example.com {
    reverse_proxy localhost:8443
}
```

### Обратный прокси (Nginx)

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

## Настройка

| Переменная | По умолчанию | Описание |
|-----------|-------------|----------|
| `PUSK_ADDR` | `:8443` | Адрес сервера |
| `PUSK_ADMIN_TOKEN` | — | Токен для Admin API |
| `PUSK_DEMO` | — | `1` — включить демо-режим |
| `PUSK_MSG_RETENTION_DAYS` | `30` | Автоудаление сообщений старше N дней. `0` — не удалять |
| `PUSK_FILE_QUOTA_MB` | `1024` | Лимит хранилища файлов на организацию (МБ) |
| `PUSK_WEBHOOK_DEBOUNCE` | `10s` | Дедупликация одинаковых webhook. `0` — отключить |
| `PUSK_ALERTMANAGER_URL` | — | URL Alertmanager для авто-silence при ACK |
| `VAPID_PUBLIC_KEY` | — | VAPID ключ для Web Push |
| `VAPID_PRIVATE_KEY` | — | Приватный ключ VAPID |
| `VAPID_EMAIL` | — | Email для push-сервиса |

## Резервное копирование

Все данные в папке `data/`:

```bash
# Горячий бэкап
sqlite3 data/orgs/default/pusk.db ".backup backup.db"

# Полный бэкап
tar czf pusk-backup-$(date +%Y%m%d).tar.gz data/
```

## Архитектура

```
pusk (22 МБ)
├── Bot API    — /bot/<token>/<method>  (совместим с Telegram)
├── Client API — /api/*                 (бэкенд PWA)
├── WebSocket  — /api/ws                (real-time)
├── Файлы      — /file/<id>             (медиа)
├── PWA        — /                      (веб-клиент)
└── SQLite     — data/orgs/*/pusk.db    (база данных)
```

## Демо

Попробуйте: [getpusk.ru](https://getpusk.ru) — кнопка «Demo», без регистрации.

## Безопасность

- CSP-заголовки, bcrypt-хеширование, JWT с 7-дневным TTL
- Rate limiting на авторизацию, регистрацию, отправку
- SSRF-защита webhook URL
- Мультитенант с изоляцией данных (отдельная SQLite на организацию)

## Лицензия

BSL 1.1 — Copyright (c) 2026 Volkov Pavel | DevITWay

Подробнее в [LICENSE](LICENSE).
