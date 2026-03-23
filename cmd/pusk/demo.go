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
		db.SetUserRole(guest.ID, "member")
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
	{sender: "bot", text: "DemoBot. Telegram Bot API на вашем сервере.\n\nВыберите тему:", markup: `{"inline_keyboard":[[{"text":"Подключение","callback_data":"connect"},{"text":"API","callback_data":"docs"}],[{"text":"GitHub","callback_data":"github"}]]}`},
	{sender: "user", text: "Как подключить?"},
	{sender: "bot", text: "Меняете `base_url` — бот работает через Pusk:\n\n```python\nbot = Bot(token=\"my-token\",\n  base_url=\"https://pusk.example.com\")\n```\n\naiogram, python-telegram-bot, telegraf — поддерживаются.", markup: `{"inline_keyboard":[[{"text":"curl пример","callback_data":"try_api"}]]}`},
	{sender: "user", text: "curl?"},
	{sender: "bot", text: "```bash\ncurl -X POST https://pusk.example.com/botTOKEN/sendMessage \\\n  -H 'Content-Type: application/json' \\\n  -d '{\"chat_id\": 1, \"text\": \"test\"}'\n```\n\nМетоды: `sendMessage`, `editMessageText`, `getUpdates`, `sendPhoto` и др."},
}

var monitorBotMessages = []demoMsg{
	{sender: "bot", text: "MonitorBot. 3 сервера, 12 сервисов.", markup: `{"inline_keyboard":[[{"text":"Статус","callback_data":"status"},{"text":"Алерты","callback_data":"alerts"}]]}`},
	{sender: "user", text: "/status"},
	{sender: "bot", text: "web-01 — CPU 23%, RAM 1.2/4 GB\nweb-02 — CPU 18%, RAM 1.4/4 GB\ndb-01 — CPU 45%, RAM 6.1/8 GB", markup: `{"inline_keyboard":[[{"text":"Обновить","callback_data":"status"}]]}`},
}

var updatesChanMessages = []string{
	"**Pusk v0.5.0**\n\n• getUpdates polling\n• Webhook: Alertmanager, Zabbix, Grafana\n• @mentions + push\n• Media upload\n• Pin message\n• Prometheus /metrics",
	"**Pusk v0.4.0**\n\n• PWA клиент\n• Inline-кнопки\n• Web Push\n• Docker 22 MB",
}

var ackButtons = `{"inline_keyboard":[[{"text":"✓ ACK","callback_data":"ack"},{"text":"⏸ Mute 1h","callback_data":"mute"},{"text":"✓ Resolved","callback_data":"resolved"}]]}`

var alertsChanMsgs = []chanMsg{
	{text: "Alertmanager: 1 alert(s), status: firing\n\n**ALERT** `HighCPU` [warning]\nInstance: *web-01:9100*\nCPU usage above 90% for 5 minutes\n\n", markup: ackButtons},
	{text: "Alertmanager: 1 alert(s), status: resolved\n\n**Resolved** `HighCPU`\nInstance: *web-01:9100*\nCPU usage normalized\n\n"},
	{text: "Alertmanager: 1 alert(s), status: firing\n\n**ALERT** `DiskSpace` [critical]\nInstance: *db-01:9100*\nDisk usage 89% on /var/lib/mysql\n\n", markup: ackButtons},
	{text: "Alertmanager: 1 alert(s), status: resolved\n\n**Resolved** `DiskSpace`\nInstance: *db-01:9100*\nDisk cleaned, usage 52%\n\n"},
}

var generalChanMessages = []string{
	"#general — канал команды.\n\nWebhook:\n```bash\ncurl -X POST https://your-server/hook/TOKEN \\\n  -d '{\"text\": \"test\"}'\n```\n\nAlertmanager:\n```bash\ncurl -X POST 'https://your-server/hook/TOKEN?format=alertmanager' \\\n  -d '{\"status\":\"firing\",\"alerts\":[{\"labels\":{\"alertname\":\"Test\"}}]}'\n```",
}

var deploysChanMessages = []string{
	"**Deploy** `api-gateway` v2.1.0\nКластер: *production*\nПоды: 3/3 Ready\nВремя: 42 сек",
	"**Deploy** `web-frontend` v1.8.2\nКластер: *production*\nПоды: 2/2 Ready\nВремя: 38 сек",
	"**Rollback** `api-gateway` v2.1.0 → v2.0.9\nПричина: рост 5xx ошибок после деплоя\nСтатус: откат завершён, ошибки устранены",
	"**Deploy** `api-gateway` v2.1.1 (hotfix)\nКластер: *production*\nПоды: 3/3 Ready\nВремя: 45 сек",
}
