package mg

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// StockChange represents a detected inventory transition.
type StockChange struct {
	ShopType string
	Item     ShopItem
	OldStock int
	NewStock int
}

// ShopState holds the current in-memory state of all shops.
type ShopState struct {
	mu    sync.RWMutex
	shops map[string]*Shop
}

// NewShopState creates an empty ShopState.
func NewShopState() *ShopState {
	return &ShopState{
		shops: make(map[string]*Shop),
	}
}

// SetFromWelcome initializes the shop state from a Welcome message.
func (s *ShopState) SetFromWelcome(shops map[string]*Shop) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.shops = shops
}

// GetShop returns a copy of a shop by type (seed, tool, egg, decor).
func (s *ShopState) GetShop(shopType string) (*Shop, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	shop, ok := s.shops[shopType]
	if !ok {
		return nil, false
	}
	cp := *shop
	items := make([]ShopItem, len(shop.Inventory))
	copy(items, shop.Inventory)
	cp.Inventory = items
	return &cp, true
}

// GetAllShops returns a snapshot of all shops.
func (s *ShopState) GetAllShops() map[string]*Shop {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]*Shop, len(s.shops))
	for k, v := range s.shops {
		cp := *v
		items := make([]ShopItem, len(v.Inventory))
		copy(items, v.Inventory)
		cp.Inventory = items
		result[k] = &cp
	}
	return result
}

// inventoryPathRegex matches paths like /child/data/shops/seed/inventory/3/initialStock
var inventoryPathRegex = regexp.MustCompile(
	`^/child/data/shops/(\w+)/inventory/(\d+)/initialStock$`,
)

// timerPathRegex matches paths like /child/data/shops/seed/secondsUntilRestock
var timerPathRegex = regexp.MustCompile(
	`^/child/data/shops/(\w+)/secondsUntilRestock$`,
)

// ApplyPatches applies a set of PartialState patches and returns any stock changes.
func (s *ShopState) ApplyPatches(patches []Patch) []StockChange {
	s.mu.Lock()
	defer s.mu.Unlock()

	var changes []StockChange

	for _, p := range patches {
		if m := inventoryPathRegex.FindStringSubmatch(p.Path); m != nil {
			shopType := m[1]
			idx, err := strconv.Atoi(m[2])
			if err != nil {
				continue
			}
			shop, ok := s.shops[shopType]
			if !ok || idx >= len(shop.Inventory) {
				continue
			}
			newStock, err := strconv.Atoi(string(p.Value))
			if err != nil {
				continue
			}
			oldStock := shop.Inventory[idx].InitialStock
			shop.Inventory[idx].InitialStock = newStock

			if oldStock != newStock {
				changes = append(changes, StockChange{
					ShopType: shopType,
					Item:     shop.Inventory[idx],
					OldStock: oldStock,
					NewStock: newStock,
				})
			}
		} else if m := timerPathRegex.FindStringSubmatch(p.Path); m != nil {
			shopType := m[1]
			shop, ok := s.shops[shopType]
			if !ok {
				continue
			}
			val, err := p.Value.Float64()
			if err != nil {
				continue
			}
			shop.SecondsUntilRestock = val
		}
	}

	return changes
}

// FormatItemName converts a camelCase item ID to a human-readable name.
func FormatItemName(id string) string {
	if id == "" {
		return ""
	}
	// Insert space before uppercase letters
	var result strings.Builder
	for i, r := range id {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteByte(' ')
		}
		result.WriteRune(r)
	}
	return result.String()
}

// FormatStock returns a human-readable stock string.
func FormatStock(stock int) string {
	if stock <= 0 {
		return "OUT OF STOCK"
	}
	return fmt.Sprintf("x%d in stock", stock)
}
