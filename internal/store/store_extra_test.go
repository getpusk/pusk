// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package store

import (
	"testing"
	"time"
)

// ── Bots ──

func TestListBots_Empty(t *testing.T) {
	s := newTestStore(t)
	bots, err := s.ListBots()
	if err != nil {
		t.Fatal(err)
	}
	if len(bots) != 0 {
		t.Errorf("expected 0 bots, got %d", len(bots))
	}
}

func TestListBots_Multiple(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.CreateBot("token1", "Bot A"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateBot("token2", "Bot B"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateBot("token3", "Bot C"); err != nil {
		t.Fatal(err)
	}

	bots, err := s.ListBots()
	if err != nil {
		t.Fatal(err)
	}
	if len(bots) != 3 {
		t.Errorf("expected 3 bots, got %d", len(bots))
	}
	for _, b := range bots {
		if b.ID == 0 || b.Name == "" || b.Token == "" {
			t.Errorf("bot has empty fields: %+v", b)
		}
	}
}

func TestRenameBot_Success(t *testing.T) {
	s := newTestStore(t)
	bot, err := s.CreateBot("tok-rename", "OldName")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RenameBot(bot.ID, "NewName"); err != nil {
		t.Fatal(err)
	}
	got, err := s.BotByID(bot.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "NewName" {
		t.Errorf("name = %q, want NewName", got.Name)
	}
}

func TestBotByID_Success(t *testing.T) {
	s := newTestStore(t)
	created, err := s.CreateBot("tok-byid", "ByIDBot")
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.BotByID(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Token != "tok-byid" {
		t.Errorf("token = %q, want tok-byid", got.Token)
	}
}

func TestBotByID_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.BotByID(999)
	if err == nil {
		t.Error("expected error for nonexistent bot ID")
	}
}

// ── Channels ──

func TestListChannels_Empty(t *testing.T) {
	s := newTestStore(t)
	chs, err := s.ListChannels()
	if err != nil {
		t.Fatal(err)
	}
	if len(chs) != 0 {
		t.Errorf("expected 0 channels, got %d", len(chs))
	}
}

func TestListChannels_Multiple(t *testing.T) {
	s := newTestStore(t)
	bot, err := s.CreateBot("tok-ch", "TestBot")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateChannel(bot.ID, "general", "Main channel"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateChannel(bot.ID, "alerts", "Alert channel"); err != nil {
		t.Fatal(err)
	}

	chs, err := s.ListChannels()
	if err != nil {
		t.Fatal(err)
	}
	if len(chs) != 2 {
		t.Errorf("expected 2 channels, got %d", len(chs))
	}
}

func TestRenameChannel_Success(t *testing.T) {
	s := newTestStore(t)
	bot, err := s.CreateBot("tok-rch", "TestBot")
	if err != nil {
		t.Fatal(err)
	}
	ch, err := s.CreateChannel(bot.ID, "old-name", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RenameChannel(ch.ID, "new-name"); err != nil {
		t.Fatal(err)
	}
	got, err := s.ChannelByID(ch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "new-name" {
		t.Errorf("name = %q, want new-name", got.Name)
	}
}

func TestSetChannelMessageReplyTo(t *testing.T) {
	s := newTestStore(t)
	bot, err := s.CreateBot("tok-reply", "TestBot")
	if err != nil {
		t.Fatal(err)
	}
	ch, err := s.CreateChannel(bot.ID, "general", "")
	if err != nil {
		t.Fatal(err)
	}
	msg, err := s.SaveChannelMessage(ch.ID, "hello", "", "", "")
	if err != nil {
		t.Fatal(err)
	}

	s.SetChannelMessageReplyTo(msg.ID, 42)

	var replyTo int64
	if err := s.DB().QueryRow("SELECT COALESCE(reply_to,0) FROM channel_messages WHERE id=?", msg.ID).Scan(&replyTo); err != nil {
		t.Fatal(err)
	}
	if replyTo != 42 {
		t.Errorf("reply_to = %d, want 42", replyTo)
	}
}

func TestCleanOldChannelMessages(t *testing.T) {
	s := newTestStore(t)
	bot, err := s.CreateBot("tok-clean", "TestBot")
	if err != nil {
		t.Fatal(err)
	}
	ch, err := s.CreateChannel(bot.ID, "general", "")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.SaveChannelMessage(ch.ID, "old message", "", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveChannelMessage(ch.ID, "new message", "", "", ""); err != nil {
		t.Fatal(err)
	}

	future := time.Now().Add(time.Hour).Format(time.RFC3339)
	s.CleanOldChannelMessages(future)

	msgs, err := s.ChannelMessages(ch.ID, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages after cleanup, got %d", len(msgs))
	}
}

func TestCleanOldChannelMessages_KeepsRecent(t *testing.T) {
	s := newTestStore(t)
	bot, err := s.CreateBot("tok-keep", "TestBot")
	if err != nil {
		t.Fatal(err)
	}
	ch, err := s.CreateChannel(bot.ID, "general", "")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.SaveChannelMessage(ch.ID, "recent", "", "", ""); err != nil {
		t.Fatal(err)
	}

	past := time.Now().Add(-time.Hour).Format(time.RFC3339)
	s.CleanOldChannelMessages(past)

	msgs, err := s.ChannelMessages(ch.ID, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Errorf("expected 1 message kept, got %d", len(msgs))
	}
}

// ── Chats ──

func TestUserChats_Empty(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.CreateUser("chatuser", "pin123", "ChatUser"); err != nil {
		t.Fatal(err)
	}
	chats, err := s.UserChats(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 0 {
		t.Errorf("expected 0 chats, got %d", len(chats))
	}
}

func TestUserChats_Multiple(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.CreateUser("chatuser2", "pin123", "ChatUser"); err != nil {
		t.Fatal(err)
	}
	b1, err := s.CreateBot("tok-c1", "Bot1")
	if err != nil {
		t.Fatal(err)
	}
	b2, err := s.CreateBot("tok-c2", "Bot2")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetOrCreateChat(1, b1.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetOrCreateChat(1, b2.ID); err != nil {
		t.Fatal(err)
	}

	chats, err := s.UserChats(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 2 {
		t.Errorf("expected 2 chats, got %d", len(chats))
	}
}

func TestChatUserID(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.CreateUser("uid-user", "pin123", "UIDUser"); err != nil {
		t.Fatal(err)
	}
	bot, err := s.CreateBot("tok-uid", "Bot")
	if err != nil {
		t.Fatal(err)
	}
	chat, err := s.GetOrCreateChat(1, bot.ID)
	if err != nil {
		t.Fatal(err)
	}

	uid, err := s.ChatUserID(chat.ID)
	if err != nil {
		t.Fatal(err)
	}
	if uid != 1 {
		t.Errorf("user_id = %d, want 1", uid)
	}
}

func TestChatBotID(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.CreateUser("bid-user", "pin123", "BIDUser"); err != nil {
		t.Fatal(err)
	}
	bot, err := s.CreateBot("tok-bid", "Bot")
	if err != nil {
		t.Fatal(err)
	}
	chat, err := s.GetOrCreateChat(1, bot.ID)
	if err != nil {
		t.Fatal(err)
	}

	bid, err := s.ChatBotID(chat.ID)
	if err != nil {
		t.Fatal(err)
	}
	if bid != bot.ID {
		t.Errorf("bot_id = %d, want %d", bid, bot.ID)
	}
}

// ── Invites ──

func TestActiveInvite_Exists(t *testing.T) {
	s := newTestStore(t)
	code := "test-invite-code"
	if err := s.CreateInvite(code, 24*time.Hour); err != nil {
		t.Fatal(err)
	}

	active, err := s.ActiveInvite()
	if err != nil {
		t.Fatal(err)
	}
	if active != code {
		t.Errorf("active = %q, want %q", active, code)
	}
}

func TestActiveInvite_None(t *testing.T) {
	s := newTestStore(t)
	active, _ := s.ActiveInvite()
	if active != "" {
		t.Errorf("expected empty, got %q", active)
	}
}

// ── Messages ──

func TestMessageCount(t *testing.T) {
	s := newTestStore(t)
	bot, err := s.CreateBot("tok-mc", "Bot")
	if err != nil {
		t.Fatal(err)
	}
	ch, err := s.CreateChannel(bot.ID, "general", "")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.SaveChannelMessage(ch.ID, "msg1", "", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveChannelMessage(ch.ID, "msg2", "", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveChannelMessage(ch.ID, "msg3", "", "", ""); err != nil {
		t.Fatal(err)
	}

	count := s.MessageCount()
	if count != 3 {
		t.Errorf("message count = %d, want 3", count)
	}
}

func TestMessageCount_Empty(t *testing.T) {
	s := newTestStore(t)

	count := s.MessageCount()
	if count != 0 {
		t.Errorf("message count = %d, want 0", count)
	}
}

// ── Files ──

func TestCleanExpiredFileTokens(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.CreateUser("ft-user", "pin", "User"); err != nil {
		t.Fatal(err)
	}
	bot, err := s.CreateBot("tok-ft", "Bot")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveFile("file123", bot.ID, "test.txt", "text/plain", 100, "/tmp/test.txt"); err != nil {
		t.Fatal(err)
	}

	if err := s.CreateFileToken("testtoken123", 1, time.Hour); err != nil {
		t.Fatal(err)
	}

	// Should not panic — cleans expired tokens (ours is fresh, won't be deleted)
	s.CleanExpiredFileTokens()
}
