package mg

import (
	"encoding/json"
	"testing"
)

// --- ApplyPatches: restock detection ---

func TestApplyPatches_RestockZeroToN(t *testing.T) {
	state := NewShopState()
	state.SetFromWelcome(map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 0},
			{ItemType: "Seed", Species: "Carrot", InitialStock: 5},
		}},
	})

	changes := state.ApplyPatches([]Patch{
		{Op: "replace", Path: "/child/data/shops/seed/inventory/0/initialStock", Value: json.Number("3")},
	})

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	ch := changes[0]
	if ch.OldStock != 0 || ch.NewStock != 3 {
		t.Errorf("expected 0→3, got %d→%d", ch.OldStock, ch.NewStock)
	}
	if ch.Item.ItemID() != "Bamboo" {
		t.Errorf("expected Bamboo, got %s", ch.Item.ItemID())
	}
	if ch.ShopType != "seed" {
		t.Errorf("expected seed, got %s", ch.ShopType)
	}
}

func TestApplyPatches_StockDecreaseNotRestock(t *testing.T) {
	state := NewShopState()
	state.SetFromWelcome(map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 5},
		}},
	})

	changes := state.ApplyPatches([]Patch{
		{Op: "replace", Path: "/child/data/shops/seed/inventory/0/initialStock", Value: json.Number("3")},
	})

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	// This is a stock change but NOT a restock (5→3, not 0→N)
	if changes[0].OldStock != 5 || changes[0].NewStock != 3 {
		t.Errorf("expected 5→3, got %d→%d", changes[0].OldStock, changes[0].NewStock)
	}
}

func TestApplyPatches_NoChangeNoEvent(t *testing.T) {
	state := NewShopState()
	state.SetFromWelcome(map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 5},
		}},
	})

	changes := state.ApplyPatches([]Patch{
		{Op: "replace", Path: "/child/data/shops/seed/inventory/0/initialStock", Value: json.Number("5")},
	})

	if len(changes) != 0 {
		t.Fatalf("expected no changes when stock is the same, got %d", len(changes))
	}
}

func TestApplyPatches_SellOutZero(t *testing.T) {
	state := NewShopState()
	state.SetFromWelcome(map[string]*Shop{
		"egg": {Inventory: []ShopItem{
			{ItemType: "Egg", EggID: "MythicalEgg", InitialStock: 2},
		}},
	})

	changes := state.ApplyPatches([]Patch{
		{Op: "replace", Path: "/child/data/shops/egg/inventory/0/initialStock", Value: json.Number("0")},
	})

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].OldStock != 2 || changes[0].NewStock != 0 {
		t.Errorf("expected 2→0, got %d→%d", changes[0].OldStock, changes[0].NewStock)
	}
}

func TestApplyPatches_MultipleShops(t *testing.T) {
	state := NewShopState()
	state.SetFromWelcome(map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 0},
		}},
		"egg": {Inventory: []ShopItem{
			{ItemType: "Egg", EggID: "CommonEgg", InitialStock: 0},
		}},
	})

	changes := state.ApplyPatches([]Patch{
		{Op: "replace", Path: "/child/data/shops/seed/inventory/0/initialStock", Value: json.Number("5")},
		{Op: "replace", Path: "/child/data/shops/egg/inventory/0/initialStock", Value: json.Number("10")},
	})

	if len(changes) != 2 {
		t.Fatalf("expected 2 changes, got %d", len(changes))
	}
}

func TestApplyPatches_InvalidIndex(t *testing.T) {
	state := NewShopState()
	state.SetFromWelcome(map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 0},
		}},
	})

	// Index 99 is out of bounds
	changes := state.ApplyPatches([]Patch{
		{Op: "replace", Path: "/child/data/shops/seed/inventory/99/initialStock", Value: json.Number("5")},
	})

	if len(changes) != 0 {
		t.Fatalf("expected no changes for out-of-bounds index, got %d", len(changes))
	}
}

func TestApplyPatches_UnknownShop(t *testing.T) {
	state := NewShopState()
	state.SetFromWelcome(map[string]*Shop{
		"seed": {Inventory: []ShopItem{}},
	})

	changes := state.ApplyPatches([]Patch{
		{Op: "replace", Path: "/child/data/shops/nonexistent/inventory/0/initialStock", Value: json.Number("5")},
	})

	if len(changes) != 0 {
		t.Fatalf("expected no changes for unknown shop, got %d", len(changes))
	}
}

func TestApplyPatches_InvalidValue(t *testing.T) {
	state := NewShopState()
	state.SetFromWelcome(map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 0},
		}},
	})

	changes := state.ApplyPatches([]Patch{
		{Op: "replace", Path: "/child/data/shops/seed/inventory/0/initialStock", Value: json.Number("notanumber")},
	})

	if len(changes) != 0 {
		t.Fatalf("expected no changes for invalid value, got %d", len(changes))
	}
}

// --- ApplyPatches: timer handling ---

