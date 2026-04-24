// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package ws

import (
	"encoding/json"
	"testing"
)

// mockConn creates a Conn without a real websocket
func mockConn(userID int64, key string) *Conn {
	return &Conn{
		send:   make(chan []byte, 64),
		UserID: userID,
		Key:    key,
	}
}

func TestHubRegisterUnregister(t *testing.T) {
	h := NewHub()
	c := mockConn(1, "org:1")
	h.Register("org:1", c)
	if h.Online() != 1 {
		t.Fatalf("Online = %d, want 1", h.Online())
	}
	if !h.IsConnected("org:1") {
		t.Fatal("expected connected")
	}
	h.Unregister("org:1", c)
	if h.Online() != 0 {
		t.Fatalf("Online = %d, want 0", h.Online())
	}
	if h.IsConnected("org:1") {
		t.Fatal("expected disconnected")
	}
}

func TestHubMultiDevice(t *testing.T) {
	h := NewHub()
	c1 := mockConn(1, "org:1")
	c2 := mockConn(1, "org:1")
	h.Register("org:1", c1)
	h.Register("org:1", c2)
	if h.Online() != 1 {
		t.Fatalf("Online = %d, want 1 (same key)", h.Online())
	}
	h.Unregister("org:1", c1)
	if !h.IsConnected("org:1") {
		t.Fatal("expected still connected with second conn")
	}
	h.Unregister("org:1", c2)
	if h.IsConnected("org:1") {
		t.Fatal("expected disconnected after both removed")
	}
}

func TestHubStatus(t *testing.T) {
	h := NewHub()
	if h.GetStatus("org:1") != "offline" {
		t.Fatal("expected offline for unregistered key")
	}
	c := mockConn(1, "org:1")
	h.Register("org:1", c)
	if h.GetStatus("org:1") != "online" {
		t.Fatal("expected online after register")
	}
	h.SetStatus("org:1", "away")
	if h.GetStatus("org:1") != "away" {
		t.Fatal("expected away")
	}
	h.Unregister("org:1", c)
	if h.GetStatus("org:1") != "offline" {
		t.Fatal("expected offline after unregister")
	}
}

func TestHubStatusMultiDevice_AwayIgnored(t *testing.T) {
	h := NewHub()
	c1 := mockConn(1, "org:1")
	c2 := mockConn(1, "org:1")
	h.Register("org:1", c1)
	h.Register("org:1", c2)
	// With 2 connections, "away" should be ignored (online wins)
	h.SetStatus("org:1", "away")
	if h.GetStatus("org:1") != "online" {
		t.Fatal("expected online with multiple connections")
	}
}

func TestHubStatusSingleDevice_AwayWorks(t *testing.T) {
	h := NewHub()
	c := mockConn(1, "org:1")
	h.Register("org:1", c)
	h.SetStatus("org:1", "away")
	if h.GetStatus("org:1") != "away" {
		t.Fatal("expected away with single connection")
	}
}

func TestHubActiveChannel(t *testing.T) {
	h := NewHub()
	c := mockConn(1, "org:1")
	h.Register("org:1", c)
	h.SetActiveChannel("org:1", 5)
	if h.GetActiveChannel("org:1") != 5 {
		t.Fatalf("active channel = %d, want 5", h.GetActiveChannel("org:1"))
	}
	h.SetActiveChannel("org:1", 0)
	if h.GetActiveChannel("org:1") != 0 {
		t.Fatalf("active channel = %d, want 0", h.GetActiveChannel("org:1"))
	}
}

func TestHubSendToUser(t *testing.T) {
	h := NewHub()
	c := mockConn(1, "org:1")
	h.Register("org:1", c)
	evt := Event{Type: "test", Payload: json.RawMessage(`{"msg":"hello"}`)}
	h.SendToUser("org:1", evt)
	select {
	case data := <-c.send:
		var e Event
		if err := json.Unmarshal(data, &e); err != nil {
			t.Fatal(err)
		}
		if e.Type != "test" {
			t.Fatalf("type = %q, want test", e.Type)
		}
	default:
		t.Fatal("expected message on send channel")
	}
}

