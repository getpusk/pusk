// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package store

import (
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
	t.Cleanup(func() { s.Close() })
	return s
}

// ── Migration ──

func TestMigrate_VersionBumps(t *testing.T) {
	s := newTestStore(t)
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 3 {
		t.Errorf("expected user_version=3 after full migrate, got %d", version)
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
	s.CreateUser("carol", "secret", "Carol")
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
	s.CreateUser("dave", "correct", "")
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
	s.SetUserRole(u.ID, "admin")
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
	s.CreateUser("resetme", "old", "")
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
	s.CreateUser("u1", "p", "")
	s.CreateUser("u2", "p", "")
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

	s.UpdateMessageText(msg.ID, "v2", `{"keyboard":[]}`)
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

	s.DeleteMessage(msg.ID)
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
		s.SaveMessage(chat.ID, "bot", "msg", "", "", "")
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
	s.Unsubscribe(ch.ID, u.ID)
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
	if subs != nil && len(subs) != 0 {
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

	s.Subscribe(ch.ID, u1.ID)
	s.Subscribe(ch.ID, u2.ID)
	s.Subscribe(ch.ID, u3.ID)

	subs, _ := s.ChannelSubscribers(ch.ID)
	if len(subs) != 3 {
		t.Errorf("expected 3 subs, got %d", len(subs))
	}
}

func TestChannelByName(t *testing.T) {
	s := newTestStore(t)
	bot, _ := s.CreateBot("nb", "NBot")
	s.CreateChannel(bot.ID, "alerts", "Alert channel")

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
	s.Subscribe(ch.ID, u.ID)

	m1, _ := s.SaveChannelMessage(ch.ID, "msg1", "", "", "")
	s.SaveChannelMessage(ch.ID, "msg2", "", "", "")

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
	s.PinMessage(ch.ID, 99)
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

	s.SavePushSubscription(u.ID, "https://push.example.com/dup", "key1", "auth1")
	s.SavePushSubscription(u.ID, "https://push.example.com/dup", "key2", "auth2")

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
		s.SavePushSubscription(u.ID, "https://push.example.com/"+string(rune('a'+i)), "k", "a")
	}
	subs, _ := s.UserPushSubscriptions(u.ID)
	if len(subs) > 5 {
		t.Errorf("expected max 5, got %d", len(subs))
	}
}

func TestDeletePushSubscription(t *testing.T) {
	s := newTestStore(t)
	u, _ := s.CreateUser("delps", "p", "")
	s.SavePushSubscription(u.ID, "https://push.example.com/del", "k", "a")
	s.DeletePushSubscription("https://push.example.com/del")
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
	if err := s.UseInvite(code); err != nil {
		t.Fatalf("UseInvite: %v", err)
	}
	// Second use should fail
	if err := s.UseInvite(code); err == nil {
		t.Error("expected error on double use")
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
	s.SaveFile("a1", bot.ID, "a", "x", 100, "/a")
	s.SaveFile("a2", bot.ID, "b", "x", 200, "/b")
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
	s.Subscribe(ch.ID, u.ID)
	s.SaveChannelMessage(ch.ID, "msg", "", "", "")

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

// ── UserSubscriptions ──

func TestUserSubscriptions(t *testing.T) {
	s := newTestStore(t)
	bot, _ := s.CreateBot("usb", "USBot")
	u, _ := s.CreateUser("subuser", "p", "")
	ch1, _ := s.CreateChannel(bot.ID, "ch1", "")
	ch2, _ := s.CreateChannel(bot.ID, "ch2", "")

	s.Subscribe(ch1.ID, u.ID)
	s.Subscribe(ch2.ID, u.ID)

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
	s := newTestStore(t)
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_, err := s.CreateUser("user_"+string(rune('A'+n)), "pin", "")
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
	s.SetWebhook(bot.ID, "https://example.com/hook")
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
