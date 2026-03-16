package store

import (
	"path/filepath"
	"testing"
)

func setupTest(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	db, err := New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// --- Subscribe ---

func TestSubscribe_CreatesNew(t *testing.T) {
	db := setupTest(t)
	created, err := db.Subscribe("user1", "guild1", "Bamboo", "seed")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !created {
		t.Error("expected true for new subscription")
	}

	// Verify the stored data has correct field values
	subs, _ := db.GetUserSubscriptions("user1")
	if len(subs) != 1 {
		t.Fatalf("expected 1 sub, got %d", len(subs))
	}
	s := subs[0]
	if s.UserID != "user1" {
		t.Errorf("UserID: got %q, want user1", s.UserID)
	}
	if s.GuildID != "guild1" {
		t.Errorf("GuildID: got %q, want guild1", s.GuildID)
	}
	if s.ItemID != "bamboo" {
		t.Errorf("ItemID: got %q, want bamboo (lowercased)", s.ItemID)
	}
	if s.ShopType != "seed" {
		t.Errorf("ShopType: got %q, want seed", s.ShopType)
	}
	if s.ID == 0 {
		t.Error("ID should be non-zero (autoincrement)")
	}
}

func TestSubscribe_DuplicateReturnsFalse(t *testing.T) {
	db := setupTest(t)
	db.Subscribe("user1", "guild1", "Bamboo", "seed")

	created, err := db.Subscribe("user1", "guild1", "Bamboo", "seed")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if created {
		t.Error("expected false for duplicate")
	}

	// Verify only 1 row exists
	subs, _ := db.GetUserSubscriptions("user1")
	if len(subs) != 1 {
		t.Errorf("expected 1 sub after duplicate attempt, got %d", len(subs))
	}
}

func TestSubscribe_StoresLowercase(t *testing.T) {
	db := setupTest(t)
	db.Subscribe("user1", "guild1", "MythicalEgg", "egg")

	subs, _ := db.GetUserSubscriptions("user1")
	if len(subs) != 1 {
		t.Fatalf("expected 1 sub, got %d", len(subs))
	}
	if subs[0].ItemID != "mythicalegg" {
		t.Errorf("ItemID: got %q, want mythicalegg", subs[0].ItemID)
	}
	if subs[0].ShopType != "egg" {
		t.Errorf("ShopType: got %q, want egg", subs[0].ShopType)
	}
}

func TestSubscribe_DifferentItemsSameUser(t *testing.T) {
	db := setupTest(t)
	db.Subscribe("user1", "guild1", "Bamboo", "seed")
	db.Subscribe("user1", "guild1", "Carrot", "seed")

	subs, _ := db.GetUserSubscriptions("user1")
	if len(subs) != 2 {
		t.Fatalf("expected 2 subs, got %d", len(subs))
	}

	items := make(map[string]bool)
	for _, s := range subs {
		items[s.ItemID] = true
		if s.UserID != "user1" {
			t.Errorf("UserID: got %q, want user1", s.UserID)
		}
	}
	if !items["bamboo"] {
		t.Error("missing bamboo subscription")
	}
	if !items["carrot"] {
		t.Error("missing carrot subscription")
	}
}

func TestSubscribe_SameItemDifferentUsers(t *testing.T) {
	db := setupTest(t)
	db.Subscribe("user1", "guild1", "Bamboo", "seed")
	db.Subscribe("user2", "guild1", "Bamboo", "seed")

	subs, _ := db.GetSubscribersForItem("bamboo")
	if len(subs) != 2 {
		t.Fatalf("expected 2 subscribers, got %d", len(subs))
	}

	users := make(map[string]bool)
	for _, s := range subs {
		users[s.UserID] = true
		if s.ItemID != "bamboo" {
			t.Errorf("ItemID: got %q, want bamboo", s.ItemID)
		}
		if s.ShopType != "seed" {
			t.Errorf("ShopType: got %q, want seed", s.ShopType)
		}
	}
	if !users["user1"] {
		t.Error("missing user1")
	}
	if !users["user2"] {
		t.Error("missing user2")
	}
}

// --- Unsubscribe ---

func TestUnsubscribe_RemovesExisting(t *testing.T) {
	db := setupTest(t)
	db.Subscribe("user1", "guild1", "Bamboo", "seed")

	removed, err := db.Unsubscribe("user1", "Bamboo")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !removed {
		t.Error("expected true for existing subscription")
	}

	subs, _ := db.GetUserSubscriptions("user1")
	if len(subs) != 0 {
		t.Errorf("expected 0 after unsubscribe, got %d", len(subs))
	}
}

func TestUnsubscribe_ThenResubscribe(t *testing.T) {
	db := setupTest(t)
	db.Subscribe("user1", "guild1", "Bamboo", "seed")
	db.Unsubscribe("user1", "Bamboo")

	created, err := db.Subscribe("user1", "guild1", "Bamboo", "seed")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !created {
		t.Error("re-subscribe should return true")
	}

	subs, _ := db.GetUserSubscriptions("user1")
	if len(subs) != 1 {
		t.Fatalf("expected 1 after re-subscribe, got %d", len(subs))
	}
	if subs[0].ItemID != "bamboo" || subs[0].UserID != "user1" {
		t.Errorf("re-subscribed data: got item=%q user=%q, want bamboo/user1", subs[0].ItemID, subs[0].UserID)
	}
}

func TestUnsubscribe_NotFoundReturnsFalse(t *testing.T) {
	db := setupTest(t)
	removed, err := db.Unsubscribe("user1", "Bamboo")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if removed {
		t.Error("expected false for non-existent")
	}
}

// --- UnsubscribeAll ---

func TestUnsubscribeAll(t *testing.T) {
	db := setupTest(t)
	db.Subscribe("user1", "guild1", "Bamboo", "seed")
	db.Subscribe("user1", "guild1", "Carrot", "seed")
	db.Subscribe("user1", "guild1", "MythicalEgg", "egg")

	count, err := db.UnsubscribeAll("user1")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if count != 3 {
		t.Errorf("removed: got %d, want 3", count)
	}

	subs, _ := db.GetUserSubscriptions("user1")
	if len(subs) != 0 {
		t.Errorf("remaining: got %d, want 0", len(subs))
	}
}

func TestUnsubscribeAll_DoesNotAffectOtherUsers(t *testing.T) {
	db := setupTest(t)
	db.Subscribe("user1", "guild1", "Bamboo", "seed")
	db.Subscribe("user2", "guild1", "Bamboo", "seed")

	db.UnsubscribeAll("user1")

	subs, _ := db.GetSubscribersForItem("bamboo")
	if len(subs) != 1 {
		t.Fatalf("expected 1 remaining, got %d", len(subs))
	}
	if subs[0].UserID != "user2" {
		t.Errorf("remaining user: got %q, want user2", subs[0].UserID)
	}
	if subs[0].ItemID != "bamboo" {
		t.Errorf("remaining item: got %q, want bamboo", subs[0].ItemID)
	}
}

// --- GetSubscribersForItem ---

func TestGetSubscribersForItem_CaseInsensitive(t *testing.T) {
	db := setupTest(t)
	db.Subscribe("user1", "guild1", "MythicalEgg", "egg")

	// Query with ALL CAPS -- should still match (both sides lowercased)
	subs, err := db.GetSubscribersForItem("MYTHICALEGG")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected 1, got %d", len(subs))
	}
	if subs[0].UserID != "user1" {
		t.Errorf("UserID: got %q, want user1", subs[0].UserID)
	}
	if subs[0].ItemID != "mythicalegg" {
		t.Errorf("ItemID: got %q, want mythicalegg", subs[0].ItemID)
	}
}