func TestApplyPatches_TimerUpdate(t *testing.T) {
	state := NewShopState()
	state.SetFromWelcome(map[string]*Shop{
		"seed": {
			Inventory:           []ShopItem{},
			SecondsUntilRestock: 100.0,
		},
	})

	state.ApplyPatches([]Patch{
		{Op: "replace", Path: "/child/data/shops/seed/secondsUntilRestock", Value: json.Number("95")},
	})

	shop, ok := state.GetShop("seed")
	if !ok {
		t.Fatal("seed shop not found")
	}
	if shop.SecondsUntilRestock != 95 {
		t.Errorf("expected timer 95, got %f", shop.SecondsUntilRestock)
	}
}

func TestApplyPatches_TimerResetDetectsCycle(t *testing.T) {
	state := NewShopState()
	state.SetFromWelcome(map[string]*Shop{
		"seed": {
			Inventory:           []ShopItem{},
			SecondsUntilRestock: 5.0, // about to expire
		},
	})

	// Timer jumps from 5 → 300 = restock cycle reset
	state.ApplyPatches([]Patch{
		{Op: "replace", Path: "/child/data/shops/seed/secondsUntilRestock", Value: json.Number("300")},
	})

	shop, _ := state.GetShop("seed")
	if shop.RestockCycle != 300 {
		t.Errorf("expected cycle 300, got %f", shop.RestockCycle)
	}
}

func TestApplyPatches_TimerCountdownDoesNotResetCycle(t *testing.T) {
	state := NewShopState()
	state.SetFromWelcome(map[string]*Shop{
		"seed": {
			Inventory:           []ShopItem{},
			SecondsUntilRestock: 100.0,
		},
	})

	state.ApplyPatches([]Patch{
		{Op: "replace", Path: "/child/data/shops/seed/secondsUntilRestock", Value: json.Number("95")},
	})

	shop, _ := state.GetShop("seed")
	if shop.RestockCycle != 0 {
		t.Errorf("expected no cycle set on countdown, got %f", shop.RestockCycle)
	}
}

// --- ShopState thread safety ---

func TestShopState_GetShopReturnsCopy(t *testing.T) {
	state := NewShopState()
	state.SetFromWelcome(map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 5},
		}},
	})

	shop, _ := state.GetShop("seed")
	shop.Inventory[0].InitialStock = 999

	original, _ := state.GetShop("seed")
	if original.Inventory[0].InitialStock != 5 {
		t.Error("GetShop did not return a copy; mutation leaked into state")
	}
}

func TestShopState_GetAllShopsReturnsCopy(t *testing.T) {
	state := NewShopState()
	state.SetFromWelcome(map[string]*Shop{
		"seed": {Inventory: []ShopItem{
			{ItemType: "Seed", Species: "Bamboo", InitialStock: 5},
		}},
	})

	shops := state.GetAllShops()
	shops["seed"].Inventory[0].InitialStock = 999

	original := state.GetAllShops()
	if original["seed"].Inventory[0].InitialStock != 5 {
		t.Error("GetAllShops did not return a copy; mutation leaked into state")
	}
}

func TestShopState_GetShopNotFound(t *testing.T) {
	state := NewShopState()
	_, ok := state.GetShop("nonexistent")
	if ok {
		t.Error("expected false for nonexistent shop")
	}
}

// --- FormatItemName ---

func TestFormatItemName_KnownItems(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"Bamboo", "Bamboo Seed"},
		{"MythicalEgg", "Mythical Egg"},
		{"WateringCan", "Watering Can"},
		{"MiniFairyCottage", "Mini Fairy Cottage"},
		{"Carrot", "Carrot Seed"},
		{"Date", "Date Seed"},
		{"Cactus", "Cactus Seed"},
	}
	for _, tt := range tests {
		got := FormatItemName(tt.input)
		if got != tt.expected {
			t.Errorf("FormatItemName(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestFormatItemName_UnknownFallbackCamelCase(t *testing.T) {
	got := FormatItemName("SuperRareThing")
	if got != "Super Rare Thing" {
		t.Errorf("expected camelCase split, got %q", got)
	}
}

func TestFormatItemName_Empty(t *testing.T) {
	got := FormatItemName("")
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

// --- FormatStock ---

func TestFormatStock(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "OUT OF STOCK"},
		{-1, "OUT OF STOCK"},
		{1, "x1 in stock"},
		{10, "x10 in stock"},
	}
	for _, tt := range tests {
		got := FormatStock(tt.input)
		if got != tt.expected {
			t.Errorf("FormatStock(%d) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- ItemID ---

func TestShopItem_ItemID(t *testing.T) {
	tests := []struct {
		item     ShopItem
		expected string
	}{
		{ShopItem{ItemType: "Seed", Species: "Bamboo"}, "Bamboo"},
		{ShopItem{ItemType: "Tool", ToolID: "Shovel"}, "Shovel"},
		{ShopItem{ItemType: "Egg", EggID: "MythicalEgg"}, "MythicalEgg"},
		{ShopItem{ItemType: "Decor", DecorID: "SmallRock"}, "SmallRock"},
		{ShopItem{ItemType: "Unknown"}, ""},
	}
	for _, tt := range tests {
		got := tt.item.ItemID()
		if got != tt.expected {
			t.Errorf("ItemID() for %s = %q, want %q", tt.item.ItemType, got, tt.expected)
		}
	}
}
