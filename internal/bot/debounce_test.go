// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package bot

import (
	"sync"
	"testing"
	"time"
)

// testDebouncer creates a Debouncer without the cleanup goroutine
// to avoid goleak failures. Use NewDebouncer only when testing cleanup.
func testDebouncer(window time.Duration) *Debouncer {
	return &Debouncer{seen: make(map[string]time.Time), window: window}
}

func TestIsDuplicate_FirstCallFalse(t *testing.T) {
	d := testDebouncer(time.Second)
	if d.IsDuplicate([]byte("hello")) {
		t.Error("first call should not be duplicate")
	}
}

func TestIsDuplicate_SecondCallTrue(t *testing.T) {
	d := testDebouncer(time.Second)
	data := []byte(`{"alertname":"disk_full","instance":"web-01"}`)
	d.IsDuplicate(data) // first
	if !d.IsDuplicate(data) {
		t.Error("identical payload within window should be duplicate")
	}
}

func TestIsDuplicate_DifferentPayloadsNotDuplicate(t *testing.T) {
	d := testDebouncer(time.Second)
	d.IsDuplicate([]byte("alert1"))
	if d.IsDuplicate([]byte("alert2")) {
		t.Error("different payloads should not be duplicates")
	}
}

func TestIsDuplicate_AfterWindowExpires(t *testing.T) {
	d := testDebouncer(50 * time.Millisecond)
	data := []byte("test-payload")
	d.IsDuplicate(data)
	time.Sleep(80 * time.Millisecond)
	if d.IsDuplicate(data) {
		t.Error("payload should not be duplicate after window expires")
	}
}

func TestIsDuplicate_Concurrent(t *testing.T) {
	d := testDebouncer(time.Second)
	var wg sync.WaitGroup
	dupes := make([]bool, 100)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			dupes[idx] = d.IsDuplicate([]byte("same"))
		}(i)
	}
	wg.Wait()
	falseCount := 0
	for _, dup := range dupes {
		if !dup {
			falseCount++
		}
	}
	if falseCount != 1 {
		t.Errorf("expected exactly 1 non-duplicate, got %d", falseCount)
	}
}

func TestIsDuplicate_EmptyPayload(t *testing.T) {
	d := testDebouncer(time.Second)
	if d.IsDuplicate([]byte{}) {
		t.Error("first empty payload should not be duplicate")
	}
	if !d.IsDuplicate([]byte{}) {
		t.Error("second empty payload should be duplicate")
	}
}

func TestIsDuplicate_HashCollisionSafety(t *testing.T) {
	d := testDebouncer(time.Second)
	// Different payloads should produce different hashes
	d.IsDuplicate([]byte("payload_a"))
	if d.IsDuplicate([]byte("payload_b")) {
		t.Error("different payloads must not collide")
	}
}
