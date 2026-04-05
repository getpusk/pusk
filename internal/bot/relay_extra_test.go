// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package bot

import (
	"testing"

	"github.com/pusk-platform/pusk/internal/ws"
)

func TestRelayHub_Register(t *testing.T) {
	rh := NewRelayHub()
	c := ws.NewConn(nil, 0)
	rh.Register(1, c)

	if !rh.IsConnected(1) {
		t.Error("bot 1 should be connected after Register")
	}
	if rh.Online() != 1 {
		t.Errorf("expected 1 online, got %d", rh.Online())
	}
}

func TestRelayHub_Register_ReplacesOld(t *testing.T) {
	rh := NewRelayHub()
	c1 := ws.NewConn(nil, 0)
	c2 := ws.NewConn(nil, 0)

	rh.Register(1, c1)
	rh.Register(1, c2) // should replace c1

	if rh.Online() != 1 {
		t.Errorf("expected 1 online after replace, got %d", rh.Online())
	}
	// Old conn should have received "replaced" message
	rh.mu.RLock()
	cur := rh.conns[1]
	rh.mu.RUnlock()
	if cur != c2 {
		t.Error("expected c2 to be current connection")
	}
}

func TestRelayHub_Unregister(t *testing.T) {
	rh := NewRelayHub()
	c := ws.NewConn(nil, 0)
	rh.Register(1, c)
	rh.Unregister(1, c)

	if rh.IsConnected(1) {
		t.Error("bot 1 should be disconnected after Unregister")
	}
	if rh.Online() != 0 {
		t.Errorf("expected 0 online, got %d", rh.Online())
	}
}

func TestRelayHub_Unregister_WrongConn(t *testing.T) {
	rh := NewRelayHub()
	c1 := ws.NewConn(nil, 0)
	c2 := ws.NewConn(nil, 0)
	rh.Register(1, c1)
	rh.Unregister(1, c2) // wrong conn, should NOT unregister

	if !rh.IsConnected(1) {
		t.Error("bot 1 should still be connected (wrong conn)")
	}
}

func TestRelayHub_Send_Connected(t *testing.T) {
	rh := NewRelayHub()
	c := ws.NewConn(nil, 0)
	rh.Register(1, c)

	if !rh.Send(1, map[string]string{"type": "test"}) {
		t.Error("Send to connected bot should return true")
	}
}

func TestRelayHub_MultipleBotsIndependent(t *testing.T) {
	rh := NewRelayHub()
	c1 := ws.NewConn(nil, 0)
	c2 := ws.NewConn(nil, 0)
	rh.Register(1, c1)
	rh.Register(2, c2)

	if rh.Online() != 2 {
		t.Errorf("expected 2 online, got %d", rh.Online())
	}

	rh.Unregister(1, c1)
	if rh.Online() != 1 {
		t.Errorf("expected 1 online after partial unregister, got %d", rh.Online())
	}
	if !rh.IsConnected(2) {
		t.Error("bot 2 should still be connected")
	}
}
