package mg

import (
	"encoding/json"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"testing"
)

// --- diffShopState ---

func TestDiffShopState_DetectsRestock(t *testing.T) {
	old := map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 0},
			{ItemType: "Seed", Species: "Carrot", InitialStock: 5},
		}},
	}
	new := map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 3},
			{ItemType: "Seed", Species: "Carrot", InitialStock: 5},
		}},
	}

	changes := diffShopState(old, new)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	ch := changes[0]
	if ch.ShopType != "seed" {
		t.Errorf("ShopType: got %q, want %q", ch.ShopType, "seed")
	}
	if ch.Item.ItemType != "Seed" || ch.Item.Species != "Bamboo" {
		t.Errorf("Item: got type=%q species=%q, want Seed/Bamboo", ch.Item.ItemType, ch.Item.Species)
	}
	if ch.OldStock != 0 {
		t.Errorf("OldStock: got %d, want 0", ch.OldStock)
	}
	if ch.NewStock != 3 {
		t.Errorf("NewStock: got %d, want 3", ch.NewStock)
	}
}

func TestDiffShopState_NoChangeNoEvent(t *testing.T) {
	old := map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 5},
		}},
	}
	new := map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 5},
		}},
	}
	changes := diffShopState(old, new)
	if len(changes) != 0 {
		t.Fatalf("expected no changes for identical data, got %d", len(changes))
	}
}

func TestDiffShopState_StockFluctuationIsNotRestock(t *testing.T) {
	old := map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 5},
			{ItemType: "Seed", Species: "Cactus", InitialStock: 3},
			{ItemType: "Seed", Species: "Date", InitialStock: 2},
		}},
	}
	new := map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 3},
			{ItemType: "Seed", Species: "Cactus", InitialStock: 1},
			{ItemType: "Seed", Species: "Date", InitialStock: 5},
		}},
	}

	changes := diffShopState(old, new)
	if len(changes) != 3 {
		t.Fatalf("expected 3 changes, got %d", len(changes))
	}

	// Build map for deterministic checking (map iteration order is random)
	byItem := make(map[string]StockChange)
	for _, ch := range changes {
		byItem[ch.Item.ItemID()] = ch
	}

	assertChange := func(name string, oldS, newS int) {
		t.Helper()
		ch, ok := byItem[name]
		if !ok {
			t.Errorf("missing change for %s", name)
			return
		}
		if ch.OldStock != oldS || ch.NewStock != newS {
			t.Errorf("%s: got %d→%d, want %d→%d", name, ch.OldStock, ch.NewStock, oldS, newS)
		}
		if ch.ShopType != "seed" {
			t.Errorf("%s: ShopType got %q, want %q", name, ch.ShopType, "seed")
		}
	}
	assertChange("Bamboo", 5, 3)
	assertChange("Cactus", 3, 1)
	assertChange("Date", 2, 5)
}

func TestDiffShopState_NewItemInInventoryIgnored(t *testing.T) {
	old := map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 5},
		}},
	}
	new := map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 5},
			{ItemType: "Seed", Species: "BrandNewItem", InitialStock: 10},
		}},
	}

	changes := diffShopState(old, new)
	for _, ch := range changes {
		t.Errorf("unexpected change: %s %d→%d (new items should be ignored)",
			ch.Item.ItemID(), ch.OldStock, ch.NewStock)
	}
}

func TestDiffShopState_RemovedItemIgnored(t *testing.T) {
	old := map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 5},
			{ItemType: "Seed", Species: "RemovedItem", InitialStock: 3},
		}},
	}
	new := map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 5},
		}},
	}

	changes := diffShopState(old, new)
	if len(changes) != 0 {
		t.Fatalf("expected 0 changes, got %d", len(changes))
	}
}

func TestDiffShopState_InventoryReorderHandled(t *testing.T) {
	old := map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 5},
			{ItemType: "Seed", Species: "Carrot", InitialStock: 0},
		}},
	}
	new := map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Carrot", InitialStock: 3},
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 5},
		}},
	}

	changes := diffShopState(old, new)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	ch := changes[0]
	if ch.Item.ItemID() != "Carrot" {
		t.Errorf("Item: got %s, want Carrot", ch.Item.ItemID())
	}
	if ch.OldStock != 0 || ch.NewStock != 3 {
		t.Errorf("Stock: got %d→%d, want 0→3", ch.OldStock, ch.NewStock)
	}
	if ch.ShopType != "seed" {
		t.Errorf("ShopType: got %q, want %q", ch.ShopType, "seed")
	}
}

