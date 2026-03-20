// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package bot

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// WebhookHandler handles incoming webhooks from monitoring systems
// and converts them to Pusk channel messages.
//
// Usage:
//
//	POST /hook/{token}?format=alertmanager|zabbix|grafana|raw
//
// The token is a bot token. Messages are sent to the first channel
// owned by that bot, or to a channel specified by ?channel= param.
func (h *Handler) webhook(w http.ResponseWriter, r *http.Request) {
	bot, err := h.authBot(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	s := h.db(r)
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "raw"
	}
	channelName := r.URL.Query().Get("channel")

	// Parse JSON body
	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		// Try as array (some systems send arrays)
		w.WriteHeader(200) // Always 200 for webhook senders
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "error": "invalid json"})
		return
	}

	// Format message based on source type
	var text string
	switch format {
	case "alertmanager":
		text = formatAlertmanager(payload)
	case "zabbix":
		text = formatZabbix(payload)
	case "grafana":
		text = formatGrafana(payload)
	case "raw":
		// Smart extract: if payload has msg/message/text field, use it as plain text
		if t := extractText(payload); t != "" {
			text = t
		} else {
			raw, _ := json.MarshalIndent(payload, "", "  ")
			text = "```json\n" + string(raw) + "\n```"
		}
	default:
		text = fmt.Sprintf("Webhook (%s):\n```json\n%s\n```", format, truncateStr(fmt.Sprintf("%v", payload), 500))
	}

	if text == "" {
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
	}

	// Find target channel
	if channelName == "" {
		channelName = "alerts" // default channel
	}

	ch, err := s.ChannelByName(bot.ID, channelName)
	if err != nil {
		// Try creating the channel
		ch, err = s.CreateChannel(bot.ID, channelName, "Webhook alerts")
		if err != nil {
			log.Printf("[webhook] cannot create channel %s: %v", channelName, err)
			w.WriteHeader(200)
			json.NewEncoder(w).Encode(map[string]string{"status": "ok", "error": "channel error"})
			return
		}
		// Auto-subscribe admin (user_id=1) to new channel
		s.Subscribe(ch.ID, 1)
		log.Printf("[webhook] auto-created channel #%s for bot %s", channelName, bot.Name)
	}

	// Send message to channel
	msg, err := s.SaveChannelMessage(ch.ID, text, "", "", "")
	if err != nil {
		log.Printf("[webhook] save error: %v", err)
	}

	// Push to subscribers via WebSocket (reuse existing push logic)
	if msg != nil {
		h.pushChannelMessage(s, ch, bot, msg)
	}

	log.Printf("[webhook] %s → #%s (%s)", format, channelName, bot.Name)
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ── Format functions ──

func formatAlertmanager(p map[string]interface{}) string {
	status, _ := p["status"].(string)
	alerts, ok := p["alerts"].([]interface{})
	if !ok || len(alerts) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, a := range alerts {
		alert, ok := a.(map[string]interface{})
		if !ok {
			continue
		}
		aStatus, _ := alert["status"].(string)
		labels, _ := alert["labels"].(map[string]interface{})
		annotations, _ := alert["annotations"].(map[string]interface{})

		alertname := getStr(labels, "alertname")
		severity := getStr(labels, "severity")
		instance := getStr(labels, "instance")
		summary := getStr(annotations, "summary")
		description := getStr(annotations, "description")

		icon := "ALERT"
		if aStatus == "resolved" {
			icon = "Resolved"
		}

		sb.WriteString(fmt.Sprintf("**%s** `%s`", icon, alertname))
		if severity != "" {
			sb.WriteString(fmt.Sprintf(" [%s]", severity))
		}
		sb.WriteString("\n")
		if instance != "" {
			sb.WriteString(fmt.Sprintf("Instance: *%s*\n", instance))
		}
		if summary != "" {
			sb.WriteString(summary + "\n")
		}
		if description != "" && description != summary {
			sb.WriteString(description + "\n")
		}
		sb.WriteString("\n")
	}

	header := fmt.Sprintf("Alertmanager: %d alert(s), status: %s\n\n", len(alerts), status)
	return header + sb.String()
}

func formatZabbix(p map[string]interface{}) string {
	subject := getStr(p, "subject")
	if subject == "" {
		subject = getStr(p, "Subject")
	}
	message := getStr(p, "message")
	if message == "" {
		message = getStr(p, "Message")
	}
	severity := getStr(p, "severity")
	if severity == "" {
		severity = getStr(p, "Severity")
	}
	status := getStr(p, "status")
	if status == "" {
		status = getStr(p, "Status")
	}
	host := getStr(p, "host")
	if host == "" {
		host = getStr(p, "Host")
	}

	if subject == "" && message == "" {
		// Fallback: raw dump
		raw, _ := json.MarshalIndent(p, "", "  ")
		return "**Zabbix**\n```json\n" + string(raw) + "\n```"
	}

	var sb strings.Builder
	icon := "ALERT"
	if strings.Contains(strings.ToLower(status), "resolved") || strings.Contains(strings.ToLower(status), "ok") {
		icon = "Resolved"
	}

	sb.WriteString(fmt.Sprintf("**%s** %s", icon, subject))
	if severity != "" {
		sb.WriteString(fmt.Sprintf(" [%s]", severity))
	}
	sb.WriteString("\n")
	if host != "" {
		sb.WriteString(fmt.Sprintf("Host: *%s*\n", host))
	}
	if message != "" {
		sb.WriteString(message + "\n")
	}
	return sb.String()
}

func formatGrafana(p map[string]interface{}) string {
	status, _ := p["status"].(string)
	alerts, ok := p["alerts"].([]interface{})

	// Grafana unified alerting format (similar to Alertmanager)
	if ok && len(alerts) > 0 {
		return formatAlertmanager(p) // Same structure
	}

	// Legacy Grafana format
	title := getStr(p, "title")
	message := getStr(p, "message")
	state := getStr(p, "state")
	ruleName := getStr(p, "ruleName")

	if title == "" && ruleName == "" {
		raw, _ := json.MarshalIndent(p, "", "  ")
		return "**Grafana**\n```json\n" + string(raw) + "\n```"
	}

	name := title
	if name == "" {
		name = ruleName
	}

	icon := "ALERT"
	if state == "ok" || status == "resolved" {
		icon = "Resolved"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**%s** %s", icon, name))
	if state != "" {
		sb.WriteString(fmt.Sprintf(" [%s]", state))
	}
	sb.WriteString("\n")
	if message != "" {
		sb.WriteString(message + "\n")
	}
	return sb.String()
}

// ── Helpers ──

func getStr(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// extractText tries to find a human-readable text field in webhook payload.
// Covers: Uptime Kuma (msg), generic webhooks (message, text).
func extractText(p map[string]interface{}) string {
	for _, key := range []string{"msg", "message", "text"} {
		if v := getStr(p, key); v != "" {
			return v
		}
	}
	return ""
}

func truncateStr(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "..."
}
