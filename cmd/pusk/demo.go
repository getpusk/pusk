// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package main

import (
	"log/slog"

	"github.com/pusk-platform/pusk/internal/store"
)

type demoMsg struct {
	sender string
	text   string
	markup string
}

func initDemo(db *store.Store) {
	// ── Bots ──
	demoBot, err := db.BotByToken("demo-bot-token")
	if err != nil {
		demoBot, err = db.CreateBot("demo-bot-token", "DemoBot")
		if err != nil {
			return
		}
		slog.Info("demo bot created", "bot", "DemoBot")
	}

	monBot, err := db.BotByToken("monitor-bot-token")
	if err != nil {
		monBot, err = db.CreateBot("monitor-bot-token", "MonitorBot")
		if err != nil {
			return
		}
		slog.Info("demo bot created", "bot", "MonitorBot")
	}

	// ── Users ──
	guest, err := db.AuthUser("guest", "guest")
	if err != nil {
		guest, err = db.CreateUser("guest", "guest", "Guest")
		if err != nil {
			return
		}
		slog.Info("demo user created", "user", "guest")
	}

	// Test users (admin uses ADMIN_TOKEN, these are regular users for demo)
	for _, u := range []struct{ name, pin, display string }{
		{"operator1", "1234", "Operator 1"},
		{"operator2", "1234", "Operator 2"},
		{"operator3", "1234", "Operator 3"},
	} {
		if _, err := db.AuthUser(u.name, u.pin); err != nil {
			db.CreateUser(u.name, u.pin, u.display)
			slog.Info("demo user created", "user", u.name)
		}
	}

	// ── DemoBot chat ──
	demoChat, err := db.GetOrCreateChat(guest.ID, demoBot.ID)
	if err != nil {
		return
	}
	msgs, _ := db.ChatMessages(demoChat.ID, 1)
	if len(msgs) == 0 {
		for _, m := range demoBotMessages {
			db.SaveMessage(demoChat.ID, m.sender, m.text, m.markup, "", "")
		}
		slog.Info("demo chat seeded", "bot", "DemoBot")
	}

	// ── MonitorBot chat ──
	monChat, err := db.GetOrCreateChat(guest.ID, monBot.ID)
	if err != nil {
		return
	}
	msgs2, _ := db.ChatMessages(monChat.ID, 1)
	if len(msgs2) == 0 {
		for _, m := range monitorBotMessages {
			db.SaveMessage(monChat.ID, m.sender, m.text, m.markup, "", "")
		}
		slog.Info("demo chat seeded", "bot", "MonitorBot")
	}

	// ── Channels ──
	seedChannel(db, demoBot, guest, "general", "Team chat", generalChanMessages)
	seedChannel(db, demoBot, guest, "updates", "Pusk release notes", updatesChanMessages)
	seedChannelWithMarkup(db, monBot, guest, "alerts", "Monitoring alerts", alertsChanMsgs)
	seedChannel(db, monBot, guest, "deploys", "Deploy notifications", deploysChanMessages)
}

type chanMsg struct {
	text   string
	markup string
}

func seedChannelWithMarkup(db *store.Store, bot *store.Bot, guest *store.User, name, desc string, messages []chanMsg) {
	_, err := db.ChannelByName(bot.ID, name)
	if err != nil {
		ch, err := db.CreateChannel(bot.ID, name, desc)
		if err != nil {
			return
		}
		db.Subscribe(ch.ID, guest.ID)
		for _, m := range messages {
			db.SaveChannelMessage(ch.ID, m.text, m.markup, "", "")
		}
		slog.Info("demo channel seeded", "channel", name, "messages", len(messages))
	}
}

func seedChannel(db *store.Store, bot *store.Bot, guest *store.User, name, desc string, messages []string) {
	_, err := db.ChannelByName(bot.ID, name)
	if err != nil {
		ch, err := db.CreateChannel(bot.ID, name, desc)
		if err != nil {
			return
		}
		db.Subscribe(ch.ID, guest.ID)
		for _, text := range messages {
			db.SaveChannelMessage(ch.ID, text, "", "", "")
		}
		slog.Info("demo channel seeded", "channel", name, "messages", len(messages))
	}
}

// ── Demo content ──

var demoBotMessages = []demoMsg{
	{sender: "bot", text: "Привет! Я **DemoBot** — демонстрационный бот платформы Pusk.\n\nPusk — self-hosted замена Telegram Bot API. Ваши боты, ваш сервер, ваши данные.\n\nВыберите тему:", markup: `{"inline_keyboard":[[{"text":"Что умеет Pusk?","callback_data":"features"},{"text":"Как подключить бота","callback_data":"connect"}],[{"text":"API документация","callback_data":"docs"},{"text":"GitHub","callback_data":"github"}]]}`},
	{sender: "user", text: "Что умеет Pusk?"},
	{sender: "bot", text: "**Возможности Pusk:**\n\n• 21 endpoint, совместимый с Telegram Bot API\n• Каналы с подпиской и рассылкой\n• Inline-кнопки и callback\n• Отправка файлов: фото, видео, голос, документы\n• Web Push уведомления (VAPID)\n• Markdown в сообщениях\n• PWA клиент из коробки\n\nВсё в одном бинарнике **11 MB**, потребление RAM — **3 MB**.", markup: `{"inline_keyboard":[[{"text":"Как подключить бота","callback_data":"connect"}]]}`},
	{sender: "user", text: "Как подключить своего бота?"},
	{sender: "bot", text: "Миграция с Telegram — **одна строка**:\n\n```python\n# Было (Telegram)\nbot = Bot(token=\"123:ABC\",\n  base_url=\"https://api.telegram.org\")\n\n# Стало (Pusk)\nbot = Bot(token=\"my-bot-token\",\n  base_url=\"https://getpusk.ru\")\n```\n\nРаботает с aiogram, python-telegram-bot, telegraf и любым HTTP-клиентом.", markup: `{"inline_keyboard":[[{"text":"Попробовать API","callback_data":"try_api"}]]}`},
	{sender: "user", text: "Круто, а curl примеры есть?"},
	{sender: "bot", text: "Конечно! Отправка сообщения через curl:\n\n```bash\ncurl -X POST https://getpusk.ru/botMY-TOKEN/sendMessage \\\n  -H 'Content-Type: application/json' \\\n  -d '{\"chat_id\": 1, \"text\": \"Hello from Pusk!\"}'\n```\n\nВсе методы Telegram Bot API поддерживаются: `sendMessage`, `sendPhoto`, `editMessageText`, `answerCallbackQuery` и другие."},
}