func TestDiffShopState_NewShopIgnored(t *testing.T) {
	old := map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 5},
		}},
	}
	new := map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 5},
		}},
		"newshop": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Magic", InitialStock: 10},
		}},
	}

	changes := diffShopState(old, new)
	if len(changes) != 0 {
		t.Fatalf("expected no changes, got %d", len(changes))
	}
}

func TestDiffShopState_MultipleShops(t *testing.T) {
	old := map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 0},
		}},
		"egg": {Inventory: []ShopItem{
			{ItemType: "Egg", EggID: "CommonEgg", InitialStock: 10},
		}},
	}
	new := map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 5},
		}},
		"egg": {Inventory: []ShopItem{
			{ItemType: "Egg", EggID: "CommonEgg", InitialStock: 3},
		}},
	}

	changes := diffShopState(old, new)
	if len(changes) != 2 {
		t.Fatalf("expected 2 changes, got %d", len(changes))
	}
	byItem := make(map[string]StockChange)
	for _, ch := range changes {
		byItem[ch.Item.ItemID()] = ch
	}
	if ch := byItem["Bamboo"]; ch.ShopType != "seed" || ch.OldStock != 0 || ch.NewStock != 5 {
		t.Errorf("Bamboo: got shop=%q %d→%d, want seed 0→5", ch.ShopType, ch.OldStock, ch.NewStock)
	}
	if ch := byItem["CommonEgg"]; ch.ShopType != "egg" || ch.OldStock != 10 || ch.NewStock != 3 {
		t.Errorf("CommonEgg: got shop=%q %d→%d, want egg 10→3", ch.ShopType, ch.OldStock, ch.NewStock)
	}
}

func TestDiffShopState_EmptyOldState(t *testing.T) {
	old := map[string]*Shop{}
	new := map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 5},
		}},
	}
	changes := diffShopState(old, new)
	if len(changes) != 0 {
		t.Fatalf("expected no changes with empty old state, got %d", len(changes))
	}
}

// --- Integration: handleWelcome ---

func testClient(t *testing.T) *Client {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewClient(ClientConfig{RoomID: "TEST", Version: "1"}, logger)
}

func makeWelcomeJSON(t *testing.T, shops map[string]*Shop) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(WelcomeState{
		Scope: "room",
		Child: ChildState{Scope: "quinoa", Data: QuinoData{Shops: shops}},
	})
	if err != nil {
		t.Fatalf("marshal WelcomeState: %v", err)
	}
	return b
}

func TestHandleWelcome_ReconnectOnlyFiresRestockForZeroToN(t *testing.T) {
	c := testClient(t)
	c.state.SetFromWelcome(map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 5},
			{ItemType: "Seed", Species: "Cactus", InitialStock: 3},
			{ItemType: "Seed", Species: "Date", InitialStock: 0},
		}, SecondsUntilRestock: 100},
	})

	var restockPayload []StockChange
	var stockChanges []StockChange
	var connectCalled bool

	c.OnRestock(func(changes []StockChange) { restockPayload = changes })
	c.OnStockChange(func(changes []StockChange) { stockChanges = changes })
	c.onConnect = func() { connectCalled = true }

	c.handleWelcome(makeWelcomeJSON(t, map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 3}, // 5→3 NOT restock
			{ItemType: "Seed", Species: "Cactus", InitialStock: 1}, // 3→1 NOT restock
			{ItemType: "Seed", Species: "Date", InitialStock: 5},   // 0→5 IS restock
		}, SecondsUntilRestock: 200},
	}))

	if !connectCalled {
		t.Error("onConnect not called")
	}

	// Verify all 3 stock changes with full payload
	if len(stockChanges) != 3 {
		t.Fatalf("onStockChange: got %d changes, want 3", len(stockChanges))
	}

	// Verify restock payload: exactly 1, and it's Date with correct fields
	if len(restockPayload) != 1 {
		t.Fatalf("onRestock: got %d, want 1", len(restockPayload))
	}
	r := restockPayload[0]
	if r.Item.ItemID() != "Date" {
		t.Errorf("restock item: got %q, want Date", r.Item.ItemID())
	}
	if r.ShopType != "seed" {
		t.Errorf("restock ShopType: got %q, want seed", r.ShopType)
	}
	if r.OldStock != 0 {
		t.Errorf("restock OldStock: got %d, want 0", r.OldStock)
	}
	if r.NewStock != 5 {
		t.Errorf("restock NewStock: got %d, want 5", r.NewStock)
	}
}

