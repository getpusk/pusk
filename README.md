[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![CI](https://github.com/getpusk/pusk/actions/workflows/ci.yml/badge.svg)](https://github.com/getpusk/pusk/actions/workflows/ci.yml)
[![Bot API](https://img.shields.io/badge/Telegram_Bot_API-13_methods-2CA5E0?logo=telegram)](https://core.telegram.org/bots/api)
[![SQLite](https://img.shields.io/badge/SQLite-per_tenant-003B57?logo=sqlite)](https://www.sqlite.org)
[![Go Report Card](https://goreportcard.com/badge/github.com/getpusk/pusk)](https://goreportcard.com/report/github.com/getpusk/pusk)

🌐 [English](README.en.md)

<img src=".github/assets/landing.png" alt="Pusk — интерфейс" width="960" />

# Pusk — self-hosted алерты для ops-команд

**Pusk** — self-hosted платформа для алертов и командной координации. Webhook из любого мониторинга, ACK одной кнопкой, push на телефон. Один бинарник, без внешних зависимостей.

## Зачем это нужно?

**Проблема:** алерт сработал. Кто взял? Тишина.
- Алерты тонут в общих чатах среди обсуждений
- Нет подтверждения (ACK) — непонятно, кто разбирается
- Нет эскалации — если дежурный спит, алерт умирает
- Данные на чужих серверах — compliance не пройдёшь

**Решение — Pusk:**
- Алерты из Grafana, Zabbix, Alertmanager, Uptime Kuma — в отдельные каналы
- ACK одной кнопкой — автоматический silence в Alertmanager
- Push-уведомления на телефон даже при закрытом браузере
- Командный чат — каналы, @упоминания, файлы
- Совместим с Telegram Bot API — существующие боты работают с заменой одной строки

<img src=".github/assets/alerts.png" alt="Pusk — канал алертов" width="960" />

<img src=".github/assets/chat.png" alt="Pusk — командный чат" width="960" />

## Кому подходит

- **DevOps/SRE-команды** — алерты из мониторинга + координация инцидентов
- **Компании с compliance** — данные на своём сервере, без внешних зависимостей
- **Те, кому нужна автономность** — работает без внешних зависимостей

## Что умеет

| Возможность | Описание |
|-------------|----------|
| **Алерты** | Webhook из Alertmanager, Grafana, Zabbix, Uptime Kuma. Цветовые индикаторы, ACK, автоматический silence |
| **Push** | Web Push на телефон и десктоп (даже при закрытом браузере) |
| **Боты** | 13 методов Telegram Bot API. Inline-кнопки, webhook, long polling |
| **Каналы** | Командные каналы, @упоминания с push, reply, pin, редактирование |
| **Файлы** | Фото, видео, голосовые, документы — загрузка и просмотр |
| **Онлайн-статус** | Имена онлайн/отошёл в реальном времени, typing-индикатор |
| **Мультитенант** | Изолированные организации (отдельная SQLite на каждую) |
| **Простота** | Один бинарник (23 МБ), SQLite, ~2 МБ RAM, запуск за 1 секунду |

## Частые вопросы

<details>
<summary><b>Это ещё один мессенджер?</b></summary>
Нет. Это платформа алертов с командным чатом. Ближе к PagerDuty и Opsgenie, чем к Slack — но на вашем сервере и бесплатно.
</details>

<details>
<summary><b>Чем отличается от PagerDuty?</b></summary>
Self-hosted, бесплатный, один бинарник. Нет расписания дежурств (пока), но есть командный чат и совместимость с Telegram Bot API.
</details>

<details>
<summary><b>Нужно ставить приложение?</b></summary>
Нет. Pusk работает в браузере — открываете ссылку и пользуетесь. Можно добавить на главный экран телефона как иконку (PWA), но это необязательно. Поддерживаются Chrome, Firefox, Edge.
</details>

<details>
<summary><b>Как приходят уведомления на телефон?</b></summary>
Через Web Push — стандарт браузера, как у Slack и Discord. Приходят даже когда браузер закрыт. На Android работает отлично, на iOS с Safari 16.4+.
</details>

## Быстрый старт

### Docker (рекомендуется)

```bash
docker run -d --name pusk \
  -p 8443:8443 \
  -v pusk-data:/app/data \
  ghcr.io/getpusk/pusk:latest
```

Откройте `http://localhost:8443` — зарегистрируйтесь и начните работу.

### Первый запуск

1. Первый пользователь создаёт **организацию** — он становится администратором
2. В настройках нажмите **Пригласить** — скопируйте ссылку и отправьте коллегам
3. Коллеги переходят по ссылке, регистрируются — и сразу в команде
4. Новые участники автоматически подписываются на все каналы

> Назначьте минимум 2 админов, чтобы не зависеть от одного человека.

### Подключение мониторинга

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

#### Любая система с curl

```bash
curl -X POST 'https://your-pusk/hook/BOT-TOKEN?format=raw' \
  -H 'Content-Type: application/json' \
  -d '{"status":"down","name":"my-service"}'
```

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

## Совместимость с Telegram Bot API

Если у вас уже есть бот на Telegram Bot API — он заработает в Pusk с заменой одной строки:

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

Поддерживается 13 из 80+ методов — достаточно для алертов, уведомлений и простых ботов.

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
sqlite3 data/orgs/my-org/pusk.db ".backup backup.db"

# Полный бэкап
tar czf pusk-backup-$(date +%Y%m%d).tar.gz data/
```

## Архитектура

```
pusk (23 МБ, ~8400 строк Go, 110 тестов)
├── Bot API    — /bot/<token>/<method>  (совместим с Telegram)
├── Client API — /api/*                 (бэкенд PWA)
├── WebSocket  — /api/ws                (real-time статусы, typing)
├── Файлы      — /file/<id>             (медиа)
├── PWA        — /                      (веб-клиент)
└── SQLite     — data/orgs/*/pusk.db    (база данных)
```

## Демо

Попробуйте: [getpusk.ru](https://getpusk.ru) — кнопка «Demo», без регистрации.

## Безопасность

- JWT с 7-дневным TTL, bcrypt-хеширование паролей
- Защита владельца организации (первый админ) от удаления и понижения
- Канал #general защищён от удаления и переименования
- Rate limiting на авторизацию, регистрацию, отправку
- SSRF-защита webhook URL
- Мультитенант с полной изоляцией данных (отдельная SQLite на организацию)

## Лицензия

BSL 1.1 — Copyright (c) 2026 Volkov Pavel | DevITWay

Подробнее в [LICENSE](LICENSE).
