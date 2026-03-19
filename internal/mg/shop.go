package mg

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// StockChange represents a detected inventory transition.
type StockChange struct {
	ShopType  string
	Item      ShopItem
	OldStock  int
	NewStock  int
	IsRestock bool // true if this is from a shop restock event (timer reset)
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

// inventoryArrayRegex matches paths like /child/data/shops/seed/inventory
var inventoryArrayRegex = regexp.MustCompile(
	`^/child/data/shops/(\w+)/inventory$`,
)

// inventoryItemRegex matches paths like /child/data/shops/seed/inventory/3
var inventoryItemRegex = regexp.MustCompile(
	`^/child/data/shops/(\w+)/inventory/(\d+)$`,
)

// shopEntryRegex matches paths like /child/data/shops/tool (whole shop object replacement)
var shopEntryRegex = regexp.MustCompile(
	`^/child/data/shops/(seed|tool|egg|decor)$`,
)

// ApplyPatches applies a set of PartialState patches and returns any stock changes.
func (s *ShopState) ApplyPatches(patches []Patch) []StockChange {
	s.mu.Lock()
	defer s.mu.Unlock()

	var changes []StockChange

	for _, p := range patches {
		if p.Op == "replace" {
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
				// Sometimes values come boxed in quotes, e.g., "5". Clean them.
				if err != nil {
					cleanVal := strings.Trim(string(p.Value), `"`)
					newStock, err = strconv.Atoi(cleanVal)
					if err != nil {
						continue
					}
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
				// Clean potential quotes around the number
				cleanVal := strings.Trim(string(p.Value), `"`)
				val, err := strconv.ParseFloat(cleanVal, 64)
				if err != nil {
					continue
				}
				// Detect timer reset: new value jumps above old value = fresh cycle
				if val > shop.SecondsUntilRestock+10 {
					shop.RestockCycle = val
					// On restock, include ALL in-stock items from this shop
					// This ensures users get notified about items they're subscribed to
					// even if the stock didn't change (e.g., was already 3, still 3)
					for _, item := range shop.Inventory {
						if item.InitialStock > 0 {
							changes = append(changes, StockChange{
								ShopType:  shopType,
								Item:      item,
								OldStock:  item.InitialStock, // same as new, but marks as "in stock"
								NewStock:  item.InitialStock,
								IsRestock: true, // flag to indicate this is a restock notification
							})
						}
					}
				}
				shop.SecondsUntilRestock = val
			} else if m := inventoryArrayRegex.FindStringSubmatch(p.Path); m != nil {
				// Whole inventory array replaced
				shopType := m[1]
				shop, ok := s.shops[shopType]
				if !ok {
					continue
				}
				var newInventory []ShopItem
				if err := json.Unmarshal(p.Value, &newInventory); err == nil {
					oldShop := *shop
					oldShop.Inventory = make([]ShopItem, len(shop.Inventory))
					copy(oldShop.Inventory, shop.Inventory)

					shop.Inventory = newInventory

					// Compute differences
					newShops := map[string]*Shop{shopType: shop}
					oldShops := map[string]*Shop{shopType: &oldShop}
					changes = append(changes, diffShopState(oldShops, newShops)...)
				}
			} else if m := shopEntryRegex.FindStringSubmatch(p.Path); m != nil {
				// Whole shop object replaced (e.g., /child/data/shops/tool)
				shopType := m[1]
				oldShop, hadOld := s.shops[shopType]
				var oldShopCopy *Shop
				if hadOld {
					cp := *oldShop
					cp.Inventory = make([]ShopItem, len(oldShop.Inventory))
					copy(cp.Inventory, oldShop.Inventory)
					oldShopCopy = &cp
				}

				var newShop Shop
				if err := json.Unmarshal(p.Value, &newShop); err == nil {
					s.shops[shopType] = &newShop

					// Compute differences
					newShops := map[string]*Shop{shopType: &newShop}
					oldShops := map[string]*Shop{}
					if oldShopCopy != nil {
						oldShops[shopType] = oldShopCopy
					}
					changes = append(changes, diffShopState(oldShops, newShops)...)
				}
			}
		} else if p.Op == "remove" {
			if m := inventoryItemRegex.FindStringSubmatch(p.Path); m != nil {
				shopType := m[1]
				idx, err := strconv.Atoi(m[2])
				if err == nil {
					shop, ok := s.shops[shopType]
					if ok && idx >= 0 && idx < len(shop.Inventory) {
						// Remove the item from slice
						item := shop.Inventory[idx]
						shop.Inventory = append(shop.Inventory[:idx], shop.Inventory[idx+1:]...)

						changes = append(changes, StockChange{
							ShopType: shopType,
							Item:     item,
							OldStock: item.InitialStock,
							NewStock: 0,
						})
					}
				}
			}
		} else if p.Op == "add" {
			if m := inventoryItemRegex.FindStringSubmatch(p.Path); m != nil {
				shopType := m[1]
				idx, err := strconv.Atoi(m[2])
				if err == nil {
					shop, ok := s.shops[shopType]
					if ok && idx >= 0 && idx <= len(shop.Inventory) {
						var newItem ShopItem
						if err := json.Unmarshal(p.Value, &newItem); err == nil {
							// Insert into slice
							shop.Inventory = append(shop.Inventory[:idx], append([]ShopItem{newItem}, shop.Inventory[idx:]...)...)
						}
					}
				}
			}
		}
	}

	return changes
}

// itemDisplayNames maps game item IDs to their official display names.
// Extracted from the Magic Garden game client (version 117).
var itemDisplayNames = map[string]string{
	// Seeds
	"Carrot":        "Carrot Seed",
	"Cabbage":       "Cabbage Seed",
	"Strawberry":    "Strawberry Seed",
	"Aloe":          "Aloe Seed",
	"Beet":          "Beet Seed",
	"Rose":          "Rose Seed",
	"FavaBean":      "Fava Bean",
	"Delphinium":    "Delphinium Seed",
	"Blueberry":     "Blueberry Seed",
	"Apple":         "Apple Seed",
	"OrangeTulip":   "Tulip Seed",
	"Tomato":        "Tomato Seed",
	"Daffodil":      "Daffodil Seed",
	"Corn":          "Corn Kernel",
	"Watermelon":    "Watermelon Seed",
	"Pumpkin":       "Pumpkin Seed",
	"Echeveria":     "Echeveria Cutting",
	"Pear":          "Pear Seed",
	"Gentian":       "Gentian Seed",
	"Coconut":       "Coconut Seed",
	"PineTree":      "Pinecone",
	"Banana":        "Banana Seed",
	"Lily":          "Lily Seed",
	"Camellia":      "Camellia Seed",
	"Squash":        "Squash Seed",
	"Peach":         "Peach Seed",
	"BurrosTail":    "Burro's Tail Cutting",
	"Mushroom":      "Mushroom Spore",
	"Cactus":        "Cactus Seed",
	"Bamboo":        "Bamboo Seed",
	"Poinsettia":    "Poinsettia Seed",
	"VioletCort":    "Violet Cort Spore",
	"Chrysanthemum": "Chrysanthemum Seed",
	"Date":          "Date Seed",
	"Grape":         "Grape Seed",
	"Pepper":        "Pepper Seed",
	"Lemon":         "Lemon Seed",
	"PassionFruit":  "Passion Fruit Seed",
	"DragonFruit":   "Dragon Fruit Seed",
	"Cacao":         "Cacao Bean",
	"Lychee":        "Lychee Pit",
	"Sunflower":     "Sunflower Seed",
	"Starweaver":    "Starweaver Pod",
	"DawnCelestial": "Dawnbinder Pod",
	"MoonCelestial": "Moonbinder Pod",
	// Tools
	"WateringCan":    "Watering Can",
	"PlanterPot":     "Planter Pot",
	"CropCleanser":   "Crop Cleanser",
	"WetPotion":      "Wet Potion",
	"ChilledPotion":  "Chilled Potion",
	"DawnlitPotion":  "Dawnlit Potion",
	"Shovel":         "Garden Shovel",
	"FrozenPotion":   "Frozen Potion",
	"AmberlitPotion": "Amberlit Potion",
	"GoldPotion":     "Gold Potion",
	"RainbowPotion":  "Rainbow Potion",
	// Eggs
	"CommonEgg":    "Common Egg",
	"UncommonEgg":  "Uncommon Egg",
	"RareEgg":      "Rare Egg",
	"LegendaryEgg": "Legendary Egg",
	"SnowEgg":      "Snow Egg",
	"HorseEgg":     "Horse Egg",
	"MythicalEgg":  "Mythical Egg",
	"WinterEgg":    "Winter Egg",
	// Decor
	"SmallRock":           "Small Garden Rock",
	"MediumRock":          "Medium Garden Rock",
	"LargeRock":           "Large Garden Rock",
	"HayBale":             "Hay Bale",
	"StringLights":        "String Lights",
	"ColoredStringLights": "Colored String Lights",
	"PaperLantern":        "Paper Lantern",
	"FanousLantern":       "Fanous Lantern",
	"SmallGravestone":     "Small Gravestone",
	"WoodCaribou":         "Wood Caribou",
	"WoodBench":           "Wood Bench",
	"WoodArch":            "Wood Arch",
	"WoodPergola":         "Wood Pergola",
	"WoodBridge":          "Wood Bridge",
	"WoodLampPost":        "Wood Lamp Post",
	"WoodOwl":             "Wood Owl",
	"WoodBirdhouse":       "Wood Birdhouse",
	"WoodWindmill":        "Wood Windmill",
	"MediumGravestone":    "Medium Gravestone",
	"StoneCaribou":        "Stone Caribou",
	"StoneBench":          "Stone Bench",
	"StoneArch":           "Stone Arch",
	"StoneBridge":         "Stone Bridge",
	"StoneLampPost":       "Stone Lamp Post",
	"StoneGnome":          "Stone Gnome",
	"StoneBirdbath":       "Stone Birdbath",
	"LargeGravestone":     "Large Gravestone",
	"MarbleCaribou":       "Marble Caribou",
	"MarbleBench":         "Marble Bench",
	"MarbleArch":          "Marble Arch",
	"MarbleBridge":        "Marble Bridge",
	"MarbleLampPost":      "Marble Lamp Post",
	"MarbleBlobling":      "Marble Blobling",
	"MarbleFountain":      "Marble Fountain",
	"MiniFairyCottage":    "Mini Fairy Cottage",
	"Cauldron":            "Cauldron",
	"StrawScarecrow":      "Straw Scarecrow",
	"MiniFairyForge":      "Mini Fairy Forge",
	"MiniFairyKeep":       "Mini Fairy Keep",
	"MiniWizardTower":     "Mini Wizard Tower",
	"FeedingTrough":       "Feeding Trough",
	"DecorShed":           "Decor Shed",
	"PetHutch":            "Pet Hutch",
	"SeedSilo":            "Seed Silo",
}

// FormatItemName returns the official display name for a game item ID.
// Falls back to splitting camelCase if the ID is not in the registry.
func FormatItemName(id string) string {
	if id == "" {
		return ""
	}
	if name, ok := itemDisplayNames[id]; ok {
		return name
	}
	// Fallback: insert space before uppercase letters
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
