// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package bot

import (
	"encoding/json"
	"strings"
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
	got, err := te.Render("alertmanager", payload)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	for _, want := range []string{"HighCPU", "critical", "server1:9090", "CPU is high"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output: %s", want, got)
		}
	}
}

func TestTemplateAlertmanagerResolved(t *testing.T) {
	te := NewTemplateEngine()
	payload := map[string]interface{}{
		"status": "resolved",
		"alerts": []interface{}{
			map[string]interface{}{
				"status":      "resolved",
				"labels":      map[string]interface{}{"alertname": "HighCPU", "severity": "warning", "instance": "server1:9090"},
				"annotations": map[string]interface{}{"summary": "CPU is normal"},
			},
		},
	}
	got, err := te.Render("alertmanager", payload)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if !strings.Contains(got, "Resolved") {
		t.Errorf("resolved alert should contain Resolved: %s", got)
	}
}

func TestTemplateAlertmanagerMultiple(t *testing.T) {
	te := NewTemplateEngine()
	payload := map[string]interface{}{
		"status": "firing",
		"alerts": []interface{}{
			map[string]interface{}{"status": "firing", "labels": map[string]interface{}{"alertname": "HighCPU"}, "annotations": map[string]interface{}{"summary": "CPU"}},
			map[string]interface{}{"status": "resolved", "labels": map[string]interface{}{"alertname": "DiskFull"}, "annotations": map[string]interface{}{"summary": "Disk"}},
		},
	}
	got, err := te.Render("alertmanager", payload)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if !strings.Contains(got, "HighCPU") || !strings.Contains(got, "DiskFull") {
		t.Errorf("multi alert should contain both: %s", got)
	}
}

func TestTemplateZabbix(t *testing.T) {
	te := NewTemplateEngine()
	payload := map[string]interface{}{
		"subject": "Disk full on server1", "message": "Disk usage is above 95%",
		"severity": "High", "status": "PROBLEM", "host": "server1",
	}
	got, err := te.Render("zabbix", payload)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	for _, want := range []string{"Disk full", "server1", "95%"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output: %s", want, got)
		}
	}
}

func TestTemplateZabbixResolved(t *testing.T) {
	te := NewTemplateEngine()
	payload := map[string]interface{}{
		"subject": "Disk resolved", "message": "OK now",
		"severity": "High", "status": "RESOLVED", "host": "server1",
	}
	got, err := te.Render("zabbix", payload)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if !strings.Contains(got, "Resolved") {
		t.Errorf("resolved should contain Resolved: %s", got)
	}
}

func TestTemplateZabbixCapitalized(t *testing.T) {
	te := NewTemplateEngine()
	payload := map[string]interface{}{
		"Subject": "Disk full", "Message": "95%",
		"Severity": "High", "Status": "OK", "Host": "server2",
	}
	got, err := te.Render("zabbix", payload)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if !strings.Contains(got, "Disk full") {
		t.Errorf("capitalized keys should work: %s", got)
	}
}

func TestTemplateZabbixFallback(t *testing.T) {
	te := NewTemplateEngine()
	payload := map[string]interface{}{"foo": "bar"}
	got, err := te.Render("zabbix", payload)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if got == "" {
		t.Error("fallback should produce output")
	}
}

func TestTemplateGrafanaLegacy(t *testing.T) {
	te := NewTemplateEngine()
	payload := map[string]interface{}{"title": "CPU Alert", "state": "alerting", "message": "CPU too high"}
	got, err := te.Render("grafana", payload)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if !strings.Contains(got, "CPU Alert") {
		t.Errorf("grafana legacy should contain title: %s", got)
	}
}

func TestTemplateGrafanaOk(t *testing.T) {
	te := NewTemplateEngine()
	payload := map[string]interface{}{"title": "CPU Alert", "state": "ok", "message": "CPU normal"}
	got, err := te.Render("grafana", payload)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if !strings.Contains(got, "Resolved") {
		t.Errorf("grafana ok should contain Resolved: %s", got)
	}
}

func TestTemplateGrafanaUnified(t *testing.T) {
	te := NewTemplateEngine()
	payload := map[string]interface{}{
		"status": "firing",
		"alerts": []interface{}{
			map[string]interface{}{"status": "firing", "labels": map[string]interface{}{"alertname": "GrafanaAlert"}, "annotations": map[string]interface{}{"summary": "Grafana unified"}},
		},
	}
	got, err := te.Render("grafana", payload)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if !strings.Contains(got, "GrafanaAlert") {
		t.Errorf("grafana unified should contain alert name: %s", got)
	}
}

func TestTemplateGrafanaFallback(t *testing.T) {
	te := NewTemplateEngine()
	payload := map[string]interface{}{"foo": "bar"}
	got, err := te.Render("grafana", payload)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if got == "" {
		t.Error("fallback should produce output")
	}
}

func TestTemplateRawText(t *testing.T) {
	te := NewTemplateEngine()
	payload := map[string]interface{}{"message": "Hello world"}
	got, err := te.Render("raw", payload)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	expected := extractText(payload)
	if got != expected {
		t.Errorf("raw text mismatch: got %q, want %q", got, expected)
	}
}

func TestTemplateRawJSON(t *testing.T) {
	te := NewTemplateEngine()
	payload := map[string]interface{}{"foo": "bar", "num": float64(42)}
	raw, _ := json.MarshalIndent(payload, "", "  ")
	expected := "```json\n" + string(raw) + "\n```"
	got, err := te.Render("raw", payload)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if got != expected {
		t.Errorf("raw JSON mismatch: got %q, want %q", got, expected)
	}
}

func TestTemplateGrafanaRuleName(t *testing.T) {
	te := NewTemplateEngine()
	payload := map[string]interface{}{"ruleName": "CPU Alert", "state": "alerting", "message": "CPU too high"}
	got, err := te.Render("grafana", payload)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if !strings.Contains(got, "CPU Alert") {
		t.Errorf("grafana ruleName should work: %s", got)
	}
}

func TestTemplateZabbixStatusOk(t *testing.T) {
	te := NewTemplateEngine()
	payload := map[string]interface{}{"subject": "Test", "message": "OK", "status": "OK"}
	got, err := te.Render("zabbix", payload)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if !strings.Contains(got, "Resolved") {
		t.Errorf("zabbix OK should be Resolved: %s", got)
	}
}