func TestHubSendToUser_MultiDevice(t *testing.T) {
	h := NewHub()
	c1 := mockConn(1, "org:1")
	c2 := mockConn(1, "org:1")
	h.Register("org:1", c1)
	h.Register("org:1", c2)
	evt := Event{Type: "broadcast", Payload: json.RawMessage(`{}`)}
	h.SendToUser("org:1", evt)
	// Both should receive
	if len(c1.send) != 1 {
		t.Fatal("c1 should have received message")
	}
	if len(c2.send) != 1 {
		t.Fatal("c2 should have received message")
	}
}

func TestHubSendToUser_NoConn(t *testing.T) {
	h := NewHub()
	// Should not panic
	h.SendToUser("org:99", Event{Type: "test", Payload: json.RawMessage(`{}`)})
}

func TestHubOrgOnline(t *testing.T) {
	h := NewHub()
	h.Register("org1:1", mockConn(1, "org1:1"))
	h.Register("org1:2", mockConn(2, "org1:2"))
	h.Register("org2:1", mockConn(1, "org2:1"))
	if n := h.OrgOnline("org1:"); n != 2 {
		t.Fatalf("OrgOnline org1 = %d, want 2", n)
	}
	if n := h.OrgOnline("org2:"); n != 1 {
		t.Fatalf("OrgOnline org2 = %d, want 1", n)
	}
	if n := h.OrgOnline("org3:"); n != 0 {
		t.Fatalf("OrgOnline org3 = %d, want 0", n)
	}
}

func TestHubOnlineKeys(t *testing.T) {
	h := NewHub()
	h.Register("a:1", mockConn(1, "a:1"))
	h.Register("b:2", mockConn(2, "b:2"))
	keys := h.OnlineKeys()
	if len(keys) != 2 {
		t.Fatalf("keys = %d, want 2", len(keys))
	}
}

func TestConnSend_BufferFull(t *testing.T) {
	c := &Conn{send: make(chan []byte, 1)}
	c.Send([]byte("first"))
	c.Send([]byte("dropped")) // buffer full, should not block
	if len(c.send) != 1 {
		t.Fatal("expected 1 message in buffer")
	}
}

func TestHubUnregister_WrongConn(t *testing.T) {
	h := NewHub()
	c1 := mockConn(1, "org:1")
	c2 := mockConn(1, "org:1")
	h.Register("org:1", c1)
	// Unregister a conn that was never registered
	h.Unregister("org:1", c2)
	// c1 should still be there
	if !h.IsConnected("org:1") {
		t.Fatal("c1 should still be connected")
	}
}

func TestHub_SendToOrg(t *testing.T) {
	h := NewHub()
	c1 := mockConn(1, "orgA:1")
	c2 := mockConn(2, "orgA:2")
	c3 := mockConn(3, "orgB:3")
	h.Register("orgA:1", c1)
	h.Register("orgA:2", c2)
	h.Register("orgB:3", c3)

	evt := Event{Type: "test_org", Payload: json.RawMessage(`{"ok":true}`)}
	h.SendToOrg("orgA", evt, "")

	// orgA users should receive
	if len(c1.send) != 1 {
		t.Fatalf("c1 (orgA:1) should have received, got %d", len(c1.send))
	}
	if len(c2.send) != 1 {
		t.Fatalf("c2 (orgA:2) should have received, got %d", len(c2.send))
	}
	// orgB user should NOT receive
	if len(c3.send) != 0 {
		t.Fatalf("c3 (orgB:3) should NOT have received, got %d", len(c3.send))
	}
}

func TestHub_SendToOrg_ExcludeKey(t *testing.T) {
	h := NewHub()
	c1 := mockConn(1, "orgA:1")
	c2 := mockConn(2, "orgA:2")
	h.Register("orgA:1", c1)
	h.Register("orgA:2", c2)

	evt := Event{Type: "test_exclude", Payload: json.RawMessage(`{}`)}
	h.SendToOrg("orgA", evt, "orgA:1")

	// c1 excluded
	if len(c1.send) != 0 {
		t.Fatalf("c1 should be excluded, got %d", len(c1.send))
	}
	// c2 should receive
	if len(c2.send) != 1 {
		t.Fatalf("c2 should have received, got %d", len(c2.send))
	}
}

func TestHubSetStatus_UnregisteredKey(t *testing.T) {
	h := NewHub()
	// Should not panic on unregistered key
	h.SetStatus("nobody:1", "online")
	if h.GetStatus("nobody:1") != "offline" {
		t.Fatal("unregistered key should remain offline")
	}
}
