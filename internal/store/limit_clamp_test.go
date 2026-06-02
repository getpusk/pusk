// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package store

import "testing"

// TestChannelMessagesLimitClamp covers STORE-7/API-2: a negative LIMIT means
// "unbounded" in SQLite, so a negative/zero limit must be clamped rather than
// returning the entire channel history.
func TestChannelMessagesLimitClamp(t *testing.T) {
	s := newTestStore(t)
	bot, _ := s.CreateBot("tok-clamp", "B")
	ch, _ := s.CreateChannel(bot.ID, "clamp", "")
	for i := 0; i < 60; i++ {
		if _, err := s.SaveChannelMessage(ch.ID, "m", "", "", ""); err != nil {
			t.Fatal(err)
		}
	}

	for _, lim := range []int{-1, 0, -100} {
		msgs, err := s.ChannelMessages(ch.ID, lim)
		if err != nil {
			t.Fatal(err)
		}
		if len(msgs) > 50 {
			t.Errorf("ChannelMessages(limit=%d) returned %d rows; negative/zero must clamp, not run unbounded", lim, len(msgs))
		}
	}

	// A normal limit still works.
	if msgs, _ := s.ChannelMessages(ch.ID, 10); len(msgs) != 10 {
		t.Errorf("ChannelMessages(limit=10) = %d rows, want 10", len(msgs))
	}
}
