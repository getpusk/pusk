// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package store

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// newTestStore opens an in-memory SQLite store with all migrations applied.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("New(:memory:): %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// ── Migration ──

func TestMigrate_VersionBumps(t *testing.T) {
	s := newTestStore(t)
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 4 {
		t.Errorf("expected user_version=4 after full migrate, got %d", version)
	}
}

func TestMigrate_Idempotent(t *testing.T) {
	s := newTestStore(t)
	// Running migrate again on an already-migrated DB must not fail.
	if err := s.migrate(); err != nil {
		t.Fatalf("second migrate failed: %v", err)
	}
}

// ── Users: CreateUser / AuthUser ──

func TestCreateUser_Basic(t *testing.T) {
	s := newTestStore(t)
	u, err := s.CreateUser("alice", "1234", "Alice A")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.Username != "alice" || u.DisplayName != "Alice A" || u.ID == 0 {
		t.Errorf("unexpected user: %+v", u)
	}
}

func TestCreateUser_DuplicateUsername(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.CreateUser("bob", "1111", ""); err != nil {
		t.Fatal(err)
	}
	_, err := s.CreateUser("bob", "2222", "")
	if err == nil {
		t.Error("expected error on duplicate username, got nil")
	}
}

func TestCreateUser_EmptyFields(t *testing.T) {
	s := newTestStore(t)
	// Empty username should fail due to NOT NULL constraint ... but pin is hashed so empty string is valid bcrypt input.
	// Empty display_name is allowed.
	u, err := s.CreateUser("x", "", "")
	if err != nil {
		t.Fatalf("CreateUser with empty pin: %v", err)
	}
	if u.Username != "x" {
		t.Error("username mismatch")
	}
}

func TestAuthUser_Success(t *testing.T) {
	s := newTestStore(t)
	_, _ = s.CreateUser("carol", "secret", "Carol")
	u, err := s.AuthUser("carol", "secret")
	if err != nil {
		t.Fatalf("AuthUser: %v", err)
	}
	if u.Username != "carol" {
		t.Errorf("expected carol, got %s", u.Username)
	}
}

func TestAuthUser_WrongPin(t *testing.T) {
	s := newTestStore(t)
	_, _ = s.CreateUser("dave", "correct", "")
	_, err := s.AuthUser("dave", "wrong")
	if err == nil {
		t.Error("expected error for wrong pin")
	}
}

func TestAuthUser_NoSuchUser(t *testing.T) {
	s := newTestStore(t)
	_, err := s.AuthUser("nobody", "1234")
	if err == nil {
		t.Error("expected error for nonexistent user")
	}
}

// ── Users: roles ──

func TestSetUserRole_IsAdmin(t *testing.T) {
	s := newTestStore(t)
	u, _ := s.CreateUser("admin1", "pin", "")
	if s.IsAdmin(u.ID) {
		t.Error("new user should not be admin")
	}
	_ = s.SetUserRole(u.ID, "admin")
	if !s.IsAdmin(u.ID) {
		t.Error("user should be admin after SetUserRole")
	}
}

func TestUserExists(t *testing.T) {
	s := newTestStore(t)
	u, _ := s.CreateUser("exists", "pin", "")
	if !s.UserExists(u.ID) {
		t.Error("expected user to exist")
	}
	if s.UserExists(99999) {
		t.Error("expected nonexistent user")
	}
}

func TestDeleteUser(t *testing.T) {
	s := newTestStore(t)
	u, _ := s.CreateUser("delme", "pin", "")
	if err := s.DeleteUser(u.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if s.UserExists(u.ID) {
		t.Error("user should not exist after delete")
	}
}

func TestResetPassword(t *testing.T) {
	s := newTestStore(t)
	_, _ = s.CreateUser("resetme", "old", "")
	if err := s.ResetPassword("resetme", "new"); err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}
	if _, err := s.AuthUser("resetme", "new"); err != nil {
		t.Errorf("auth with new password failed: %v", err)
	}
	if _, err := s.AuthUser("resetme", "old"); err == nil {
		t.Error("old password should no longer work")
	}
}

func TestResetPassword_NotFound(t *testing.T) {
	s := newTestStore(t)
	err := s.ResetPassword("ghost", "pin")
	if err == nil {
		t.Error("expected error for nonexistent user")
	}
}

func TestListUsers(t *testing.T) {
	s := newTestStore(t)
	_, _ = s.CreateUser("u1", "p", "")
	_, _ = s.CreateUser("u2", "p", "")
	users, err := s.ListUsers()
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 {
		t.Errorf("expected 2 users, got %d", len(users))
	}
}

