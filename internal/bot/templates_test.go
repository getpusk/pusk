// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package bot

import (
	"encoding/json"
	"testing"
)

func TestTemplateAlertmanager(t *testing.T) {
	te := NewTemplateEngine()

	payload := map[string]interface{}{
		"status": "firing",
		"alerts": []interface{}{
			map[string]interface{}{
				"status": "firing",
				"labels": map[string]interface{}{
					"alertname": "HighCPU",
					"severity":  "critical",
					"instance":  "server1:9090",
				},
				"annotations": map[string]interface{}{
					"summary":     "CPU is high",
					"description": "CPU usage is above 90%",
				},
			},
		},
	}

	expected := formatAlertmanager(payload)
	got, err := te.Render("alertmanager", payload)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}

	if got != expected {
		t.Errorf("Alertmanager output mismatch.\n--- EXPECTED (len=%d) ---\n%q\n--- GOT (len=%d) ---\n%q", len(expected), expected, len(got), got)
	}
}

func TestTemplateAlertmanagerResolved(t *testing.T) {
	te := NewTemplateEngine()

	payload := map[string]interface{}{
		"status": "resolved",
		"alerts": []interface{}{
			map[string]interface{}{
				"status": "resolved",
				"labels": map[string]interface{}{
					"alertname": "HighCPU",
					"severity":  "warning",
					"instance":  "server1:9090",
				},
				"annotations": map[string]interface{}{
					"summary": "CPU is normal",
				},
			},
		},
	}

	expected := formatAlertmanager(payload)
	got, err := te.Render("alertmanager", payload)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}

	if got != expected {
		t.Errorf("Alertmanager resolved mismatch.\n--- EXPECTED (len=%d) ---\n%q\n--- GOT (len=%d) ---\n%q", len(expected), expected, len(got), got)
	}
}

func TestTemplateAlertmanagerMultiple(t *testing.T) {
	te := NewTemplateEngine()

	payload := map[string]interface{}{
		"status": "firing",
		"alerts": []interface{}{
			map[string]interface{}{
				"status": "firing",
				"labels": map[string]interface{}{
					"alertname": "HighCPU",
					"severity":  "critical",
					"instance":  "server1:9090",
				},
				"annotations": map[string]interface{}{
					"summary":     "CPU is high",
					"description": "CPU usage is above 90%",
				},
			},
			map[string]interface{}{
				"status": "resolved",
				"labels": map[string]interface{}{
					"alertname": "DiskFull",
					"severity":  "warning",
				},
				"annotations": map[string]interface{}{
					"summary":     "Disk resolved",
					"description": "Disk resolved",
				},
			},
		},
	}

	expected := formatAlertmanager(payload)
	got, err := te.Render("alertmanager", payload)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}

	if got != expected {
		t.Errorf("Alertmanager multi mismatch.\n--- EXPECTED (len=%d) ---\n%q\n--- GOT (len=%d) ---\n%q", len(expected), expected, len(got), got)
	}
}

func TestTemplateAlertmanagerNoSeverity(t *testing.T) {
	te := NewTemplateEngine()

	payload := map[string]interface{}{
		"status": "firing",
		"alerts": []interface{}{
			map[string]interface{}{
				"status": "firing",
				"labels": map[string]interface{}{
					"alertname": "TestAlert",
				},
				"annotations": map[string]interface{}{
					"summary": "test",
				},
			},
		},
	}

	expected := formatAlertmanager(payload)
	got, err := te.Render("alertmanager", payload)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}

	if got != expected {
		t.Errorf("Alertmanager no-severity mismatch.\n--- EXPECTED (len=%d) ---\n%q\n--- GOT (len=%d) ---\n%q", len(expected), expected, len(got), got)
	}
}

func TestTemplateZabbix(t *testing.T) {
	te := NewTemplateEngine()

	payload := map[string]interface{}{
		"subject":  "Disk full on server1",
		"message":  "Disk usage is above 95%",
		"severity": "High",
		"status":   "PROBLEM",
		"host":     "server1",
	}

	expected := formatZabbix(payload)
	got, err := te.Render("zabbix", payload)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}

	if got != expected {
		t.Errorf("Zabbix output mismatch.\n--- EXPECTED (len=%d) ---\n%q\n--- GOT (len=%d) ---\n%q", len(expected), expected, len(got), got)
	}
}

func TestTemplateZabbixResolved(t *testing.T) {
	te := NewTemplateEngine()

	payload := map[string]interface{}{
		"subject":  "Disk resolved",
		"message":  "OK now",
		"severity": "High",
		"status":   "RESOLVED",
		"host":     "server1",
	}

	expected := formatZabbix(payload)
	got, err := te.Render("zabbix", payload)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}

	if got != expected {
		t.Errorf("Zabbix resolved mismatch.\n--- EXPECTED (len=%d) ---\n%q\n--- GOT (len=%d) ---\n%q", len(expected), expected, len(got), got)
	}
}

func TestTemplateZabbixCapitalized(t *testing.T) {
	te := NewTemplateEngine()

	payload := map[string]interface{}{
		"Subject":  "Disk full",
		"Message":  "95%",
		"Severity": "High",
		"Status":   "OK",
		"Host":     "server2",
	}

	expected := formatZabbix(payload)
	got, err := te.Render("zabbix", payload)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}

	if got != expected {
		t.Errorf("Zabbix cap mismatch.\n--- EXPECTED (len=%d) ---\n%q\n--- GOT (len=%d) ---\n%q", len(expected), expected, len(got), got)
	}
}

