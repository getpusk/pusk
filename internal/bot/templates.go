// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package bot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"text/template"
)

// Default template strings that reproduce EXACT output of hardcoded format functions.

const defaultAlertmanagerTpl = `Alertmanager: {{ len .alerts }} alert(s), status: {{ .status }}

{{ range .alerts }}{{ $labels := .labels }}{{ $annotations := .annotations }}{{ $aStatus := .status }}{{ if eq $aStatus "resolved" }}**Resolved**{{ else }}**ALERT**{{ end }} ` + "`" + `{{ getStr $labels "alertname" }}` + "`" + `{{ $sev := getStr $labels "severity" }}{{ if $sev }} [{{ $sev }}]{{ end }}
{{ $inst := getStr $labels "instance" }}{{ if $inst }}Instance: *{{ $inst }}*
{{ end }}{{ $sum := getStr $annotations "summary" }}{{ if $sum }}{{ $sum }}
{{ end }}{{ $desc := getStr $annotations "description" }}{{ if and $desc (ne $desc $sum) }}{{ $desc }}
{{ end }}
{{ end }}`

const defaultZabbixTpl = `{{ $subject := getStrCI . "subject" }}{{ $message := getStrCI . "message" }}{{ $severity := getStrCI . "severity" }}{{ $status := getStrCI . "status" }}{{ $host := getStrCI . "host" }}{{ if and (eq $subject "") (eq $message "") }}**Zabbix**
` + "```json" + `
{{ jsonIndent . }}
` + "```" + `{{ else }}{{ if isResolvedZabbix $status }}**Resolved**{{ else }}**ALERT**{{ end }} {{ $subject }}{{ if $severity }} [{{ $severity }}]{{ end }}
{{ if $host }}Host: *{{ $host }}*
{{ end }}{{ if $message }}{{ $message }}
{{ end }}{{ end }}`

const defaultGrafanaTpl = `{{ $alerts := .alerts }}{{ if isAlertsList $alerts }}{{ renderAlertmanager . }}{{ else }}{{ $title := getStr . "title" }}{{ $ruleName := getStr . "ruleName" }}{{ $message := getStr . "message" }}{{ $state := getStr . "state" }}{{ $status := getStr . "status" }}{{ if and (eq $title "") (eq $ruleName "") }}**Grafana**
` + "```json" + `
{{ jsonIndent . }}
` + "```" + `{{ else }}{{ $name := $title }}{{ if eq $name "" }}{{ $name = $ruleName }}{{ end }}{{ if or (eq $state "ok") (eq $status "resolved") }}**Resolved**{{ else }}**ALERT**{{ end }} {{ $name }}{{ if $state }} [{{ $state }}]{{ end }}
{{ if $message }}{{ $message }}
{{ end }}{{ end }}{{ end }}`

const defaultRawTpl = `{{ $text := extractText . }}{{ if $text }}{{ $text }}{{ else }}` + "```json" + `
{{ jsonIndent . }}
` + "```" + `{{ end }}`

// TemplateEngine manages parsed templates for webhook formatting.
type TemplateEngine struct {
	templates map[string]*template.Template
	funcMap   template.FuncMap
}

// NewTemplateEngine creates a template engine with defaults, then overrides from env vars.
func NewTemplateEngine() *TemplateEngine {
	te := &TemplateEngine{
		templates: make(map[string]*template.Template),
	}

	// Build the function map — we need a forward reference for renderAlertmanager
	te.funcMap = template.FuncMap{
		"getStr": func(m interface{}, key string) string {
			if mm, ok := m.(map[string]interface{}); ok {
				return getStr(mm, key)
			}
			return ""
		},
		"getStrCI": func(m interface{}, key string) string {
			mm, ok := m.(map[string]interface{})
			if !ok {
				return ""
			}
			v := getStr(mm, key)
			if v == "" {
				// Try capitalized
				cap := strings.ToUpper(key[:1]) + key[1:]
				v = getStr(mm, cap)
			}
			return v
		},
		"jsonIndent": func(v interface{}) string {
			b, _ := json.MarshalIndent(v, "", "  ")
			return string(b)
		},
		"truncate": func(s string, max int) string {
			return truncateStr(s, max)
		},
		"isResolvedZabbix": func(status string) bool {
			lower := strings.ToLower(status)
			return strings.Contains(lower, "resolved") || strings.Contains(lower, "ok")
		},
		"isAlertsList": func(v interface{}) bool {
			if v == nil {
				return false
			}
			arr, ok := v.([]interface{})
			return ok && len(arr) > 0
		},
		"extractText": func(m interface{}) string {
			if mm, ok := m.(map[string]interface{}); ok {
				return extractText(mm)
			}
			return ""
		},
		"renderAlertmanager": func(p interface{}) string {
			if mm, ok := p.(map[string]interface{}); ok {
				result, err := te.Render("alertmanager", mm)
				if err != nil {
					return ""
				}
				return result
			}
			return ""
		},
	}

	// Parse default templates
	defaults := map[string]string{
		"alertmanager": defaultAlertmanagerTpl,
		"zabbix":       defaultZabbixTpl,
		"grafana":      defaultGrafanaTpl,
		"raw":          defaultRawTpl,
	}

	for name, tplStr := range defaults {
		t, err := template.New(name).Funcs(te.funcMap).Parse(tplStr)
		if err != nil {
			slog.Error("failed to parse default template", "name", name, "error", err)
			continue
		}
		te.templates[name] = t
	}

	// Override from env vars
	envMap := map[string]string{
		"alertmanager": "PUSK_TPL_ALERTMANAGER",
		"zabbix":       "PUSK_TPL_ZABBIX",
		"grafana":      "PUSK_TPL_GRAFANA",
		"raw":          "PUSK_TPL_RAW",
	}

	for name, envKey := range envMap {
		path := os.Getenv(envKey)
		if path == "" {
			continue
		}
		//nolint:gosec // G703,G304: path from filepath.Join with fixed templates dir
		data, err := os.ReadFile(path) // #nosec G703 G304
		if err != nil {
			slog.Warn("cannot read custom template", "env", envKey, "path", path, "error", err)
			continue
		}
		//nolint:gosec // G708: templates from local files, not user input
		t, err := template.New(name).Funcs(te.funcMap).Parse(string(data)) // #nosec G708
		if err != nil {
			slog.Warn("cannot parse custom template", "env", envKey, "path", path, "error", err)
			continue
		}
		te.templates[name] = t
		slog.Info("loaded custom webhook template", "format", name, "path", path)
	}

	return te
}

// Render executes the named template with the given payload.
func (te *TemplateEngine) Render(format string, payload map[string]interface{}) (string, error) {
	t, ok := te.templates[format]
	if !ok {
		return "", fmt.Errorf("unknown template: %s", format)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, payload); err != nil {
		return "", err
	}
	return buf.String(), nil
}