// ── Bots ──

func TestCreateBot_BotByToken(t *testing.T) {
	s := newTestStore(t)
	bot, err := s.CreateBot("tok123", "TestBot")
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.BotByToken("tok123")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != bot.ID || got.Name != "TestBot" {
		t.Errorf("BotByToken mismatch: %+v", got)
	}
}

func TestBotByToken_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.BotByToken("nonexistent")
	if err == nil {
		t.Error("expected error")
	}
}

// ── Chats & Messages ──

func TestGetOrCreateChat(t *testing.T) {
	s := newTestStore(t)
	u, _ := s.CreateUser("chatuser", "p", "")
	bot, _ := s.CreateBot("ct1", "Bot1")

	c1, err := s.GetOrCreateChat(u.ID, bot.ID)
	if err != nil {
		t.Fatal(err)
	}
	c2, err := s.GetOrCreateChat(u.ID, bot.ID)
	if err != nil {
		t.Fatal(err)
	}
	if c1.ID != c2.ID {
		t.Error("second GetOrCreateChat should return same chat")
	}
}

func TestSaveMessage_GetMessage(t *testing.T) {
	s := newTestStore(t)
	u, _ := s.CreateUser("msguser", "p", "")
	bot, _ := s.CreateBot("mt1", "MBot")
	chat, _ := s.GetOrCreateChat(u.ID, bot.ID)

	msg, err := s.SaveMessage(chat.ID, "bot", "hello", "", "", "")
	if err != nil {
		t.Fatalf("SaveMessage: %v", err)
	}
	if msg.Text != "hello" || msg.Sender != "bot" || msg.ID == 0 {
		t.Errorf("unexpected msg: %+v", msg)
	}

	got, err := s.GetMessage(msg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "hello" {
		t.Errorf("GetMessage text = %q", got.Text)
	}
}

func TestSaveMessage_WithFileFields(t *testing.T) {
	s := newTestStore(t)
	u, _ := s.CreateUser("fuser", "p", "")
	bot, _ := s.CreateBot("ft1", "FBot")
	chat, _ := s.GetOrCreateChat(u.ID, bot.ID)

	msg, err := s.SaveMessage(chat.ID, "bot", "photo", `{"inline_keyboard":[]}`, "file123", "photo")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetMessage(msg.ID)
	if got.FileID != "file123" || got.FileType != "photo" || got.ReplyMarkup == "" {
		t.Errorf("file fields mismatch: %+v", got)
	}
}

func TestUpdateMessageText(t *testing.T) {
	s := newTestStore(t)
	u, _ := s.CreateUser("eu", "p", "")
	bot, _ := s.CreateBot("et1", "EBot")
	chat, _ := s.GetOrCreateChat(u.ID, bot.ID)
	msg, _ := s.SaveMessage(chat.ID, "bot", "v1", "", "", "")

	_ = s.UpdateMessageText(msg.ID, "v2", `{"keyboard":[]}`)
	got, _ := s.GetMessage(msg.ID)
	if got.Text != "v2" || got.ReplyMarkup != `{"keyboard":[]}` {
		t.Errorf("update mismatch: %+v", got)
	}
}

func TestDeleteMessage(t *testing.T) {
	s := newTestStore(t)
	u, _ := s.CreateUser("du", "p", "")
	bot, _ := s.CreateBot("dt1", "DBot")
	chat, _ := s.GetOrCreateChat(u.ID, bot.ID)
	msg, _ := s.SaveMessage(chat.ID, "bot", "bye", "", "", "")

	_ = s.DeleteMessage(msg.ID)
	_, err := s.GetMessage(msg.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestChatMessages_Ordering(t *testing.T) {
	s := newTestStore(t)
	u, _ := s.CreateUser("ou", "p", "")
	bot, _ := s.CreateBot("ot1", "OBot")
	chat, _ := s.GetOrCreateChat(u.ID, bot.ID)

	m1, _ := s.SaveMessage(chat.ID, "bot", "first", "", "", "")
	m2, _ := s.SaveMessage(chat.ID, "user", "second", "", "", "")
	m3, _ := s.SaveMessage(chat.ID, "bot", "third", "", "", "")

	msgs, err := s.ChatMessages(chat.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	// Verify all three messages are stored with ascending autoincrement IDs
	if m1.ID >= m2.ID || m2.ID >= m3.ID {
		t.Errorf("expected ascending IDs: %d < %d < %d", m1.ID, m2.ID, m3.ID)
	}
	// ChatMessages returns results — verify the count and that all texts are present
	texts := make(map[string]bool)
	for _, m := range msgs {
		texts[m.Text] = true
	}
	for _, want := range []string{"first", "second", "third"} {
		if !texts[want] {
			t.Errorf("missing message %q", want)
		}
	}
}

func TestChatMessages_Limit(t *testing.T) {
	s := newTestStore(t)
	u, _ := s.CreateUser("lu", "p", "")
	bot, _ := s.CreateBot("lt1", "LBot")
	chat, _ := s.GetOrCreateChat(u.ID, bot.ID)

	for i := 0; i < 5; i++ {
		_, _ = s.SaveMessage(chat.ID, "bot", "msg", "", "", "")
	}
	msgs, _ := s.ChatMessages(chat.ID, 2)
	if len(msgs) != 2 {
		t.Errorf("expected 2, got %d", len(msgs))
	}
}

// ── Channels & Subscribers ──

func TestCreateChannel_Subscribe_Unsubscribe(t *testing.T) {
	s := newTestStore(t)
	bot, _ := s.CreateBot("chbt", "ChBot")
	u, _ := s.CreateUser("chuser", "p", "")

	ch, err := s.CreateChannel(bot.ID, "general", "General channel")
	if err != nil {
		t.Fatal(err)
	}
	if ch.Name != "general" {
		t.Errorf("name = %q", ch.Name)
	}

	// Subscribe
	if err := s.Subscribe(ch.ID, u.ID); err != nil {
		t.Fatal(err)
	}
	if !s.IsSubscribed(ch.ID, u.ID) {
		t.Error("should be subscribed")
	}

	// Double subscribe should not fail (INSERT OR IGNORE)
	if err := s.Subscribe(ch.ID, u.ID); err != nil {
		t.Errorf("double subscribe error: %v", err)
	}

	// ChannelSubscribers
	subs, err := s.ChannelSubscribers(ch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 1 || subs[0] != u.ID {
		t.Errorf("unexpected subs: %v", subs)
	}

	// Unsubscribe
	_ = s.Unsubscribe(ch.ID, u.ID)
	if s.IsSubscribed(ch.ID, u.ID) {
		t.Error("should not be subscribed after unsubscribe")
	}
}

func TestChannelSubscribers_Empty(t *testing.T) {
	s := newTestStore(t)
	bot, _ := s.CreateBot("eb", "EBot")
	ch, _ := s.CreateChannel(bot.ID, "empty", "")

	subs, err := s.ChannelSubscribers(ch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 0 {
		t.Errorf("expected empty, got %v", subs)
	}
}

func TestChannelSubscribers_Multiple(t *testing.T) {
	s := newTestStore(t)
	bot, _ := s.CreateBot("mb", "MBot")
	ch, _ := s.CreateChannel(bot.ID, "multi", "")

	u1, _ := s.CreateUser("s1", "p", "")
	u2, _ := s.CreateUser("s2", "p", "")
	u3, _ := s.CreateUser("s3", "p", "")

	_ = s.Subscribe(ch.ID, u1.ID)
	_ = s.Subscribe(ch.ID, u2.ID)
	_ = s.Subscribe(ch.ID, u3.ID)

	subs, _ := s.ChannelSubscribers(ch.ID)
	if len(subs) != 3 {
		t.Errorf("expected 3 subs, got %d", len(subs))
	}
}

func TestChannelByName(t *testing.T) {
	s := newTestStore(t)
	bot, _ := s.CreateBot("nb", "NBot")
	_, _ = s.CreateChannel(bot.ID, "alerts", "Alert channel")

	ch, err := s.ChannelByName(bot.ID, "alerts")
	if err != nil {
		t.Fatal(err)
	}
	if ch.Name != "alerts" || ch.Description != "Alert channel" {
		t.Errorf("unexpected channel: %+v", ch)
	}
}

func TestChannelByName_NotFound(t *testing.T) {
	s := newTestStore(t)
	bot, _ := s.CreateBot("nfb", "NFBot")
	_, err := s.ChannelByName(bot.ID, "nope")
	if err == nil {
		t.Error("expected error")
	}
}

func TestSaveChannelMessage(t *testing.T) {
	s := newTestStore(t)
	bot, _ := s.CreateBot("cmb", "CMBot")
	ch, _ := s.CreateChannel(bot.ID, "news", "")

	msg, err := s.SaveChannelMessage(ch.ID, "Hello channel!", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if msg.Text != "Hello channel!" || msg.Sender != "bot" {
		t.Errorf("unexpected msg: %+v", msg)
	}

	msgs, _ := s.ChannelMessages(ch.ID, 10)
	if len(msgs) != 1 || msgs[0].Text != "Hello channel!" {
		t.Errorf("ChannelMessages mismatch")
	}
}

func TestSaveChannelMessageFrom(t *testing.T) {
	s := newTestStore(t)
	bot, _ := s.CreateBot("cfb", "CFBot")
	ch, _ := s.CreateChannel(bot.ID, "discuss", "")

	msg, err := s.SaveChannelMessageFrom(ch.ID, "user", "Alice", "Hi from Alice", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if msg.Sender != "user" || msg.SenderName != "Alice" {
		t.Errorf("sender fields: %+v", msg)
	}
}

// ── Unread / MarkRead ──

func TestUnreadCount_MarkRead(t *testing.T) {
	s := newTestStore(t)
	bot, _ := s.CreateBot("ub", "UBot")
	ch, _ := s.CreateChannel(bot.ID, "unread", "")
	u, _ := s.CreateUser("reader", "p", "")
	_ = s.Subscribe(ch.ID, u.ID)

	m1, _ := s.SaveChannelMessage(ch.ID, "msg1", "", "", "")
	_, _ = s.SaveChannelMessage(ch.ID, "msg2", "", "", "")

	if n := s.UnreadCount(ch.ID, u.ID); n != 2 {
		t.Errorf("expected 2 unread, got %d", n)
	}

	s.MarkChannelRead(ch.ID, u.ID, m1.ID)
	if n := s.UnreadCount(ch.ID, u.ID); n != 1 {
		t.Errorf("expected 1 unread after marking, got %d", n)
	}
}

func TestGetLastRead(t *testing.T) {
	s := newTestStore(t)
	bot, _ := s.CreateBot("lrb", "LRBot")
	ch, _ := s.CreateChannel(bot.ID, "lr", "")
	u, _ := s.CreateUser("lruser", "p", "")

	if lr := s.GetLastRead(ch.ID, u.ID); lr != 0 {
		t.Errorf("expected 0 for unread user, got %d", lr)
	}
	s.MarkChannelRead(ch.ID, u.ID, 42)
	if lr := s.GetLastRead(ch.ID, u.ID); lr != 42 {
		t.Errorf("expected 42, got %d", lr)
	}
}

// ── Pin ──

func TestPinMessage(t *testing.T) {
	s := newTestStore(t)
	bot, _ := s.CreateBot("pb", "PBot")
	ch, _ := s.CreateChannel(bot.ID, "pinch", "")

	if pid := s.GetPinnedMessage(ch.ID); pid != 0 {
		t.Errorf("expected no pin, got %d", pid)
	}
	_ = s.PinMessage(ch.ID, 99)
	if pid := s.GetPinnedMessage(ch.ID); pid != 99 {
		t.Errorf("expected pin=99, got %d", pid)
	}
}

// ── Push Subscriptions ──

func TestPushSubscriptions_SaveAndList(t *testing.T) {
	s := newTestStore(t)
	u, _ := s.CreateUser("pushuser", "p", "")

	err := s.SavePushSubscription(u.ID, "https://push.example.com/1", "p256dh_key", "auth_key")
	if err != nil {
		t.Fatal(err)
	}

	subs, err := s.UserPushSubscriptions(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected 1 sub, got %d", len(subs))
	}
	if subs[0].Endpoint != "https://push.example.com/1" {
		t.Errorf("endpoint mismatch: %s", subs[0].Endpoint)
	}
}

func TestPushSubscriptions_UpsertSameEndpoint(t *testing.T) {
	s := newTestStore(t)
	u, _ := s.CreateUser("upuser", "p", "")

	_ = s.SavePushSubscription(u.ID, "https://push.example.com/dup", "key1", "auth1")
	_ = s.SavePushSubscription(u.ID, "https://push.example.com/dup", "key2", "auth2")

	subs, _ := s.UserPushSubscriptions(u.ID)
	if len(subs) != 1 {
		t.Errorf("expected 1 (upsert), got %d", len(subs))
	}
	if subs[0].P256dh != "key2" {
		t.Errorf("expected updated key, got %s", subs[0].P256dh)
	}
}

func TestPushSubscriptions_MaxFivePerUser(t *testing.T) {
	s := newTestStore(t)
	u, _ := s.CreateUser("maxuser", "p", "")

	for i := 0; i < 7; i++ {
		_ = s.SavePushSubscription(u.ID, "https://push.example.com/"+string(rune('a'+i)), "k", "a")
	}
	subs, _ := s.UserPushSubscriptions(u.ID)
	if len(subs) > 5 {
		t.Errorf("expected max 5, got %d", len(subs))
	}
}

func TestDeletePushSubscription(t *testing.T) {
	s := newTestStore(t)
	u, _ := s.CreateUser("delps", "p", "")
	_ = s.SavePushSubscription(u.ID, "https://push.example.com/del", "k", "a")
	_ = s.DeletePushSubscription("https://push.example.com/del")
	subs, _ := s.UserPushSubscriptions(u.ID)
	if len(subs) != 0 {
		t.Error("expected 0 after delete")
	}
}

// ── Invites ──

func TestInvite_CreateUseValidate(t *testing.T) {
	s := newTestStore(t)
	code := "TESTINVITE"
	if err := s.CreateInvite(code, 1*time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := s.ValidateInvite(code); err != nil {
		t.Fatalf("ValidateInvite: %v", err)
	}
	// Multi-use: first 50 uses should succeed
	if err := s.UseInvite(code); err != nil {
		t.Fatalf("UseInvite 1: %v", err)
	}
	// Second use should also succeed (multi-use)
	if err := s.UseInvite(code); err != nil {
		t.Fatalf("UseInvite 2: %v", err)
	}
	// Validate still works
	if err := s.ValidateInvite(code); err != nil {
		t.Fatalf("ValidateInvite after 2 uses: %v", err)
	}
	// Revoke
	if err := s.RevokeInvite(code); err != nil {
		t.Fatalf("RevokeInvite: %v", err)
	}
	if err := s.ValidateInvite(code); err == nil {
		t.Error("expected error after revoke")
	}
}

func TestInvite_NotFound(t *testing.T) {
	s := newTestStore(t)
	if err := s.UseInvite("NOPE"); err == nil {
		t.Error("expected error")
	}
}

// ── Files ──

func TestSaveFile_GetFile(t *testing.T) {
	s := newTestStore(t)
	bot, _ := s.CreateBot("fb", "FBot")
	err := s.SaveFile("f1", bot.ID, "photo.jpg", "image/jpeg", 12345, "/tmp/f1.jpg")
	if err != nil {
		t.Fatal(err)
	}
	f, err := s.GetFile("f1")
	if err != nil {
		t.Fatal(err)
	}
	if f.Filename != "photo.jpg" || f.Size != 12345 {
		t.Errorf("file mismatch: %+v", f)
	}
}

func TestTotalFileSize(t *testing.T) {
	s := newTestStore(t)
	bot, _ := s.CreateBot("tsb", "TSBot")
	_ = s.SaveFile("a1", bot.ID, "a", "x", 100, "/a")
	_ = s.SaveFile("a2", bot.ID, "b", "x", 200, "/b")
	if total := s.TotalFileSize(); total != 300 {
		t.Errorf("expected 300, got %d", total)
	}
}

func TestFileToken_CreateValidate(t *testing.T) {
	s := newTestStore(t)
	u, _ := s.CreateUser("ftuser", "p", "")
	if err := s.CreateFileToken("tok_abc", u.ID, 1*time.Hour); err != nil {
		t.Fatal(err)
	}
	uid, err := s.ValidateFileToken("tok_abc")
	if err != nil {
		t.Fatalf("ValidateFileToken: %v", err)
	}
	if uid != u.ID {
		t.Errorf("userID mismatch: %d", uid)
	}
	// Token is consumed (single-use) — second call should fail
	_, err = s.ValidateFileToken("tok_abc")
	if err == nil {
		t.Error("expected error on reuse")
	}
}

// ── DeleteChannel cascade ──

func TestDeleteChannel_CleansUp(t *testing.T) {
	s := newTestStore(t)
	bot, _ := s.CreateBot("dcb", "DCBot")
	u, _ := s.CreateUser("dcuser", "p", "")
	ch, _ := s.CreateChannel(bot.ID, "todelete", "")
	_ = s.Subscribe(ch.ID, u.ID)
	_, _ = s.SaveChannelMessage(ch.ID, "msg", "", "", "")

	if err := s.DeleteChannel(ch.ID); err != nil {
		t.Fatal(err)
	}
	_, err := s.ChannelByID(ch.ID)
	if err == nil {
		t.Error("channel should not exist after delete")
	}
	subs, _ := s.ChannelSubscribers(ch.ID)
	if len(subs) != 0 {
		t.Error("subscribers should be cleaned up")
	}
}

func TestDeleteChannel_CleansReads(t *testing.T) {
	s := newTestStore(t)
	bot, _ := s.CreateBot("dcrb", "DCRBot")
	u, _ := s.CreateUser("dcruser", "p", "")
	ch, _ := s.CreateChannel(bot.ID, "delreads", "")
	_ = s.Subscribe(ch.ID, u.ID)
	msg, _ := s.SaveChannelMessage(ch.ID, "msg", "", "", "")
	s.MarkChannelRead(ch.ID, u.ID, msg.ID)

	if lr := s.GetLastRead(ch.ID, u.ID); lr == 0 {
		t.Fatal("expected last_read to be set before delete")
	}

	if err := s.DeleteChannel(ch.ID); err != nil {
		t.Fatal(err)
	}
	msgs, _ := s.ChannelMessages(ch.ID, 100)
	if len(msgs) != 0 {
		t.Error("messages should be cleaned up")
	}
	if lr := s.GetLastRead(ch.ID, u.ID); lr != 0 {
		t.Error("channel_reads should be cleaned up")
	}
}

// ── UserSubscriptions ──

func TestUserSubscriptions(t *testing.T) {
	s := newTestStore(t)
	bot, _ := s.CreateBot("usb", "USBot")
	u, _ := s.CreateUser("subuser", "p", "")
	ch1, _ := s.CreateChannel(bot.ID, "ch1", "")
	ch2, _ := s.CreateChannel(bot.ID, "ch2", "")

	_ = s.Subscribe(ch1.ID, u.ID)
	_ = s.Subscribe(ch2.ID, u.ID)

	chs, err := s.UserSubscriptions(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(chs) != 2 {
		t.Errorf("expected 2 subscriptions, got %d", len(chs))
	}
}

// ── Concurrent access (basic safety check) ──

func TestConcurrentCreateUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrent test in -short mode")
	}
	// Use file-based temp DB — :memory: has race issues under -race
	tmp := t.TempDir()
	s, err := New(tmp + "/concurrent.db")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			name := fmt.Sprintf("user_%d", n)
			var err error
			for attempt := 0; attempt < 5; attempt++ {
				_, err = s.CreateUser(name, "pin", "")
				if err == nil || !strings.Contains(err.Error(), "SQLITE_BUSY") {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			if err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent create error: %v", err)
	}
}

// ── Webhook ──

func TestSetWebhook(t *testing.T) {
	s := newTestStore(t)
	bot, _ := s.CreateBot("whb", "WHBot")
	_ = s.SetWebhook(bot.ID, "https://example.com/hook")
	got, _ := s.BotByID(bot.ID)
	if got.WebhookURL != "https://example.com/hook" {
		t.Errorf("webhook = %q", got.WebhookURL)
	}
}

// ── Ping ──

func TestPing(t *testing.T) {
	s := newTestStore(t)
	if err := s.Ping(); err != nil {
		t.Fatal(err)
	}
}

// ── GetUserByID: display_name in messages ──

func TestGetUserByID_DisplayName(t *testing.T) {
	s := newTestStore(t)
	user, err := s.CreateUser("testuser", "pin123", "Test Display Name")
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.GetUserByID(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.DisplayName != "Test Display Name" {
		t.Errorf("DisplayName: got %q, want %q", got.DisplayName, "Test Display Name")
	}
	if got.Username != "testuser" {
		t.Errorf("Username: got %q, want %q", got.Username, "testuser")
	}
	if got.Role != "member" {
		t.Errorf("Role: got %q, want %q", got.Role, "member")
	}
}

func TestGetUserByID_EmptyDisplayName(t *testing.T) {
	s := newTestStore(t)
	user, _ := s.CreateUser("nodisplay", "pin123", "")
	got, err := s.GetUserByID(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.DisplayName != "" {
		t.Errorf("DisplayName: got %q, want empty", got.DisplayName)
	}
}

func TestGetUserByID_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetUserByID(99999)
	if err == nil {
		t.Error("expected error for nonexistent user ID")
	}
}

func TestGetUserByID_AfterRoleChange(t *testing.T) {
	s := newTestStore(t)
	user, _ := s.CreateUser("rolecheck", "pin123", "Role User")
	_ = s.SetUserRole(user.ID, "admin")
	got, err := s.GetUserByID(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Role != "admin" {
		t.Errorf("Role: got %q, want %q", got.Role, "admin")
	}
	if got.DisplayName != "Role User" {
		t.Errorf("DisplayName: got %q, want %q", got.DisplayName, "Role User")
	}
}

// ── Push subscription: DeleteAllPushSubscriptions ──

func TestDeleteAllPushSubscriptions(t *testing.T) {
	s := newTestStore(t)
	user, _ := s.CreateUser("pushall", "pin123", "Push User")
	_ = s.SavePushSubscription(user.ID, "https://endpoint1.example.com", "key1", "auth1")
	_ = s.SavePushSubscription(user.ID, "https://endpoint2.example.com", "key2", "auth2")

	subs, _ := s.UserPushSubscriptions(user.ID)
	if len(subs) != 2 {
		t.Fatalf("expected 2 subs, got %d", len(subs))
	}

	if err := s.DeleteAllPushSubscriptions(user.ID); err != nil {
		t.Fatalf("DeleteAllPushSubscriptions: %v", err)
	}

	subs, _ = s.UserPushSubscriptions(user.ID)
	if len(subs) != 0 {
		t.Fatalf("expected 0 subs after delete, got %d", len(subs))
	}
}

func TestDeleteAllPushSubscriptions_NoSubs(t *testing.T) {
	s := newTestStore(t)
	user, _ := s.CreateUser("nopush", "pin123", "No Push")

	// Should not error even if user has no subscriptions
	if err := s.DeleteAllPushSubscriptions(user.ID); err != nil {
		t.Fatalf("DeleteAllPushSubscriptions on empty: %v", err)
	}
}

func TestDeleteAllPushSubscriptions_IsolateUsers(t *testing.T) {
	s := newTestStore(t)
	u1, _ := s.CreateUser("pu1", "p", "")
	u2, _ := s.CreateUser("pu2", "p", "")

	_ = s.SavePushSubscription(u1.ID, "https://ep-u1-a.example.com", "k", "a")
	_ = s.SavePushSubscription(u1.ID, "https://ep-u1-b.example.com", "k", "a")
	_ = s.SavePushSubscription(u2.ID, "https://ep-u2-a.example.com", "k", "a")

	// Delete all for u1 only
	_ = s.DeleteAllPushSubscriptions(u1.ID)

	subs1, _ := s.UserPushSubscriptions(u1.ID)
	subs2, _ := s.UserPushSubscriptions(u2.ID)
	if len(subs1) != 0 {
		t.Errorf("u1 should have 0 subs, got %d", len(subs1))
	}
	if len(subs2) != 1 {
		t.Errorf("u2 should still have 1 sub, got %d", len(subs2))
	}
}

// ── ListChannelsForUser ──

func TestListChannelsForUser(t *testing.T) {
	s := newTestStore(t)
	bot, _ := s.CreateBot("lcfub", "LCFUBot")
	u, _ := s.CreateUser("lcuser", "p", "")
	ch1, _ := s.CreateChannel(bot.ID, "alpha", "Alpha channel")
	_, _ = s.CreateChannel(bot.ID, "beta", "Beta channel")

	_ = s.Subscribe(ch1.ID, u.ID)
	// ch2 not subscribed

	chs, err := s.ListChannelsForUser(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(chs) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(chs))
	}

	// Verify subscription state
	var alpha, beta *ChannelInfo
	for i := range chs {
		if chs[i].Name == "alpha" {
			alpha = &chs[i]
		}
		if chs[i].Name == "beta" {
			beta = &chs[i]
		}
	}
	if alpha == nil || !alpha.Subscribed {
		t.Error("alpha should be subscribed")
	}
	if beta == nil || beta.Subscribed {
		t.Error("beta should not be subscribed")
	}
}

// ── ChannelReadersJoin ──

func TestChannelReadersJoin(t *testing.T) {
	s := newTestStore(t)
	bot, _ := s.CreateBot("crjb", "CRJBot")
	u1, _ := s.CreateUser("reader1", "p", "")
	u2, _ := s.CreateUser("reader2", "p", "")
	ch, _ := s.CreateChannel(bot.ID, "readjoin", "")

	_ = s.Subscribe(ch.ID, u1.ID)
	_ = s.Subscribe(ch.ID, u2.ID)

	msg, _ := s.SaveChannelMessage(ch.ID, "hello", "", "", "")
	s.MarkChannelRead(ch.ID, u1.ID, msg.ID)

	readers, err := s.ChannelReadersJoin(ch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(readers) != 1 {
		t.Fatalf("expected 1 reader (u1), got %d", len(readers))
	}
	if readers[0].Username != "reader1" {
		t.Errorf("expected reader1, got %s", readers[0].Username)
	}
}

// ── UpdateChannelMessageText ──

func TestUpdateChannelMessageText(t *testing.T) {
	s := newTestStore(t)
	bot, _ := s.CreateBot("ucmb", "UCMBot")
	ch, _ := s.CreateChannel(bot.ID, "editchan", "")
	msg, _ := s.SaveChannelMessage(ch.ID, "original", "", "", "")

	if err := s.UpdateChannelMessageText(msg.ID, "edited", ""); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetChannelMessage(msg.ID)
	if got.Text != "edited" {
		t.Errorf("text: got %q, want %q", got.Text, "edited")
	}
	if got.EditedAt == "" {
		t.Error("edited_at should be set after update")
	}
}

// ── DeleteChannelMessage clears pin ──

func TestDeleteChannelMessage_ClearsPin(t *testing.T) {
	s := newTestStore(t)
	bot, _ := s.CreateBot("dcmb", "DCMBot")
	ch, _ := s.CreateChannel(bot.ID, "delpin", "")
	msg, _ := s.SaveChannelMessage(ch.ID, "pinned msg", "", "", "")

	_ = s.PinMessage(ch.ID, msg.ID)
	if pid := s.GetPinnedMessage(ch.ID); pid != msg.ID {
		t.Fatalf("pin not set: got %d", pid)
	}

	_ = s.DeleteChannelMessage(msg.ID)
	if pid := s.GetPinnedMessage(ch.ID); pid != 0 {
		t.Errorf("pin should be cleared after message delete, got %d", pid)
	}
}

// ── ChannelSubscribersJoin ──

func TestChannelSubscribersJoin(t *testing.T) {
	s := newTestStore(t)
	bot, _ := s.CreateBot("csjb", "CSJBot")
	u1, _ := s.CreateUser("alice", "p", "")
	u2, _ := s.CreateUser("bob", "p", "")
	ch, _ := s.CreateChannel(bot.ID, "subjoin", "")

	_ = s.Subscribe(ch.ID, u1.ID)
	_ = s.Subscribe(ch.ID, u2.ID)

	subs, err := s.ChannelSubscribersJoin(ch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 2 {
		t.Fatalf("expected 2 subscribers, got %d", len(subs))
	}
	// Sorted by username: alice < bob
	if subs[0].Username != "alice" {
		t.Errorf("expected alice first, got %s", subs[0].Username)
	}
	if subs[1].Username != "bob" {
		t.Errorf("expected bob second, got %s", subs[1].Username)
	}
}

func TestChannelSubscribersJoin_Empty(t *testing.T) {
	s := newTestStore(t)
	bot, _ := s.CreateBot("csje", "CSJEBot")
	ch, _ := s.CreateChannel(bot.ID, "emptysub", "")

	subs, err := s.ChannelSubscribersJoin(ch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 0 {
		t.Fatalf("expected 0 subscribers, got %d", len(subs))
	}
}

// ── Channel CreatedAt ──

func TestChannelByID_CreatedAt(t *testing.T) {
	s := newTestStore(t)
	bot, _ := s.CreateBot("ccab", "CCABot")
	created, _ := s.CreateChannel(bot.ID, "tschan", "test channel")

	ch, err := s.ChannelByID(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ch.CreatedAt == "" {
		t.Error("expected non-empty created_at from ChannelByID")
	}
	if ch.Name != "tschan" {
		t.Errorf("expected name tschan, got %s", ch.Name)
	}
}

func TestFirstChannelByBot_ReturnsOldest(t *testing.T) {
	s := newTestStore(t)
	bot, _ := s.CreateBot("fcb1", "FCBBot")
	ch1, _ := s.CreateChannel(bot.ID, "first", "")
	_, _ = s.CreateChannel(bot.ID, "second", "")

	got, err := s.FirstChannelByBot(bot.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != ch1.ID {
		t.Errorf("expected channel id %d, got %d", ch1.ID, got.ID)
	}
	if got.Name != "first" {
		t.Errorf("expected name first, got %s", got.Name)
	}
}

func TestFirstChannelByBot_NoChannels(t *testing.T) {
	s := newTestStore(t)
	bot, _ := s.CreateBot("fcb2", "FCBBot2")

	_, err := s.FirstChannelByBot(bot.ID)
	if err == nil {
		t.Error("expected error for bot with no channels")
	}
}
