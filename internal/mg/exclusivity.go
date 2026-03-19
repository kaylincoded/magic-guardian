package mg

import (
	"fmt"
	"strings"
)

// ItemExclusivity contains platform and server availability information for items.
type ItemExclusivity struct {
	NotOnDiscord bool     // Not available on Discord (iOS + web only)
	ServerOnly   []string // Only spawns on specific Discord server IDs
}

// exclusivityData maps item IDs to their exclusivity constraints.
// Data from Magic Garden Wiki: https://magicgarden.wiki/Crops (Exclusive column)
// NOTE: Apple shows "iOS/Web App" but Discord IS the web app, so Apple is NOT excluded.
var exclusivityData = map[string]ItemExclusivity{
	// Server-exclusive items based on Discord server ID patterns are in serverPatterns
}

// ServerIDPattern defines how to check if a guild ID matches exclusivity rules
type ServerIDPattern struct {
	EndsWithDigits []string // Guild ID must end with one of these digits
}

// serverPatterns maps item IDs to their server ID pattern requirements
var serverPatterns = map[string]ServerIDPattern{
	"Banana": {EndsWithDigits: []string{"0", "2", "4", "6", "8"}}, // even IDs
	"Grape":  {EndsWithDigits: []string{"1"}},
	"Lemon":  {EndsWithDigits: []string{"2"}},
	"Lychee": {EndsWithDigits: []string{"2"}},
}

// GetExclusivity returns the exclusivity info for an item, or nil if unrestricted.
func GetExclusivity(itemID string) *ItemExclusivity {
	if ex, ok := exclusivityData[itemID]; ok {
		return &ex
	}
	// Check if it has a server pattern (these are also exclusive)
	if _, ok := serverPatterns[itemID]; ok {
		return &ItemExclusivity{} // Has restrictions but not NotOnDiscord
	}
	return nil
}

// IsAvailableOnDiscord returns true if the item can appear on Discord.
func (e *ItemExclusivity) IsAvailableOnDiscord() bool {
	return !e.NotOnDiscord
}

// IsAvailableInGuild returns true if the item can appear in the given guild.
func IsAvailableInGuild(itemID, guildID string) bool {
	normalized := normalizeItemID(itemID)
	// Check if not on Discord at all
	if ex, ok := exclusivityData[normalized]; ok && ex.NotOnDiscord {
		return false
	}
	// Check server pattern
	pattern, hasPattern := serverPatterns[normalized]
	if !hasPattern {
		return true // No server restriction
	}
	if guildID == "" {
		return false // Can't check without guild ID
	}
	lastChar := guildID[len(guildID)-1:]
	for _, digit := range pattern.EndsWithDigits {
		if lastChar == digit {
			return true
		}
	}
	return false
}

// normalizeItemID converts item ID to title case for lookup
func normalizeItemID(itemID string) string {
	if len(itemID) == 0 {
		return itemID
	}
	// Convert first letter to uppercase, rest stays as-is
	return strings.ToUpper(itemID[:1]) + itemID[1:]
}

// FormatExclusivityBadge returns a badge string for display.
func FormatExclusivityBadge(itemID string) string {
	normalized := normalizeItemID(itemID)
	if ex, ok := exclusivityData[normalized]; ok && ex.NotOnDiscord {
		return "🚫 Not on Discord"
	}
	if _, ok := serverPatterns[normalized]; ok {
		return "🔒 Server Exclusive"
	}
	return ""
}

// FormatExclusivityBadgeShort returns a short badge for autocomplete labels.
func FormatExclusivityBadgeShort(itemID string) string {
	normalized := normalizeItemID(itemID)
	if ex, ok := exclusivityData[normalized]; ok && ex.NotOnDiscord {
		return "🚫"
	}
	if _, ok := serverPatterns[normalized]; ok {
		return "🔒"
	}
	return ""
}

// FormatExclusivityDetail returns a detailed explanation for subscribe/unsubscribe commands.
func FormatExclusivityDetail(itemID string, currentGuildID string) string {
	normalized := normalizeItemID(itemID)
	if ex, ok := exclusivityData[normalized]; ok && ex.NotOnDiscord {
		return "⚠️ This item is **not available on Discord** and won't appear in Discord shops."
	}
	pattern, hasPattern := serverPatterns[normalized]
	if hasPattern {
		available := IsAvailableInGuild(normalized, currentGuildID)
		serverDesc := formatServerPattern(pattern)
		if available {
			return fmt.Sprintf("ℹ️ This item is **server exclusive** — available here and on servers %s.", serverDesc)
		}
		return fmt.Sprintf("⚠️ This item is **server exclusive** and won't appear in this server. Only available on servers %s.", serverDesc)
	}
	return ""
}

// formatServerPattern returns a human-readable description of the server pattern
func formatServerPattern(p ServerIDPattern) string {
	if len(p.EndsWithDigits) == 5 {
		// Check if it's even digits
		isEven := true
		for _, d := range p.EndsWithDigits {
			if d != "0" && d != "2" && d != "4" && d != "6" && d != "8" {
				isEven = false
				break
			}
		}
		if isEven {
			return "with even IDs"
		}
	}
	if len(p.EndsWithDigits) == 1 {
		return fmt.Sprintf("with ID ending in %s", p.EndsWithDigits[0])
	}
	return fmt.Sprintf("with ID ending in %v", p.EndsWithDigits)
}
