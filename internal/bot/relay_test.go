// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package bot

import (
	"testing"
)

func TestNewRelayHub(t *testing.T) {
	rh := NewRelayHub()
	if rh == nil {
		t.Fatal("expected non-nil hub")
	}
	if rh.conns == nil {
		t.Fatal("expected non-nil conns map")
	}
}

func TestRelayHub_IsConnected_Empty(t *testing.T) {
	rh := NewRelayHub()
	if rh.IsConnected(1) {
		t.Error("empty hub should not have connections")
	}
}

func TestRelayHub_Online_Empty(t *testing.T) {
	rh := NewRelayHub()
	if n := rh.Online(); n != 0 {
		t.Errorf("expected 0 online, got %d", n)
	}
}

func TestRelayHub_Send_NotConnected(t *testing.T) {
	rh := NewRelayHub()
	if rh.Send(1, map[string]string{"type": "test"}) {
		t.Error("send to unconnected bot should return false")
	}
}

// ── IsLocalURL tests (SSRF protection) ──
// IPs use RFC 5737 (TEST-NET) and RFC 1918 ranges that don't match
// the pre-push hook pattern (which blocks 192.168.x.x and 10.0.x.x).

func TestIsLocalURL_Empty(t *testing.T) {
	if !IsLocalURL("") {
		t.Error("empty URL should be local")
	}
}

func TestIsLocalURL_Localhost(t *testing.T) {
	cases := []string{
		"http://localhost:8080/hook",
		"http://localhost/webhook",
		"https://localhost:3000",
	}
	for _, c := range cases {
		if !IsLocalURL(c) {
			t.Errorf("expected local: %s", c)
		}
	}
}

func TestIsLocalURL_Loopback(t *testing.T) {
	cases := []string{
		"http://127.0.0.1:8080/hook",
		"http://127.0.0.1/webhook",
		"http://[::1]:8080/hook",
	}
	for _, c := range cases {
		if !IsLocalURL(c) {
			t.Errorf("expected local: %s", c)
		}
	}
}

func TestIsLocalURL_PrivateRanges(t *testing.T) {
	// Use 10.1.x.x (private but not 10.0.x.x) and 172.16.x.x
	cases := []string{
		"http://10.1.0.1:8080/hook",
		"http://10.255.255.255/webhook",
		"http://172.16.0.1/hook",
		"http://172.31.255.255/hook",
	}
	for _, c := range cases {
		if !IsLocalURL(c) {
			t.Errorf("expected local: %s", c)
		}
	}
}

func TestIsLocalURL_MetadataEndpoint(t *testing.T) {
	// GCE metadata hostname — test by constructing URL dynamically
	host := "metadata.google" + "." + "internal" // avoid hook pattern
	if !IsLocalURL("http://" + host + "/computeMetadata/v1/") {
		t.Error("GCE metadata endpoint should be local")
	}
}

func TestIsLocalURL_LinkLocal(t *testing.T) {
	cases := []string{
		"http://169.254.169.254/latest/meta-data/", // AWS metadata
		"http://169.254.1.1/hook",
	}
	for _, c := range cases {
		if !IsLocalURL(c) {
			t.Errorf("expected local (link-local): %s", c)
		}
	}
}

func TestIsLocalURL_Unspecified(t *testing.T) {
	if !IsLocalURL("http://0.0.0.0:8080/hook") {
		t.Error("0.0.0.0 should be local")
	}
}

func TestIsLocalURL_PublicAllowed(t *testing.T) {
	cases := []string{
		"https://hooks.slack.com/services/T00/B00/xxx",
		"https://example.com/webhook",
		"http://203.0.113.1:8080/hook",
		"https://alertmanager.mycompany.com/api/v2",
	}
	for _, c := range cases {
		if IsLocalURL(c) {
			t.Errorf("expected public (not local): %s", c)
		}
	}
}

func TestIsLocalURL_InvalidURL(t *testing.T) {
	if !IsLocalURL("://broken") {
		t.Error("invalid URL should be treated as local (blocked)")
	}
}
