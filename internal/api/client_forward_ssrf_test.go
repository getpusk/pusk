// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package api

import (
	"strings"
	"testing"
)

// TestWebhookClientBlocksLoopback verifies the SSRF connect-time gate: the
// shared webhook client refuses to dial a loopback/internal address, even if a
// URL were to slip past the IsLocalURL pre-filter (e.g. via DNS rebinding).
func TestWebhookClientBlocksLoopback(t *testing.T) {
	_, err := webhookClient.Get("http://127.0.0.1:9/hook")
	if err == nil {
		t.Fatal("expected webhook client to refuse a loopback connection")
	}
	if !strings.Contains(err.Error(), "internal address") {
		t.Errorf("error = %q, want an SSRF refusal mentioning the internal address", err.Error())
	}
}