func TestHandleWelcome_NoRestockWhenStockOnlyFluctuates(t *testing.T) {
	c := testClient(t)
	c.state.SetFromWelcome(map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 5},
			{ItemType: "Seed", Species: "Carrot", InitialStock: 10},
		}, SecondsUntilRestock: 100},
	})

	var restockCalled bool
	c.OnRestock(func(changes []StockChange) { restockCalled = true })
	c.onConnect = func() {}

	c.handleWelcome(makeWelcomeJSON(t, map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 3},
			{ItemType: "Seed", Species: "Carrot", InitialStock: 7},
		}, SecondsUntilRestock: 200},
	}))

	if restockCalled {
		t.Error("onRestock should NOT fire when stock only fluctuates (N→M, not 0→N)")
	}
}

func TestHandleWelcome_FirstConnectNoRestockAlerts(t *testing.T) {
	c := testClient(t)
	var restockCalled bool
	c.OnRestock(func(changes []StockChange) { restockCalled = true })
	c.onConnect = func() {}

	c.handleWelcome(makeWelcomeJSON(t, map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 5},
		}, SecondsUntilRestock: 100},
	}))

	if restockCalled {
		t.Error("onRestock should NOT fire on first connect")
	}

	// Verify state was still populated
	shop, ok := c.state.GetShop("seed")
	if !ok {
		t.Fatal("seed shop should exist after first Welcome")
	}
	if shop.Inventory[0].InitialStock != 5 {
		t.Errorf("stock: got %d, want 5", shop.Inventory[0].InitialStock)
	}
}

func TestHandleWelcome_NewItemDoesNotTriggerRestock(t *testing.T) {
	c := testClient(t)
	c.state.SetFromWelcome(map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 5},
		}, SecondsUntilRestock: 100},
	})

	var restockPayload []StockChange
	c.OnRestock(func(changes []StockChange) { restockPayload = changes })
	c.onConnect = func() {}

	c.handleWelcome(makeWelcomeJSON(t, map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 5},
			{ItemType: "Seed", Species: "BrandNewItem", InitialStock: 10},
		}, SecondsUntilRestock: 200},
	}))

	if len(restockPayload) != 0 {
		t.Errorf("new item should not trigger restock, got %d items: %v", len(restockPayload), restockPayload)
	}
}

func TestHandleWelcome_MalformedJSON(t *testing.T) {
	c := testClient(t)
	c.onConnect = func() { t.Error("onConnect should not fire for malformed JSON") }
	c.OnRestock(func(changes []StockChange) { t.Error("onRestock should not fire") })

	// Must not panic
	c.handleWelcome(json.RawMessage(`{invalid json`))

	// State should remain empty
	shops := c.state.GetAllShops()
	if len(shops) != 0 {
		t.Errorf("state should remain empty after malformed Welcome, got %d shops", len(shops))
	}
}

func TestHandleWelcome_NilShops(t *testing.T) {
	c := testClient(t)
	c.onConnect = func() { t.Error("onConnect should not fire when shops is nil") }

	b, _ := json.Marshal(WelcomeState{
		Child: ChildState{Data: QuinoData{Shops: nil}},
	})
	c.handleWelcome(b)
}

// --- Integration: handleMessage routing ---

func TestHandleMessage_RoutesWelcome(t *testing.T) {
	c := testClient(t)
	var connectCalled bool
	c.onConnect = func() { connectCalled = true }

	welcome := WelcomeState{
		Child: ChildState{Data: QuinoData{
			Shops: map[string]*Shop{
				"seed": {Inventory: []ShopItem{
					{ItemType: "Seed", Species: "Bamboo", InitialStock: 5},
				}, SecondsUntilRestock: 100},
			},
		}},
	}
	fullState, _ := json.Marshal(welcome)
	raw, _ := json.Marshal(ServerMessage{Type: "Welcome", FullState: fullState})

	c.handleMessage(raw)

	if !connectCalled {
		t.Error("Welcome should trigger onConnect via handleWelcome")
	}
	shop, ok := c.state.GetShop("seed")
	if !ok {
		t.Fatal("seed shop should be loaded after Welcome")
	}
	if shop.Inventory[0].InitialStock != 5 {
		t.Errorf("stock: got %d, want 5", shop.Inventory[0].InitialStock)
	}
	if shop.Inventory[0].Species != "Bamboo" {
		t.Errorf("species: got %q, want Bamboo", shop.Inventory[0].Species)
	}
}

