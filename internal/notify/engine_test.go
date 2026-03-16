package notify

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaylincoded/magic-guardian/internal/mg"
	"github.com/kaylincoded/magic-guardian/internal/store"
)

// mockSender records all alerts sent, tracking individual calls for batch verification.
type mockSender struct {
	// callLog records each SendStockAlert invocation: userID → list of per-call payloads
	callLog map[string][][]mg.StockChange
}

func newMockSender() *mockSender {
	return &mockSender{callLog: make(map[string][][]mg.StockChange)}
}

func (m *mockSender) SendStockAlert(userID string, changes []mg.StockChange) error {
	// Deep copy the slice to avoid aliasing
	cp := make([]mg.StockChange, len(changes))
	copy(cp, changes)
	m.callLog[userID] = append(m.callLog[userID], cp)
	return nil
}

// calls returns the number of times SendStockAlert was called for a user.
func (m *mockSender) calls(userID string) int {
	return len(m.callLog[userID])
}

// allItems returns all items delivered to a user across all calls (flattened).
func (m *mockSender) allItems(userID string) []mg.StockChange {
	var out []mg.StockChange
	for _, batch := range m.callLog[userID] {
		out = append(out, batch...)
	}
	return out
}

func setupTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	db, err := store.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestHandleRestocks_SendsAlertToSubscribers(t *testing.T) {
	db := setupTestStore(t)
	sender := newMockSender()
	engine := NewEngine(db, sender, testLogger())

	db.Subscribe("user1", "guild1", "bamboo", "seed")

	engine.HandleRestocks([]mg.StockChange{
		{ShopType: "seed", Item: mg.ShopItem{ItemType: "Seed", Species: "Bamboo"}, OldStock: 0, NewStock: 5},
	})

	if sender.calls("user1") != 1 {
		t.Fatalf("calls: got %d, want 1", sender.calls("user1"))
	}
	items := sender.allItems("user1")
	if len(items) != 1 {
		t.Fatalf("items: got %d, want 1", len(items))
	}
	ch := items[0]
	if ch.Item.ItemID() != "Bamboo" {
		t.Errorf("item: got %q, want Bamboo", ch.Item.ItemID())
	}
	if ch.ShopType != "seed" {
		t.Errorf("ShopType: got %q, want seed", ch.ShopType)
	}
	if ch.OldStock != 0 || ch.NewStock != 5 {
		t.Errorf("stock: got %d→%d, want 0→5", ch.OldStock, ch.NewStock)
	}
}

func TestHandleRestocks_NoAlertForUnsubscribed(t *testing.T) {
	db := setupTestStore(t)
	sender := newMockSender()
	engine := NewEngine(db, sender, testLogger())

	db.Subscribe("user1", "guild1", "bamboo", "seed")

	engine.HandleRestocks([]mg.StockChange{
		{ShopType: "seed", Item: mg.ShopItem{ItemType: "Seed", Species: "Carrot"}, OldStock: 0, NewStock: 5},
	})

	if sender.calls("user1") != 0 {
		t.Errorf("calls: got %d, want 0 (user not subscribed to Carrot)", sender.calls("user1"))
	}
}

func TestHandleRestocks_BatchMultipleItemsPerUser(t *testing.T) {
	db := setupTestStore(t)
	sender := newMockSender()
	engine := NewEngine(db, sender, testLogger())

	db.Subscribe("user1", "guild1", "bamboo", "seed")
	db.Subscribe("user1", "guild1", "carrot", "seed")

	engine.HandleRestocks([]mg.StockChange{
		{ShopType: "seed", Item: mg.ShopItem{ItemType: "Seed", Species: "Bamboo"}, OldStock: 0, NewStock: 5},
		{ShopType: "seed", Item: mg.ShopItem{ItemType: "Seed", Species: "Carrot"}, OldStock: 0, NewStock: 3},
	})

	// CRITICAL: verify batching -- exactly 1 call with both items, not 2 calls with 1 each
	if sender.calls("user1") != 1 {
		t.Fatalf("calls: got %d, want 1 (should be batched into single call)", sender.calls("user1"))
	}
	batch := sender.callLog["user1"][0]
	if len(batch) != 2 {
		t.Fatalf("batch size: got %d, want 2", len(batch))
	}

	// Verify both items have correct payload
	byItem := make(map[string]mg.StockChange)
	for _, ch := range batch {
		byItem[ch.Item.ItemID()] = ch
	}
	if ch := byItem["Bamboo"]; ch.OldStock != 0 || ch.NewStock != 5 || ch.ShopType != "seed" {
		t.Errorf("Bamboo: got shop=%q %d→%d, want seed 0→5", ch.ShopType, ch.OldStock, ch.NewStock)
	}
	if ch := byItem["Carrot"]; ch.OldStock != 0 || ch.NewStock != 3 || ch.ShopType != "seed" {
		t.Errorf("Carrot: got shop=%q %d→%d, want seed 0→3", ch.ShopType, ch.OldStock, ch.NewStock)
	}
}

