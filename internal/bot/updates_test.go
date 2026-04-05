// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package bot

import (
	"sync"
	"testing"
	"time"
)

func TestNewUpdateQueue(t *testing.T) {
	q := NewUpdateQueue()
	if q == nil {
		t.Fatal("expected non-nil queue")
	}
	if q.channels == nil {
		t.Fatal("expected non-nil channels map")
	}
	// Counter should be initialized to current unix timestamp
	val := q.counter.Load()
	now := time.Now().Unix()
	if val < now-2 || val > now+2 {
		t.Errorf("counter = %d, expected ~%d", val, now)
	}
}

func TestNextID_Monotonic(t *testing.T) {
	q := NewUpdateQueue()
	ids := make([]int64, 100)
	for i := range ids {
		ids[i] = q.nextID()
	}
	for i := 1; i < len(ids); i++ {
		if ids[i] <= ids[i-1] {
			t.Errorf("ID %d (%d) not > ID %d (%d)", i, ids[i], i-1, ids[i-1])
		}
	}
}

func TestPush_AssignsMonotonicID(t *testing.T) {
	q := NewUpdateQueue()
	q.Push(1, Update{Message: "first"})
	q.Push(1, Update{Message: "second"})

	ch := q.channels[int64(1)]
	u1 := <-ch
	u2 := <-ch
	if u1.UpdateID >= u2.UpdateID {
		t.Errorf("IDs not monotonic: %d >= %d", u1.UpdateID, u2.UpdateID)
	}
}

func TestPush_DifferentBots(t *testing.T) {
	q := NewUpdateQueue()
	q.Push(1, Update{Message: "bot1"})
	q.Push(2, Update{Message: "bot2"})

	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.channels) != 2 {
		t.Errorf("expected 2 channels, got %d", len(q.channels))
	}
}

func TestPush_DropsWhenFull(t *testing.T) {
	q := NewUpdateQueue()
	// Fill buffer (capacity is 100)
	for i := 0; i < 100; i++ {
		q.Push(1, Update{Message: "fill"})
	}
	// This should be dropped without panic
	q.Push(1, Update{Message: "overflow"})

	q.mu.Lock()
	ch := q.channels[int64(1)]
	q.mu.Unlock()
	if len(ch) != 100 {
		t.Errorf("expected 100 buffered, got %d", len(ch))
	}
}

func TestPoll_ReturnsBuffered(t *testing.T) {
	q := NewUpdateQueue()
	q.Push(1, Update{Message: "a"})
	q.Push(1, Update{Message: "b"})

	updates := q.Poll(1, 0, time.Second)
	if len(updates) != 2 {
		t.Fatalf("expected 2 updates, got %d", len(updates))
	}
	if updates[0].Message != "a" || updates[1].Message != "b" {
		t.Errorf("wrong order: %v, %v", updates[0].Message, updates[1].Message)
	}
}

func TestPoll_SkipsBelowOffset(t *testing.T) {
	q := NewUpdateQueue()
	q.Push(1, Update{Message: "old"})
	q.Push(1, Update{Message: "new"})

	// Drain to get IDs
	ch := q.channels[int64(1)]
	u1 := <-ch
	u2 := <-ch
	// Re-push
	ch <- u1
	ch <- u2

	// Poll with offset = first ID (should skip it)
	updates := q.Poll(1, u1.UpdateID, time.Second)
	if len(updates) != 1 {
		t.Fatalf("expected 1 update (skipped old), got %d", len(updates))
	}
	if updates[0].UpdateID != u2.UpdateID {
		t.Errorf("expected ID %d, got %d", u2.UpdateID, updates[0].UpdateID)
	}
}

func TestPoll_TimeoutReturnsEmpty(t *testing.T) {
	q := NewUpdateQueue()
	start := time.Now()
	updates := q.Poll(1, 0, 100*time.Millisecond)
	elapsed := time.Since(start)

	if len(updates) != 0 {
		t.Errorf("expected 0 updates on timeout, got %d", len(updates))
	}
	if elapsed < 80*time.Millisecond {
		t.Errorf("returned too fast: %v", elapsed)
	}
}

func TestPoll_WakesOnPush(t *testing.T) {
	q := NewUpdateQueue()
	var updates []Update
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		updates = q.Poll(1, 0, 5*time.Second)
	}()

	// Push after short delay
	time.Sleep(50 * time.Millisecond)
	q.Push(1, Update{Message: "wake"})

	wg.Wait()
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}
	if updates[0].Message != "wake" {
		t.Errorf("unexpected message: %v", updates[0].Message)
	}
}

func TestPoll_DifferentBotsIsolated(t *testing.T) {
	q := NewUpdateQueue()
	q.Push(1, Update{Message: "bot1"})
	q.Push(2, Update{Message: "bot2"})

	u1 := q.Poll(1, 0, time.Second)
	u2 := q.Poll(2, 0, time.Second)

	if len(u1) != 1 || u1[0].Message != "bot1" {
		t.Errorf("bot1 got: %v", u1)
	}
	if len(u2) != 1 || u2[0].Message != "bot2" {
		t.Errorf("bot2 got: %v", u2)
	}
}

func TestPoll_ConcurrentPushPoll(t *testing.T) {
	q := NewUpdateQueue()
	const n = 50
	var wg sync.WaitGroup

	// Push N updates concurrently
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			q.Push(1, Update{Message: idx})
		}(i)
	}
	wg.Wait()

	// Poll should get all N
	updates := q.Poll(1, 0, time.Second)
	if len(updates) != n {
		t.Errorf("expected %d updates, got %d", n, len(updates))
	}

	// All IDs should be unique
	seen := make(map[int64]bool)
	for _, u := range updates {
		if seen[u.UpdateID] {
			t.Errorf("duplicate update_id: %d", u.UpdateID)
		}
		seen[u.UpdateID] = true
	}
}
