// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package notify

import "testing"

// TestNewPushServiceHasHTTPTimeout locks in the PERF-1/REL-3 fix: the push
// client must have a non-zero timeout so a hung provider cannot block the
// caller indefinitely.
func TestNewPushServiceHasHTTPTimeout(t *testing.T) {
	p := NewPushService("pub", "priv", "mailto:ops@example.com")
	if p.httpClient == nil {
		t.Fatal("httpClient is nil — sends would use webpush-go's zero-timeout default")
	}
	if p.httpClient.Timeout != pushTimeout {
		t.Errorf("httpClient.Timeout = %v, want %v", p.httpClient.Timeout, pushTimeout)
	}
}

// TestSendToUserNoConfigNoop verifies SendToUser is a cheap no-op when push is
// not configured (empty VAPID key), so the async fan-out goroutine costs
// nothing in that common case.
func TestSendToUserNoConfigNoop(t *testing.T) {
	p := NewPushService("", "", "")
	// Must not panic or block; store is unused because it returns before any
	// lookup when the VAPID public key is empty.
	p.SendToUser(nil, 1, PushPayload{Title: "x", Body: "y"})
}