func TestHandleRestocks_MultipleUsers(t *testing.T) {
	db := setupTestStore(t)
	sender := newMockSender()
	engine := NewEngine(db, sender, testLogger())

	db.Subscribe("user1", "guild1", "bamboo", "seed")
	db.Subscribe("user2", "guild1", "bamboo", "seed")

	engine.HandleRestocks([]mg.StockChange{
		{ShopType: "seed", Item: mg.ShopItem{ItemType: "Seed", Species: "Bamboo"}, OldStock: 0, NewStock: 5},
	})

	// Each user gets exactly 1 call with 1 item
	for _, uid := range []string{"user1", "user2"} {
		if sender.calls(uid) != 1 {
			t.Errorf("%s calls: got %d, want 1", uid, sender.calls(uid))
		}
		items := sender.allItems(uid)
		if len(items) != 1 {
			t.Errorf("%s items: got %d, want 1", uid, len(items))
			continue
		}
		ch := items[0]
		if ch.Item.ItemID() != "Bamboo" || ch.OldStock != 0 || ch.NewStock != 5 {
			t.Errorf("%s payload: got item=%q %d→%d, want Bamboo 0→5", uid, ch.Item.ItemID(), ch.OldStock, ch.NewStock)
		}
	}
}

func TestHandleRestocks_EmptyChanges(t *testing.T) {
	db := setupTestStore(t)
	sender := newMockSender()
	engine := NewEngine(db, sender, testLogger())

	engine.HandleRestocks([]mg.StockChange{})

	if len(sender.callLog) != 0 {
		t.Errorf("expected no calls, got calls for %d users", len(sender.callLog))
	}
}

func TestHandleRestocks_CaseInsensitiveItemMatching(t *testing.T) {
	db := setupTestStore(t)
	sender := newMockSender()
	engine := NewEngine(db, sender, testLogger())

	db.Subscribe("user1", "guild1", "MythicalEgg", "egg")

	engine.HandleRestocks([]mg.StockChange{
		{ShopType: "egg", Item: mg.ShopItem{ItemType: "Egg", EggID: "MythicalEgg"}, OldStock: 0, NewStock: 1},
	})

	if sender.calls("user1") != 1 {
		t.Fatalf("calls: got %d, want 1", sender.calls("user1"))
	}
	ch := sender.allItems("user1")[0]
	if ch.Item.ItemID() != "MythicalEgg" {
		t.Errorf("item: got %q, want MythicalEgg", ch.Item.ItemID())
	}
	if ch.ShopType != "egg" || ch.OldStock != 0 || ch.NewStock != 1 {
		t.Errorf("payload: got shop=%q %d→%d, want egg 0→1", ch.ShopType, ch.OldStock, ch.NewStock)
	}
}

// --- Error handling ---

type failingSender struct {
	failFor map[string]bool
	callLog map[string][][]mg.StockChange
}

func newFailingSender(failUsers ...string) *failingSender {
	f := &failingSender{
		failFor: make(map[string]bool),
		callLog: make(map[string][][]mg.StockChange),
	}
	for _, u := range failUsers {
		f.failFor[u] = true
	}
	return f
}

func (f *failingSender) SendStockAlert(userID string, changes []mg.StockChange) error {
	cp := make([]mg.StockChange, len(changes))
	copy(cp, changes)
	f.callLog[userID] = append(f.callLog[userID], cp)
	if f.failFor[userID] {
		return fmt.Errorf("send failed for %s", userID)
	}
	return nil
}

func TestHandleRestocks_ErrorForOneUserDoesNotBlockOthers(t *testing.T) {
	db := setupTestStore(t)
	sender := newFailingSender("user1")
	engine := NewEngine(db, sender, testLogger())

	db.Subscribe("user1", "guild1", "bamboo", "seed")
	db.Subscribe("user2", "guild1", "bamboo", "seed")

	engine.HandleRestocks([]mg.StockChange{
		{ShopType: "seed", Item: mg.ShopItem{ItemType: "Seed", Species: "Bamboo"}, OldStock: 0, NewStock: 5},
	})

	// user1 was attempted (call was made) but it errored
	if len(sender.callLog["user1"]) != 1 {
		t.Errorf("user1: expected 1 call attempt, got %d", len(sender.callLog["user1"]))
	}

	// user2 should still succeed with correct payload
	if len(sender.callLog["user2"]) != 1 {
		t.Fatalf("user2: expected 1 call, got %d", len(sender.callLog["user2"]))
	}
	ch := sender.callLog["user2"][0][0]
	if ch.Item.ItemID() != "Bamboo" || ch.OldStock != 0 || ch.NewStock != 5 {
		t.Errorf("user2 payload: got item=%q %d→%d, want Bamboo 0→5",
			ch.Item.ItemID(), ch.OldStock, ch.NewStock)
	}
}
