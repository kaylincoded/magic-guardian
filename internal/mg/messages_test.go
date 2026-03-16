package mg

import (
	"encoding/json"
	"strconv"
	"testing"
)

func TestServerMessage_Unmarshal_Welcome(t *testing.T) {
	raw := `{"type":"Welcome","fullState":{"scope":"room","data":{"roomId":"8GJG"},"child":{"scope":"quinoa","data":{"currentTime":1234,"shops":{}}}}}`
	var msg ServerMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.Type != "Welcome" {
		t.Errorf("expected Welcome, got %s", msg.Type)
	}
	if msg.FullState == nil {
		t.Error("expected fullState to be set")
	}
}

func TestServerMessage_Unmarshal_PartialState(t *testing.T) {
	raw := `{"type":"PartialState","patches":[{"op":"replace","path":"/child/data/shops/seed/inventory/0/initialStock","value":5}]}`
	var msg ServerMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.Type != "PartialState" {
		t.Errorf("expected PartialState, got %s", msg.Type)
	}
	if len(msg.Patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(msg.Patches))
	}
	if msg.Patches[0].Op != "replace" {
		t.Errorf("expected replace, got %s", msg.Patches[0].Op)
	}
	val, _ := strconv.ParseInt(string(msg.Patches[0].Value), 10, 64)
	if val != 5 {
		t.Errorf("expected value 5, got %d", val)
	}
}

func TestWelcomeState_Unmarshal(t *testing.T) {
	raw := `{
		"scope": "room",
		"data": {"roomId": "8GJG"},
		"child": {
			"scope": "quinoa",
			"data": {
				"currentTime": 1710500000,
				"shops": {
					"seed": {
						"inventory": [
							{"itemType": "Seed", "species": "Bamboo", "initialStock": 5},
							{"itemType": "Seed", "species": "Carrot", "initialStock": 0}
						],
						"secondsUntilRestock": 120.5
					}
				}
			}
		}
	}`
	var state WelcomeState
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if state.Data.RoomID != "8GJG" {
		t.Errorf("expected room 8GJG, got %s", state.Data.RoomID)
	}
	if len(state.Child.Data.Shops) != 1 {
		t.Fatalf("expected 1 shop, got %d", len(state.Child.Data.Shops))
	}
	seed := state.Child.Data.Shops["seed"]
	if len(seed.Inventory) != 2 {
		t.Fatalf("expected 2 items, got %d", len(seed.Inventory))
	}
	if seed.Inventory[0].Species != "Bamboo" {
		t.Errorf("expected Bamboo, got %s", seed.Inventory[0].Species)
	}
	if seed.Inventory[0].InitialStock != 5 {
		t.Errorf("expected stock 5, got %d", seed.Inventory[0].InitialStock)
	}
	if seed.SecondsUntilRestock != 120.5 {
		t.Errorf("expected timer 120.5, got %f", seed.SecondsUntilRestock)
	}
}

func TestShopItem_ItemID_AllTypes(t *testing.T) {
	tests := []struct {
		item     ShopItem
		expected string
	}{
		{ShopItem{ItemType: "Seed", Species: "Bamboo"}, "Bamboo"},
		{ShopItem{ItemType: "Tool", ToolID: "WateringCan"}, "WateringCan"},
		{ShopItem{ItemType: "Egg", EggID: "RareEgg"}, "RareEgg"},
		{ShopItem{ItemType: "Decor", DecorID: "MiniFairyCottage"}, "MiniFairyCottage"},
		{ShopItem{ItemType: ""}, ""},
		{ShopItem{ItemType: "FutureType"}, ""},
	}
	for _, tt := range tests {
		got := tt.item.ItemID()
		if got != tt.expected {
			t.Errorf("ItemID() for type=%q = %q, want %q", tt.item.ItemType, got, tt.expected)
		}
	}
}

func TestPatch_Unmarshal(t *testing.T) {
	raw := `{"op":"replace","path":"/child/data/shops/seed/secondsUntilRestock","value":95.5}`
	var p Patch
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.Op != "replace" {
		t.Errorf("expected replace, got %s", p.Op)
	}
	f, _ := strconv.ParseFloat(string(p.Value), 64)
	if f != 95.5 {
		t.Errorf("expected 95.5, got %f", f)
	}
}
