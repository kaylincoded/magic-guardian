package discord

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/kaylin/magic-guardian/internal/mg"
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

	for shopType, items := range grouped {
		emoji := shopEmoji[shopType]
		var lines []string
		for _, item := range items {
			name := mg.FormatItemName(item.Item.ItemID())
			lines = append(lines, fmt.Sprintf("**%s** — x%d", name, item.NewStock))
		}
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("%s %s Shop", emoji, cases.Title(language.English).String(shopType)),
			Value:  strings.Join(lines, "\n"),
			Inline: false,
		})
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

// BuildStockEmbed creates a rich embed showing current shop inventory.
func BuildStockEmbed(shopType string, shop *mg.Shop) *discordgo.MessageEmbed {
	emoji := shopEmoji[shopType]
	var inStockLines, outOfStockLines []string

	for _, item := range shop.Inventory {
		name := mg.FormatItemName(item.ItemID())
		if item.InitialStock > 0 {
			inStockLines = append(inStockLines, fmt.Sprintf("✅ **%s** — x%d", name, item.InitialStock))
		} else {
			outOfStockLines = append(outOfStockLines, fmt.Sprintf("❌ %s", name))
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
		lines = append(lines, fmt.Sprintf("%s **%s** — %s", emoji, mg.FormatItemName(item.ItemID), status))
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