func TestHandleMessage_RoutesPartialState(t *testing.T) {
	c := testClient(t)
	c.state.SetFromWelcome(map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 0},
		}},
	})

	var deliveredChanges []StockChange
	c.OnStockChange(func(changes []StockChange) { deliveredChanges = changes })

	raw, _ := json.Marshal(ServerMessage{
		Type: "PartialState",
		Patches: []Patch{
			{Op: "replace", Path: "/child/data/shops/seed/inventory/0/initialStock", Value: json.Number("5")},
		},
	})
	c.handleMessage(raw)

	if len(deliveredChanges) != 1 {
		t.Fatalf("expected 1 change, got %d", len(deliveredChanges))
	}
	ch := deliveredChanges[0]
	if ch.Item.ItemID() != "Bamboo" || ch.OldStock != 0 || ch.NewStock != 5 || ch.ShopType != "seed" {
		t.Errorf("change payload: got item=%q shop=%q %d→%d, want Bamboo seed 0→5",
			ch.Item.ItemID(), ch.ShopType, ch.OldStock, ch.NewStock)
	}
}

func TestHandleMessage_UnknownTypeDoesNotPanic(t *testing.T) {
	c := testClient(t)
	raw, _ := json.Marshal(ServerMessage{Type: "SomeNewType"})
	c.handleMessage(raw) // must not panic
}

func TestHandleMessage_InvalidJSONDoesNotPanic(t *testing.T) {
	c := testClient(t)
	c.handleMessage([]byte(`not json at all`)) // must not panic
}

func TestHandleMessage_CaseSensitiveRouting(t *testing.T) {
	c := testClient(t)
	c.onConnect = func() { t.Error("lowercase 'welcome' should NOT route to handleWelcome") }

	welcome := WelcomeState{
		Child: ChildState{Data: QuinoData{
			Shops: map[string]*Shop{"seed": {Inventory: []ShopItem{}}},
		}},
	}
	fullState, _ := json.Marshal(welcome)
	raw, _ := json.Marshal(ServerMessage{Type: "welcome", FullState: fullState}) // lowercase!
	c.handleMessage(raw)
}

// --- Integration: handlePartialState ---

func TestHandlePartialState_OnlyFiresRestockForZeroToN(t *testing.T) {
	c := testClient(t)
	c.state.SetFromWelcome(map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 0},
			{ItemType: "Seed", Species: "Carrot", InitialStock: 5},
		}, SecondsUntilRestock: 100},
	})

	var restockPayload []StockChange
	var allChanges []StockChange
	c.OnRestock(func(changes []StockChange) { restockPayload = changes })
	c.OnStockChange(func(changes []StockChange) { allChanges = changes })

	c.handlePartialState([]Patch{
		{Op: "replace", Path: "/child/data/shops/seed/inventory/0/initialStock", Value: json.Number("3")}, // 0→3 restock
		{Op: "replace", Path: "/child/data/shops/seed/inventory/1/initialStock", Value: json.Number("2")}, // 5→2 NOT restock
	})

	// Verify all changes payload
	if len(allChanges) != 2 {
		t.Fatalf("onStockChange: got %d, want 2", len(allChanges))
	}

	// Verify restock payload
	if len(restockPayload) != 1 {
		t.Fatalf("onRestock: got %d, want 1", len(restockPayload))
	}
	r := restockPayload[0]
	if r.Item.ItemID() != "Bamboo" {
		t.Errorf("restock item: got %q, want Bamboo", r.Item.ItemID())
	}
	if r.ShopType != "seed" {
		t.Errorf("restock ShopType: got %q, want seed", r.ShopType)
	}
	if r.OldStock != 0 {
		t.Errorf("restock OldStock: got %d, want 0", r.OldStock)
	}
	if r.NewStock != 3 {
		t.Errorf("restock NewStock: got %d, want 3", r.NewStock)
	}

	// Verify non-restock change has correct payload too
	byItem := make(map[string]StockChange)
	for _, ch := range allChanges {
		byItem[ch.Item.ItemID()] = ch
	}
	carrot := byItem["Carrot"]
	if carrot.OldStock != 5 || carrot.NewStock != 2 {
		t.Errorf("Carrot change: got %d→%d, want 5→2", carrot.OldStock, carrot.NewStock)
	}
}

