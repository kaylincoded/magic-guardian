package discord

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/kaylincoded/magic-guardian/internal/mg"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const (
	colorGreen  = 0x2ecc71
	colorRed    = 0xe74c3c
	colorBlue   = 0x3498db
	colorGold   = 0xf1c40f
	colorPurple = 0x9b59b6
)

var shopEmoji = map[string]string{
	"seed":  "🌱",
	"tool":  "🔧",
	"egg":   "🥚",
	"decor": "🎨",
}

// BuildStockAlertEmbed creates a rich embed for a batch of stock changes.
func BuildStockAlertEmbed(changes []mg.StockChange) *discordgo.MessageEmbed {
	var fields []*discordgo.MessageEmbedField

	// Group by shop type
	grouped := make(map[string][]mg.StockChange)
	for _, ch := range changes {
		grouped[ch.ShopType] = append(grouped[ch.ShopType], ch)
	}

	// DEBUG: Log what we're building
	fmt.Printf("[DEBUG] BuildStockAlertEmbed: Total changes=%d\n", len(changes))
	for shopType, items := range grouped {
		fmt.Printf("[DEBUG] Building field for %s shop with %d items\n", shopType, len(items))
		for _, item := range items {
			fmt.Printf("[DEBUG]   - %s: x%d\n", mg.FormatItemName(item.Item.ItemID()), item.NewStock)
		}
	}

	for shopType, items := range grouped {
		emoji := shopEmoji[shopType]
		var lines []string
		for _, item := range items {
			name := mg.FormatItemName(item.Item.ItemID())
			badge := mg.FormatExclusivityBadge(item.Item.ItemID())
			if badge != "" {
				lines = append(lines, fmt.Sprintf("**%s** — x%d %s", name, item.NewStock, badge))
			} else {
				lines = append(lines, fmt.Sprintf("**%s** — x%d", name, item.NewStock))
			}
		}
		field := &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("%s %s Shop", emoji, cases.Title(language.English).String(shopType)),
			Value:  strings.Join(lines, "\n"),
			Inline: false,
		}
		fields = append(fields, field)
		fmt.Printf("[DEBUG] Created %s field with %d items, value length=%d\n", shopType, len(items), len(field.Value))
	}

	return &discordgo.MessageEmbed{
		Title:       "🔔 Stock Alert!",
		Description: "Items you're watching are now in stock!",
		Color:       colorGreen,
		Fields:      fields,
		Footer: &discordgo.MessageEmbedFooter{
			Text: "magic-guardian • quality of life for Magic Garden",
		},
	}
}

// buildDMUnsubButtons creates action rows with per-item unsubscribe buttons
// (only for items in the alert) plus a red "Stop all notifications" button.
// Discord allows max 5 action rows with max 5 buttons each.
func buildDMUnsubButtons(changes []mg.StockChange) []discordgo.MessageComponent {
	// Deduplicate items (same item could appear in multiple changes)
	seen := make(map[string]bool)
	var buttons []discordgo.MessageComponent
	for _, ch := range changes {
		itemID := ch.Item.ItemID()
		if seen[itemID] {
			continue
		}
		seen[itemID] = true
		name := mg.FormatItemName(itemID)
		buttons = append(buttons, discordgo.Button{
			Label:    fmt.Sprintf("Unsubscribe from %s", name),
			Style:    discordgo.SecondaryButton,
			CustomID: fmt.Sprintf("dm_unsub_%s", itemID),
		})
	}

	// Discord limits: 5 action rows, 5 buttons per row.
	// Reserve the last row for the Stop All button if we have many items.
	maxItemButtons := 19 // 4 rows × 5 buttons = 20, minus 1 for stop-all in 5th row
	if len(buttons) > maxItemButtons {
		buttons = buttons[:maxItemButtons]
	}

	var rows []discordgo.MessageComponent
	for i := 0; i < len(buttons); i += 5 {
		end := i + 5
		if end > len(buttons) {
			end = len(buttons)
		}
		rows = append(rows, discordgo.ActionsRow{Components: buttons[i:end]})
	}

	// Add the Stop All button in its own row
	rows = append(rows, discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.Button{
				Label:    "Stop all notifications",
				Style:    discordgo.DangerButton,
				CustomID: "dm_unsub_all",
			},
		},
	})

	// Cap at 5 action rows total
	if len(rows) > 5 {
		rows = rows[:5]
	}

	return rows
}