func TestTemplateZabbixFallback(t *testing.T) {
	te := NewTemplateEngine()

	payload := map[string]interface{}{
		"foo": "bar",
	}

	expected := formatZabbix(payload)
	got, err := te.Render("zabbix", payload)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}

	if got != expected {
		t.Errorf("Zabbix fallback mismatch.\n--- EXPECTED (len=%d) ---\n%q\n--- GOT (len=%d) ---\n%q", len(expected), expected, len(got), got)
	}
}

func TestTemplateGrafanaLegacy(t *testing.T) {
	te := NewTemplateEngine()

	payload := map[string]interface{}{
		"title":   "CPU Alert",
		"state":   "alerting",
		"message": "CPU too high",
	}

	expected := formatGrafana(payload)
	got, err := te.Render("grafana", payload)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}

	if got != expected {
		t.Errorf("Grafana legacy mismatch.\n--- EXPECTED (len=%d) ---\n%q\n--- GOT (len=%d) ---\n%q", len(expected), expected, len(got), got)
	}
}

func TestTemplateGrafanaOk(t *testing.T) {
	te := NewTemplateEngine()

	payload := map[string]interface{}{
		"title":   "CPU Alert",
		"state":   "ok",
		"message": "CPU normal",
	}

	expected := formatGrafana(payload)
	got, err := te.Render("grafana", payload)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}

	if got != expected {
		t.Errorf("Grafana ok mismatch.\n--- EXPECTED (len=%d) ---\n%q\n--- GOT (len=%d) ---\n%q", len(expected), expected, len(got), got)
	}
}

func TestTemplateGrafanaUnified(t *testing.T) {
	te := NewTemplateEngine()

	payload := map[string]interface{}{
		"status": "firing",
		"alerts": []interface{}{
			map[string]interface{}{
				"status": "firing",
				"labels": map[string]interface{}{
					"alertname": "GrafanaAlert",
					"severity":  "critical",
				},
				"annotations": map[string]interface{}{
					"summary": "Grafana unified alerting",
				},
			},
		},
	}

	expected := formatGrafana(payload)
	got, err := te.Render("grafana", payload)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}

	if got != expected {
		t.Errorf("Grafana unified mismatch.\n--- EXPECTED (len=%d) ---\n%q\n--- GOT (len=%d) ---\n%q", len(expected), expected, len(got), got)
	}
}

func TestTemplateGrafanaFallback(t *testing.T) {
	te := NewTemplateEngine()

	payload := map[string]interface{}{
		"foo": "bar",
	}

	expected := formatGrafana(payload)
	got, err := te.Render("grafana", payload)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}

	if got != expected {
		t.Errorf("Grafana fallback mismatch.\n--- EXPECTED (len=%d) ---\n%q\n--- GOT (len=%d) ---\n%q", len(expected), expected, len(got), got)
	}
}

func TestTemplateRawText(t *testing.T) {
	te := NewTemplateEngine()

	payload := map[string]interface{}{
		"message": "Hello world",
	}

	expected := extractText(payload)
	got, err := te.Render("raw", payload)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}

	if got != expected {
		t.Errorf("Raw text mismatch.\n--- EXPECTED (len=%d) ---\n%q\n--- GOT (len=%d) ---\n%q", len(expected), expected, len(got), got)
	}
}

func TestTemplateRawJSON(t *testing.T) {
	te := NewTemplateEngine()

	payload := map[string]interface{}{
		"foo": "bar",
		"num": float64(42),
	}

	raw, _ := json.MarshalIndent(payload, "", "  ")
	expected := "```json\n" + string(raw) + "\n```"

	got, err := te.Render("raw", payload)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}

	if got != expected {
		t.Errorf("Raw JSON mismatch.\n--- EXPECTED (len=%d) ---\n%q\n--- GOT (len=%d) ---\n%q", len(expected), expected, len(got), got)
	}
}

func TestTemplateGrafanaRuleName(t *testing.T) {
	te := NewTemplateEngine()

	payload := map[string]interface{}{
		"ruleName": "CPU Alert",
		"state":    "alerting",
		"message":  "CPU too high",
	}

	expected := formatGrafana(payload)
	got, err := te.Render("grafana", payload)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}

	if got != expected {
		t.Errorf("Grafana ruleName mismatch.\n--- EXPECTED (len=%d) ---\n%q\n--- GOT (len=%d) ---\n%q", len(expected), expected, len(got), got)
	}
}

func TestTemplateZabbixStatusOk(t *testing.T) {
	te := NewTemplateEngine()

	payload := map[string]interface{}{
		"subject": "Test",
		"message": "OK",
		"status":  "OK",
	}

	expected := formatZabbix(payload)
	got, err := te.Render("zabbix", payload)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}

	if got != expected {
		t.Errorf("Zabbix OK mismatch.\n--- EXPECTED (len=%d) ---\n%q\n--- GOT (len=%d) ---\n%q", len(expected), expected, len(got), got)
	}
}