func TestHandlePartialState_NoChangesNoCallbacks(t *testing.T) {
	c := testClient(t)
	c.state.SetFromWelcome(map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 5},
		}},
	})

	var restockCalled, changeCalled bool
	c.OnRestock(func(changes []StockChange) { restockCalled = true })
	c.OnStockChange(func(changes []StockChange) { changeCalled = true })

	c.handlePartialState([]Patch{
		{Op: "replace", Path: "/child/data/shops/seed/inventory/0/initialStock", Value: json.Number("5")},
	})

	if restockCalled {
		t.Error("onRestock should not fire for no-op patch")
	}
	if changeCalled {
		t.Error("onStockChange should not fire for no-op patch")
	}
}

func TestHandlePartialState_NilCallbacksSafe(t *testing.T) {
	c := testClient(t)
	c.state.SetFromWelcome(map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 0},
		}},
	})

	// No callbacks registered -- must not panic
	c.handlePartialState([]Patch{
		{Op: "replace", Path: "/child/data/shops/seed/inventory/0/initialStock", Value: json.Number("5")},
	})

	shop, _ := c.state.GetShop("seed")
	if shop.Inventory[0].InitialStock != 5 {
		t.Errorf("state should update even with nil callbacks: got %d, want 5", shop.Inventory[0].InitialStock)
	}
}

func TestHandlePartialState_OnlyRestockCallbackSet(t *testing.T) {
	c := testClient(t)
	c.state.SetFromWelcome(map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 0},
		}},
	})

	var restockPayload []StockChange
	c.OnRestock(func(changes []StockChange) { restockPayload = changes })
	// onStockChange is nil

	c.handlePartialState([]Patch{
		{Op: "replace", Path: "/child/data/shops/seed/inventory/0/initialStock", Value: json.Number("5")},
	})

	if len(restockPayload) != 1 {
		t.Fatalf("onRestock should fire: got %d, want 1", len(restockPayload))
	}
	if restockPayload[0].OldStock != 0 || restockPayload[0].NewStock != 5 {
		t.Errorf("payload: got %d→%d, want 0→5", restockPayload[0].OldStock, restockPayload[0].NewStock)
	}
}

func TestHandlePartialState_OnlyStockChangeCallbackSet(t *testing.T) {
	c := testClient(t)
	c.state.SetFromWelcome(map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 0},
		}},
	})

	var changePayload []StockChange
	c.OnStockChange(func(changes []StockChange) { changePayload = changes })
	// onRestock is nil

	c.handlePartialState([]Patch{
		{Op: "replace", Path: "/child/data/shops/seed/inventory/0/initialStock", Value: json.Number("5")},
	})

	if len(changePayload) != 1 {
		t.Fatalf("onStockChange should fire: got %d, want 1", len(changePayload))
	}
	ch := changePayload[0]
	if ch.Item.ItemID() != "Bamboo" || ch.OldStock != 0 || ch.NewStock != 5 || ch.ShopType != "seed" {
		t.Errorf("payload: got item=%q shop=%q %d→%d, want Bamboo seed 0→5",
			ch.Item.ItemID(), ch.ShopType, ch.OldStock, ch.NewStock)
	}
}

// --- Concurrent access ---

func TestShopState_ConcurrentApplyAndRead(t *testing.T) {
	state := NewShopState()
	state.SetFromWelcome(map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 0},
		}, SecondsUntilRestock: 300},
	})

	var wg sync.WaitGroup
	var readCount atomic.Int64

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			state.ApplyPatches([]Patch{
				{Op: "replace", Path: "/child/data/shops/seed/inventory/0/initialStock", Value: json.Number("5")},
			})
			state.ApplyPatches([]Patch{
				{Op: "replace", Path: "/child/data/shops/seed/inventory/0/initialStock", Value: json.Number("0")},
			})
		}
	}()

	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				shop, ok := state.GetShop("seed")
				if ok && shop != nil {
					readCount.Add(1)
					stock := shop.Inventory[0].InitialStock
					if stock != 0 && stock != 5 {
						t.Errorf("unexpected stock value %d (should be 0 or 5)", stock)
					}
				}
				_ = state.GetAllShops()
			}
		}()
	}

	wg.Wait()
	if readCount.Load() == 0 {
		t.Error("readers never succeeded")
	}
}

