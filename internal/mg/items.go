package mg

// Item represents a subscribable item in the game.
type Item struct {
	ID       string // Canonical ID (e.g., "Banana", "Shovel")
	Name     string // Display name (e.g., "Banana Seed", "Shovel")
	ShopType string // "seed", "tool", "egg", "decor"
}

// AllItems is the complete catalog of subscribable items.
// This allows users to subscribe to items even when they're not in the current shop rotation.
var AllItems = []Item{
	// Seeds
	{ID: "Aloe", Name: "Aloe Seed", ShopType: "seed"},
	{ID: "Apple", Name: "Apple Seed", ShopType: "seed"},
	{ID: "Bamboo", Name: "Bamboo Seed", ShopType: "seed"},
	{ID: "Banana", Name: "Banana Seed", ShopType: "seed"},
	{ID: "Beet", Name: "Beet Seed", ShopType: "seed"},
	{ID: "Blueberry", Name: "Blueberry Seed", ShopType: "seed"},
	{ID: "BurrosTail", Name: "Burro's Tail Cutting", ShopType: "seed"},
	{ID: "Cabbage", Name: "Cabbage Seed", ShopType: "seed"},
	{ID: "Camellia", Name: "Camellia Seed", ShopType: "seed"},
	{ID: "Carrot", Name: "Carrot Seed", ShopType: "seed"},
	{ID: "Clover", Name: "Clover", ShopType: "seed"},
	{ID: "Coconut", Name: "Coconut Seed", ShopType: "seed"},
	{ID: "Corn", Name: "Corn Kernel", ShopType: "seed"},
	{ID: "Daffodil", Name: "Daffodil Seed", ShopType: "seed"},
	{ID: "Echeveria", Name: "Echeveria Cutting", ShopType: "seed"},
	{ID: "FavaBean", Name: "Fava Bean", ShopType: "seed"},
	{ID: "Gentian", Name: "Gentian Seed", ShopType: "seed"},
	{ID: "Grape", Name: "Grape Seed", ShopType: "seed"},
	{ID: "Lemon", Name: "Lemon Seed", ShopType: "seed"},
	{ID: "Lily", Name: "Lily Seed", ShopType: "seed"},
	{ID: "Lychee", Name: "Lychee Seed", ShopType: "seed"},
	{ID: "Mushroom", Name: "Mushroom Spore", ShopType: "seed"},
	{ID: "Peach", Name: "Peach Seed", ShopType: "seed"},
	{ID: "Pear", Name: "Pear Seed", ShopType: "seed"},
	{ID: "Pumpkin", Name: "Pumpkin Seed", ShopType: "seed"},
	{ID: "Strawberry", Name: "Strawberry Seed", ShopType: "seed"},
	{ID: "Tomato", Name: "Tomato Seed", ShopType: "seed"},
	{ID: "Tulip", Name: "Tulip Seed", ShopType: "seed"},
	{ID: "Watermelon", Name: "Watermelon Seed", ShopType: "seed"},
	{ID: "Rose", Name: "Rose Seed", ShopType: "seed"},
	{ID: "Delphinium", Name: "Delphinium Seed", ShopType: "seed"},
	{ID: "PineTree", Name: "Pine Tree Seed", ShopType: "seed"},
	{ID: "Squash", Name: "Squash Seed", ShopType: "seed"},
	{ID: "Cactus", Name: "Cactus Seed", ShopType: "seed"},
	{ID: "Poinsettia", Name: "Poinsettia Seed", ShopType: "seed"},
	{ID: "VioletCort", Name: "Violet Cort Spore", ShopType: "seed"},
	{ID: "Chrysanthemum", Name: "Chrysanthemum Seed", ShopType: "seed"},
	{ID: "Date", Name: "Date Seed", ShopType: "seed"},
	{ID: "Pepper", Name: "Pepper Seed", ShopType: "seed"},
	{ID: "PassionFruit", Name: "Passion Fruit Seed", ShopType: "seed"},
	{ID: "DragonFruit", Name: "Dragon Fruit Seed", ShopType: "seed"},
	{ID: "Cacao", Name: "Cacao Seed", ShopType: "seed"},
	{ID: "Sunflower", Name: "Sunflower Seed", ShopType: "seed"},
	{ID: "Starweaver", Name: "Starweaver Seed", ShopType: "seed"},
	{ID: "Dawnbinder", Name: "Dawnbinder Seed", ShopType: "seed"},
	{ID: "Moonbinder", Name: "Moonbinder Seed", ShopType: "seed"},

	// Tools
	{ID: "WateringCan", Name: "Watering Can", ShopType: "tool"},
	{ID: "PlanterPot", Name: "Planter Pot", ShopType: "tool"},
	{ID: "CropCleanser", Name: "Crop Cleanser", ShopType: "tool"},
	{ID: "Shovel", Name: "Shovel", ShopType: "tool"},
	{ID: "FeedingTrough", Name: "Feeding Trough", ShopType: "tool"},
	{ID: "DecorShed", Name: "Decor Shed", ShopType: "tool"},
	{ID: "PetHutch", Name: "Pet Hutch", ShopType: "tool"},
	{ID: "SeedSilo", Name: "Seed Silo", ShopType: "tool"},

	// Eggs
	{ID: "MythicalEgg", Name: "Mythical Egg", ShopType: "egg"},
	{ID: "RareEgg", Name: "Rare Egg", ShopType: "egg"},
	{ID: "CommonEgg", Name: "Common Egg", ShopType: "egg"},
	{ID: "UncommonEgg", Name: "Uncommon Egg", ShopType: "egg"},
	{ID: "LegendaryEgg", Name: "Legendary Egg", ShopType: "egg"},
	{ID: "EpicEgg", Name: "Epic Egg", ShopType: "egg"},
	{ID: "WinterEgg", Name: "Winter Egg", ShopType: "egg"},
	{ID: "SnowEgg", Name: "Snow Egg", ShopType: "egg"},
	{ID: "HorseEgg", Name: "Horse Egg", ShopType: "egg"},

	// Decor - Rocks
	{ID: "SmallGardenRock", Name: "Small Garden Rock", ShopType: "decor"},
	{ID: "MediumGardenRock", Name: "Medium Garden Rock", ShopType: "decor"},
	{ID: "LargeGardenRock", Name: "Large Garden Rock", ShopType: "decor"},

	// Decor - Misc
	{ID: "HayBale", Name: "Hay Bale", ShopType: "decor"},
	{ID: "StringLights", Name: "String Lights", ShopType: "decor"},
	{ID: "ColoredStringLights", Name: "Colored String Lights", ShopType: "decor"},
	{ID: "PaperLantern", Name: "Paper Lantern", ShopType: "decor"},
	{ID: "FanousLantern", Name: "Fanous Lantern", ShopType: "decor"},

	// Decor - Gravestones (Halloween)
	{ID: "SmallGravestone", Name: "Small Gravestone", ShopType: "decor"},
	{ID: "MediumGravestone", Name: "Medium Gravestone", ShopType: "decor"},
	{ID: "LargeGravestone", Name: "Large Gravestone", ShopType: "decor"},
	{ID: "Cauldron", Name: "Cauldron", ShopType: "decor"},

	// Decor - Wood tier
	{ID: "WoodCaribou", Name: "Wood Caribou", ShopType: "decor"},
	{ID: "WoodBench", Name: "Wood Bench", ShopType: "decor"},
	{ID: "WoodArch", Name: "Wood Arch", ShopType: "decor"},
	{ID: "WoodPergola", Name: "Wood Pergola", ShopType: "decor"},
	{ID: "WoodBridge", Name: "Wood Bridge", ShopType: "decor"},
	{ID: "WoodLampPost", Name: "Wood Lamp Post", ShopType: "decor"},
	{ID: "WoodOwl", Name: "Wood Owl", ShopType: "decor"},
	{ID: "WoodBirdhouse", Name: "Wood Birdhouse", ShopType: "decor"},
	{ID: "WoodWindmill", Name: "Wood Windmill", ShopType: "decor"},

	// Decor - Stone tier
	{ID: "StoneCaribou", Name: "Stone Caribou", ShopType: "decor"},
	{ID: "StoneBench", Name: "Stone Bench", ShopType: "decor"},
	{ID: "StoneArch", Name: "Stone Arch", ShopType: "decor"},
	{ID: "StoneBridge", Name: "Stone Bridge", ShopType: "decor"},
	{ID: "StoneLampPost", Name: "Stone Lamp Post", ShopType: "decor"},
	{ID: "StoneGnome", Name: "Stone Gnome", ShopType: "decor"},
	{ID: "StoneBirdbath", Name: "Stone Birdbath", ShopType: "decor"},

	// Decor - Marble tier
	{ID: "MarbleCaribou", Name: "Marble Caribou", ShopType: "decor"},
	{ID: "MarbleBench", Name: "Marble Bench", ShopType: "decor"},
	{ID: "MarbleArch", Name: "Marble Arch", ShopType: "decor"},
	{ID: "MarbleBridge", Name: "Marble Bridge", ShopType: "decor"},
	{ID: "MarbleLampPost", Name: "Marble Lamp Post", ShopType: "decor"},
	{ID: "MarbleBlobling", Name: "Marble Blobling", ShopType: "decor"},
	{ID: "MarbleFountain", Name: "Marble Fountain", ShopType: "decor"},

	// Decor - Fairy/Special
	{ID: "MiniFairyCottage", Name: "Mini Fairy Cottage", ShopType: "decor"},
	{ID: "MiniFairyForge", Name: "Mini Fairy Forge", ShopType: "decor"},
	{ID: "MiniFairyKeep", Name: "Mini Fairy Keep", ShopType: "decor"},
	{ID: "MiniWizardTower", Name: "Mini Wizard Tower", ShopType: "decor"},
	{ID: "StrawScarecrow", Name: "Straw Scarecrow", ShopType: "decor"},
}

// GetAllItems returns all subscribable items.
func GetAllItems() []Item {
	return AllItems
}

// GetItemByID returns an item by its ID (case-insensitive).
func GetItemByID(id string) *Item {
	normalized := normalizeItemID(id)
	for i := range AllItems {
		if normalizeItemID(AllItems[i].ID) == normalized {
			return &AllItems[i]
		}
	}
	return nil
}