func TestGetSubscribersForItem_NoSubscribers(t *testing.T) {
	db := setupTest(t)
	subs, err := db.GetSubscribersForItem("bamboo")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if subs == nil {
		// nil is acceptable but let's document it
	}
	if len(subs) != 0 {
		t.Errorf("expected 0, got %d", len(subs))
	}
}

// --- Board Config ---

func TestBoardConfig_SetAndGet(t *testing.T) {
	db := setupTest(t)
	db.SetBoardMessage("guild1", "chan1", "seed", "msg1")
	db.SetBoardMessage("guild1", "chan2", "egg", "msg2")

	cfg, err := db.GetBoardConfig("guild1")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config, got nil")
	}
	if cfg.GuildID != "guild1" {
		t.Errorf("GuildID: got %q, want guild1", cfg.GuildID)
	}
	if cfg.Channels["seed"] != "chan1" {
		t.Errorf("Channels[seed]: got %q, want chan1", cfg.Channels["seed"])
	}
	if cfg.Channels["egg"] != "chan2" {
		t.Errorf("Channels[egg]: got %q, want chan2", cfg.Channels["egg"])
	}
	if cfg.Messages["seed"] != "msg1" {
		t.Errorf("Messages[seed]: got %q, want msg1", cfg.Messages["seed"])
	}
	if cfg.Messages["egg"] != "msg2" {
		t.Errorf("Messages[egg]: got %q, want msg2", cfg.Messages["egg"])
	}
}