var monitorBotMessages = []demoMsg{
	{sender: "bot", text: "MonitorBot подключён. Мониторинг серверов активен.\n\nОтслеживаю: **3 сервера**, **12 сервисов**", markup: `{"inline_keyboard":[[{"text":"Статус","callback_data":"status"},{"text":"Последние алерты","callback_data":"alerts"}]]}`},
	{sender: "user", text: "/status"},
	{sender: "bot", text: "**Статус серверов:**\n\nweb-01 — CPU 23%, RAM 1.2/4 GB\nweb-02 — CPU 18%, RAM 1.4/4 GB\ndb-01 — CPU 45%, RAM 6.1/8 GB\n\nВсе сервисы работают штатно.", markup: `{"inline_keyboard":[[{"text":"Обновить","callback_data":"status"}]]}`},
	{sender: "bot", text: "**ALERT #1047** `HighMemory`\nСервер: *db-01*\nRAM: 92% (7.4/8 GB)\nMySQL buffer pool — основной потребитель"},
	{sender: "bot", text: "**Resolved #1047** `HighMemory` на *db-01*\nRAM: 68% после автоочистки кеша"},
}

var updatesChanMessages = []string{
	"**Pusk v0.4.0**\n\nНовое:\n• PWA клиент с Mattermost-style layout\n• Inline-кнопки и callback\n• Desktop sidebar\n• Web Push уведомления\n• Docker образ 18.9 MB",
	"**Pusk v0.3.0**\n\nНовое:\n• Каналы с подпиской\n• Отправка файлов (фото, видео, голос)\n• JWT авторизация\n• i18n (русский / английский)",
}

var ackButtons = `{"inline_keyboard":[[{"text":"✓ ACK","callback_data":"ack"},{"text":"⏸ Mute 1h","callback_data":"mute"},{"text":"✓ Resolved","callback_data":"resolved"}]]}`

var alertsChanMsgs = []chanMsg{
	{text: "Alertmanager: 1 alert(s), status: firing\n\n**ALERT** `HighCPU` [warning]\nInstance: *web-01:9100*\nCPU usage above 90% for 5 minutes\n\n", markup: ackButtons},
	{text: "Alertmanager: 1 alert(s), status: resolved\n\n**Resolved** `HighCPU`\nInstance: *web-01:9100*\nCPU usage normalized\n\n"},
	{text: "Alertmanager: 1 alert(s), status: firing\n\n**ALERT** `DiskSpace` [critical]\nInstance: *db-01:9100*\nDisk usage 89% on /var/lib/mysql\n\n", markup: ackButtons},
	{text: "Alertmanager: 1 alert(s), status: resolved\n\n**Resolved** `DiskSpace`\nInstance: *db-01:9100*\nDisk cleaned, usage 52%\n\n"},
}

var generalChanMessages = []string{
	"Добро пожаловать в **Pusk**!\n\nЭто #general — канал для обсуждений вашей команды.\n\nPusk получает алерты через webhook и показывает их в каналах с кнопками ACK/Mute/Resolved.\n\nОтправьте webhook:\n```bash\ncurl -X POST https://getpusk.ru/hook/YOUR-BOT-TOKEN \\\n  -H 'Content-Type: application/json' \\\n  -d '{\"text\": \"Hello from curl!\"}'\n```\n\nАлерт от Alertmanager:\n```bash\ncurl -X POST 'https://getpusk.ru/hook/YOUR-TOKEN?format=alertmanager' \\\n  -H 'Content-Type: application/json' \\\n  -d '{\"status\":\"firing\",\"alerts\":[{\"status\":\"firing\",\"labels\":{\"alertname\":\"Test\"},\"annotations\":{\"summary\":\"Test alert\"}}]}'\n```",
}

var deploysChanMessages = []string{
	"**Deploy** `api-gateway` v2.1.0\nКластер: *production*\nПоды: 3/3 Ready\nВремя: 42 сек",
	"**Deploy** `web-frontend` v1.8.2\nКластер: *production*\nПоды: 2/2 Ready\nВремя: 38 сек",
	"**Rollback** `api-gateway` v2.1.0 → v2.0.9\nПричина: рост 5xx ошибок после деплоя\nСтатус: откат завершён, ошибки устранены",
	"**Deploy** `api-gateway` v2.1.1 (hotfix)\nКластер: *production*\nПоды: 3/3 Ready\nВремя: 45 сек",
}
