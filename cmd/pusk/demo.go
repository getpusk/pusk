// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package main

import (
	"log"

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
		log.Printf("[demo] DemoBot created")
	}

	monBot, err := db.BotByToken("monitor-bot-token")
	if err != nil {
		monBot, err = db.CreateBot("monitor-bot-token", "MonitorBot")
		if err != nil {
			return
		}
		log.Printf("[demo] MonitorBot created")
	}

	// ── Users ──
	guest, err := db.AuthUser("guest", "guest")
	if err != nil {
		guest, err = db.CreateUser("guest", "guest", "Guest")
		if err != nil {
			return
		}
		log.Printf("[demo] guest user created")
	}

	// Test users (admin uses ADMIN_TOKEN, these are regular users for demo)
	for _, u := range []struct{ name, pin, display string }{
		{"pavel", "1234", "Pavel"},
		{"alex", "1234", "Alex"},
		{"ilya", "1234", "Ilya"},
	} {
		if _, err := db.AuthUser(u.name, u.pin); err != nil {
			db.CreateUser(u.name, u.pin, u.display)
			log.Printf("[demo] user %s created", u.name)
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
		log.Printf("[demo] DemoBot chat seeded")
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
		log.Printf("[demo] MonitorBot chat seeded")
	}

	// ── Channels ──
	seedChannel(db, demoBot, guest, "updates", "Pusk release notes", updatesChanMessages)
	seedChannel(db, monBot, guest, "alerts", "Server monitoring alerts", alertsChanMessages)
	seedChannel(db, monBot, guest, "deploys", "Deploy notifications", deploysChanMessages)
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
		log.Printf("[demo] #%s channel created with %d messages", name, len(messages))
	}
}

// ── Demo content ──

var demoBotMessages = []demoMsg{
	{sender: "bot", text: "Привет! Я **DemoBot** — демонстрационный бот платформы Pusk.\n\nPusk — self-hosted замена Telegram Bot API. Ваши боты, ваш сервер, ваши данные.\n\nВыберите тему:", markup: `{"inline_keyboard":[[{"text":"Что умеет Pusk?","callback_data":"features"},{"text":"Как подключить бота","callback_data":"connect"}],[{"text":"API документация","callback_data":"docs"},{"text":"GitHub","callback_data":"github"}]]}`},
	{sender: "user", text: "Что умеет Pusk?"},
	{sender: "bot", text: "**Возможности Pusk:**\n\n• 21 endpoint, совместимый с Telegram Bot API\n• Каналы с подпиской и рассылкой\n• Inline-кнопки и callback\n• Отправка файлов: фото, видео, голос, документы\n• Web Push уведомления (VAPID)\n• Markdown в сообщениях\n• PWA клиент из коробки\n\nВсё в одном бинарнике **16 MB**, потребление RAM — **3 MB**.", markup: `{"inline_keyboard":[[{"text":"Как подключить бота","callback_data":"connect"}]]}`},
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

var alertsChanMessages = []string{
	"**ALERT #1043** `HighCPU`\nСервер: *web-01*\nCPU: 94% более 5 минут\nПричина: индексация поисковым ботом",
	"**Resolved #1043** `HighCPU` на *web-01* — нагрузка снизилась до 31%",
	"**ALERT #1044** `DiskSpace`\nСервер: *db-01*\nДиск: 89% занято (178/200 GB)\nРекомендация: очистить старые бекапы",
	"**Resolved #1044** `DiskSpace` на *db-01* — удалены бекапы старше 30 дней, занято 52%",
}

var deploysChanMessages = []string{
	"**Deploy** `api-gateway` v2.1.0\nКластер: *production*\nПоды: 3/3 Ready\nВремя: 42 сек",
	"**Deploy** `web-frontend` v1.8.2\nКластер: *production*\nПоды: 2/2 Ready\nВремя: 38 сек",
	"**Rollback** `api-gateway` v2.1.0 → v2.0.9\nПричина: рост 5xx ошибок после деплоя\nСтатус: откат завершён, ошибки устранены",
	"**Deploy** `api-gateway` v2.1.1 (hotfix)\nКластер: *production*\nПоды: 3/3 Ready\nВремя: 45 сек",
}