func TestBoardConfig_NotFound(t *testing.T) {
	db := setupTest(t)
	cfg, err := db.GetBoardConfig("nonexistent")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if cfg != nil {
		t.Errorf("expected nil, got %+v", cfg)
	}
}

func TestBoardConfig_Upsert(t *testing.T) {
	db := setupTest(t)
	db.SetBoardMessage("guild1", "chan1", "seed", "msg1")
	db.SetBoardMessage("guild1", "chan1", "seed", "msg2")

	cfg, _ := db.GetBoardConfig("guild1")
	if cfg.Messages["seed"] != "msg2" {
		t.Errorf("Messages[seed]: got %q, want msg2 (upserted)", cfg.Messages["seed"])
	}
	if cfg.Channels["seed"] != "chan1" {
		t.Errorf("Channels[seed]: got %q, want chan1", cfg.Channels["seed"])
	}
}

func TestBoardConfig_Delete(t *testing.T) {
	db := setupTest(t)
	db.SetBoardMessage("guild1", "chan1", "seed", "msg1")
	db.DeleteBoardConfig("guild1")

	cfg, _ := db.GetBoardConfig("guild1")
	if cfg != nil {
		t.Errorf("expected nil after delete, got %+v", cfg)
	}
}

func TestGetAllBoardConfigs(t *testing.T) {
	db := setupTest(t)
	db.SetBoardMessage("guild1", "chan1", "seed", "msg1")
	db.SetBoardMessage("guild2", "chan2", "egg", "msg2")

	configs, err := db.GetAllBoardConfigs()
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(configs) != 2 {
		t.Fatalf("expected 2 configs, got %d", len(configs))
	}

	byGuild := make(map[string]BoardConfig)
	for _, c := range configs {
		byGuild[c.GuildID] = c
	}
	if g1 := byGuild["guild1"]; g1.Channels["seed"] != "chan1" || g1.Messages["seed"] != "msg1" {
		t.Errorf("guild1: got channels=%v messages=%v", g1.Channels, g1.Messages)
	}
	if g2 := byGuild["guild2"]; g2.Channels["egg"] != "chan2" || g2.Messages["egg"] != "msg2" {
		t.Errorf("guild2: got channels=%v messages=%v", g2.Channels, g2.Messages)
	}
}

// --- Config ---

func TestConfig_SetAndGet(t *testing.T) {
	db := setupTest(t)
	db.SetConfig("discord_token", "test-token")

	val, err := db.GetConfig("discord_token")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if val != "test-token" {
		t.Errorf("got %q, want test-token", val)
	}
}

func TestConfig_GetMissing(t *testing.T) {
	db := setupTest(t)
	val, err := db.GetConfig("nonexistent")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if val != "" {
		t.Errorf("got %q, want empty string", val)
	}
}

func TestConfig_Upsert(t *testing.T) {
	db := setupTest(t)
	db.SetConfig("key", "value1")
	db.SetConfig("key", "value2")

	val, _ := db.GetConfig("key")
	if val != "value2" {
		t.Errorf("got %q, want value2 (upserted)", val)
	}
}

func TestGetAllConfig(t *testing.T) {
	db := setupTest(t)
	db.SetConfig("key1", "val1")
	db.SetConfig("key2", "val2")

	cfg, err := db.GetAllConfig()
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(cfg) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(cfg))
	}
	if cfg["key1"] != "val1" {
		t.Errorf("key1: got %q, want val1", cfg["key1"])
	}
	if cfg["key2"] != "val2" {
		t.Errorf("key2: got %q, want val2", cfg["key2"])
	}
}