// BuildStockEmbed creates a rich embed showing current shop inventory.
func BuildStockEmbed(shopType string, shop *mg.Shop) *discordgo.MessageEmbed {
	emoji := shopEmoji[shopType]
	var inStockLines, outOfStockLines []string

	for _, item := range shop.Inventory {
		name := mg.FormatItemName(item.ItemID())
		badge := mg.FormatExclusivityBadge(item.ItemID())
		if item.InitialStock > 0 {
			if badge != "" {
				inStockLines = append(inStockLines, fmt.Sprintf("✅ **%s** — x%d %s", name, item.InitialStock, badge))
			} else {
				inStockLines = append(inStockLines, fmt.Sprintf("✅ **%s** — x%d", name, item.InitialStock))
			}
		} else {
			if badge != "" {
				outOfStockLines = append(outOfStockLines, fmt.Sprintf("❌ %s %s", name, badge))
			} else {
				outOfStockLines = append(outOfStockLines, fmt.Sprintf("❌ %s", name))
			}
		}
	}

	var fields []*discordgo.MessageEmbedField
	if len(inStockLines) > 0 {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:  "In Stock",
			Value: strings.Join(inStockLines, "\n"),
		})
	}
	if len(outOfStockLines) > 0 {
		value := strings.Join(outOfStockLines, "\n")
		if len(value) > 1024 {
			value = value[:1020] + "..."
		}
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:  "Out of Stock",
			Value: value,
		})
	}

	restockMin := int(shop.SecondsUntilRestock) / 60
	restockSec := int(shop.SecondsUntilRestock) % 60

	return &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("%s %s Shop", emoji, cases.Title(language.English).String(shopType)),
		Description: fmt.Sprintf("Next restock in **%dm %ds**", restockMin, restockSec),
		Color:       colorBlue,
		Fields:      fields,
		Footer: &discordgo.MessageEmbedFooter{
			Text: "magic-guardian",
		},
	}
}

// BuildWatchlistEmbed creates an embed showing a user's subscriptions.
func BuildWatchlistEmbed(items []WatchlistItem) *discordgo.MessageEmbed {
	if len(items) == 0 {
		return &discordgo.MessageEmbed{
			Title:       "📋 Your Watchlist",
			Description: "You have no active subscriptions.\nUse `/subscribe <item>` to get started!",
			Color:       colorGold,
		}
	}

	var lines []string
	for _, item := range items {
		emoji := shopEmoji[item.ShopType]
		status := "❌ out of stock"
		if item.CurrentStock > 0 {
			status = fmt.Sprintf("✅ x%d in stock", item.CurrentStock)
		}
		badge := mg.FormatExclusivityBadge(item.ItemID)
		if badge != "" {
			lines = append(lines, fmt.Sprintf("%s **%s** — %s %s", emoji, mg.FormatItemName(item.ItemID), status, badge))
		} else {
			lines = append(lines, fmt.Sprintf("%s **%s** — %s", emoji, mg.FormatItemName(item.ItemID), status))
		}
	}

	return &discordgo.MessageEmbed{
		Title:       "📋 Your Watchlist",
		Description: strings.Join(lines, "\n"),
		Color:       colorPurple,
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("%d items watched", len(items)),
		},
	}
}

// BuildRestockEmbed creates an embed showing restock timers.
func BuildRestockEmbed(shops map[string]*mg.Shop) *discordgo.MessageEmbed {
	var lines []string
	for _, shopType := range []string{"seed", "tool", "egg", "decor"} {
		shop, ok := shops[shopType]
		if !ok {
			continue
		}
		emoji := shopEmoji[shopType]
		min := int(shop.SecondsUntilRestock) / 60
		sec := int(shop.SecondsUntilRestock) % 60
		lines = append(lines, fmt.Sprintf("%s **%s** — %dm %ds", emoji, cases.Title(language.English).String(shopType), min, sec))
	}

	return &discordgo.MessageEmbed{
		Title:       "⏱️ Restock Timers",
		Description: strings.Join(lines, "\n"),
		Color:       colorGold,
		Footer: &discordgo.MessageEmbedFooter{
			Text: "magic-guardian",
		},
	}
}

// WatchlistItem is a subscription enriched with current stock data.
type WatchlistItem struct {
	ItemID       string
	ShopType     string
	CurrentStock int
}