// --- ApplyPatches edge cases ---

func TestApplyPatches_Idempotent(t *testing.T) {
	state := NewShopState()
	state.SetFromWelcome(map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 0},
		}},
	})

	patch := []Patch{
		{Op: "replace", Path: "/child/data/shops/seed/inventory/0/initialStock", Value: json.Number("5")},
	}

	changes1 := state.ApplyPatches(patch)
	if len(changes1) != 1 {
		t.Fatalf("first apply: got %d changes, want 1", len(changes1))
	}

	changes2 := state.ApplyPatches(patch)
	if len(changes2) != 0 {
		t.Fatalf("second apply: got %d changes, want 0 (idempotent)", len(changes2))
	}

	// State should be 5 after both applies
	shop, _ := state.GetShop("seed")
	if shop.Inventory[0].InitialStock != 5 {
		t.Errorf("state: got %d, want 5", shop.Inventory[0].InitialStock)
	}
}

func TestApplyPatches_MixedValidAndInvalid(t *testing.T) {
	state := NewShopState()
	state.SetFromWelcome(map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 0},
			{ItemType: "Seed", Species: "Carrot", InitialStock: 0},
		}},
	})

	changes := state.ApplyPatches([]Patch{
		{Op: "replace", Path: "/child/data/shops/seed/inventory/0/initialStock", Value: json.Number("5")},   // valid
		{Op: "replace", Path: "/child/data/shops/seed/inventory/99/initialStock", Value: json.Number("3")},  // invalid index
		{Op: "replace", Path: "/child/data/shops/seed/inventory/1/initialStock", Value: json.Number("abc")}, // invalid value
		{Op: "replace", Path: "/child/data/shops/seed/inventory/1/initialStock", Value: json.Number("7")},   // valid
	})

	if len(changes) != 2 {
		t.Fatalf("expected 2 valid changes, got %d", len(changes))
	}

	shop, _ := state.GetShop("seed")
	if shop.Inventory[0].InitialStock != 5 {
		t.Errorf("Bamboo: got %d, want 5", shop.Inventory[0].InitialStock)
	}
	if shop.Inventory[1].InitialStock != 7 {
		t.Errorf("Carrot: got %d, want 7", shop.Inventory[1].InitialStock)
	}

	// Verify change payloads
	byItem := make(map[string]StockChange)
	for _, ch := range changes {
		byItem[ch.Item.ItemID()] = ch
	}
	if ch := byItem["Bamboo"]; ch.OldStock != 0 || ch.NewStock != 5 {
		t.Errorf("Bamboo change: got %d→%d, want 0→5", ch.OldStock, ch.NewStock)
	}
	if ch := byItem["Carrot"]; ch.OldStock != 0 || ch.NewStock != 7 {
		t.Errorf("Carrot change: got %d→%d, want 0→7", ch.OldStock, ch.NewStock)
	}
}

// --- StockChange.Item semantic verification ---

func TestStockChange_ItemInitialStockEqualsNewStock(t *testing.T) {
	state := NewShopState()
	state.SetFromWelcome(map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 0},
		}},
	})

	changes := state.ApplyPatches([]Patch{
		{Op: "replace", Path: "/child/data/shops/seed/inventory/0/initialStock", Value: json.Number("5")},
	})

	if len(changes) != 1 {
		t.Fatalf("got %d changes, want 1", len(changes))
	}
	ch := changes[0]

	// Verify all fields explicitly
	if ch.OldStock != 0 {
		t.Errorf("OldStock: got %d, want 0", ch.OldStock)
	}
	if ch.NewStock != 5 {
		t.Errorf("NewStock: got %d, want 5", ch.NewStock)
	}
	if ch.ShopType != "seed" {
		t.Errorf("ShopType: got %q, want seed", ch.ShopType)
	}
	if ch.Item.ItemType != "Seed" {
		t.Errorf("Item.ItemType: got %q, want Seed", ch.Item.ItemType)
	}
	if ch.Item.Species != "Bamboo" {
		t.Errorf("Item.Species: got %q, want Bamboo", ch.Item.Species)
	}
	// Document: Item.InitialStock captures the post-mutation value
	if ch.Item.InitialStock != ch.NewStock {
		t.Errorf("Item.InitialStock (%d) should equal NewStock (%d)", ch.Item.InitialStock, ch.NewStock)
	}
}
