package discord

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/kaylin/magic-guardian/internal/mg"
	"github.com/kaylin/magic-guardian/internal/store"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Bot manages the Discord bot session and slash commands.
type Bot struct {
	session *discordgo.Session
	store   *store.Store
	mgState *mg.ShopState
	logger  *slog.Logger
	appID   string
}

// NewBot creates and configures a new Discord bot.
func NewBot(token, appID string, st *store.Store, mgState *mg.ShopState, logger *slog.Logger) (*Bot, error) {
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	b := &Bot{
		session: session,
		store:   st,
		mgState: mgState,
		logger:  logger,
		appID:   appID,
	}

	session.AddHandler(b.handleInteraction)

	return b, nil
}

// Start opens the Discord session and registers slash commands.
func (b *Bot) Start() error {
	if err := b.session.Open(); err != nil {
		return fmt.Errorf("open session: %w", err)
	}
	b.logger.Info("discord bot connected", "user", b.session.State.User.Username)

	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "subscribe",
			Description: "Get notified when an item is in stock",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:         discordgo.ApplicationCommandOptionString,
					Name:         "item",
					Description:  "Item name (e.g. Bamboo, MythicalEgg, Shovel)",
					Required:     true,
					Autocomplete: true,
				},
			},
		},
		{
			Name:        "unsubscribe",
			Description: "Stop notifications for an item",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:         discordgo.ApplicationCommandOptionString,
					Name:         "item",
					Description:  "Item name to unsubscribe from",
					Required:     true,
					Autocomplete: true,
				},
			},
		},
		{
			Name:        "watchlist",
			Description: "Show your current subscriptions",
		},
		{
			Name:        "stock",
			Description: "Show current shop inventory",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "shop",
					Description: "Shop type (seed, tool, egg, decor). Leave empty for all.",
					Required:    false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "🌱 Seed", Value: "seed"},
						{Name: "🔧 Tool", Value: "tool"},
						{Name: "🥚 Egg", Value: "egg"},
						{Name: "🎨 Decor", Value: "decor"},
					},
				},
			},
		},
		{
			Name:        "restock",
			Description: "Show time until next restock for each shop",
		},
	}

	_, err := b.session.ApplicationCommandBulkOverwrite(b.appID, "", commands)
	if err != nil {
		return fmt.Errorf("register commands: %w", err)
	}
	b.logger.Info("slash commands registered", "count", len(commands))
	return nil
}

// Stop closes the Discord session.
func (b *Bot) Stop() error {
	return b.session.Close()
}

// Session returns the underlying discordgo session.
func (b *Bot) Session() *discordgo.Session {
	return b.session
}

// SendStockAlert sends a DM to a user with stock alert information.
func (b *Bot) SendStockAlert(userID string, changes []mg.StockChange) error {
	ch, err := b.session.UserChannelCreate(userID)
	if err != nil {
		return fmt.Errorf("create DM channel: %w", err)
	}
	embed := BuildStockAlertEmbed(changes)
	_, err = b.session.ChannelMessageSendEmbed(ch.ID, embed)
	return err
}

func interactionUserID(i *discordgo.InteractionCreate) string {
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User.ID
	}
	if i.User != nil {
		return i.User.ID
	}
	return ""
}

func (b *Bot) handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		b.handleCommand(s, i)
	case discordgo.InteractionApplicationCommandAutocomplete:
		b.handleAutocomplete(s, i)
	}
}

func (b *Bot) handleCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	switch data.Name {
	case "subscribe":
		b.cmdSubscribe(s, i)
	case "unsubscribe":
		b.cmdUnsubscribe(s, i)
	case "watchlist":
		b.cmdWatchlist(s, i)
	case "stock":
		b.cmdStock(s, i)
	case "restock":
		b.cmdRestock(s, i)
	}
}

func (b *Bot) cmdSubscribe(s *discordgo.Session, i *discordgo.InteractionCreate) {
	itemRaw := i.ApplicationCommandData().Options[0].StringValue()
	itemID, shopType := b.resolveItem(itemRaw)

	if itemID == "" {
		b.respond(s, i, fmt.Sprintf("❌ Unknown item: **%s**. Try using autocomplete!", itemRaw))
		return
	}

	guildID := ""
	if i.GuildID != "" {
		guildID = i.GuildID
	}

	created, err := b.store.Subscribe(interactionUserID(i), guildID, itemID, shopType)
	if err != nil {
		b.logger.Error("subscribe failed", "error", err)
		b.respond(s, i, "❌ Something went wrong. Please try again.")
		return
	}

	name := mg.FormatItemName(itemRaw)
	if !created {
		b.respond(s, i, fmt.Sprintf("ℹ️ You're already subscribed to **%s**.", name))
		return
	}
	b.respond(s, i, fmt.Sprintf("✅ Subscribed to **%s**! You'll be DM'd when it's in stock.", name))
}

func (b *Bot) cmdUnsubscribe(s *discordgo.Session, i *discordgo.InteractionCreate) {
	itemRaw := i.ApplicationCommandData().Options[0].StringValue()

	userID := interactionUserID(i)
	removed, err := b.store.Unsubscribe(userID, itemRaw)
	if err != nil {
		b.logger.Error("unsubscribe failed", "error", err)
		b.respond(s, i, "❌ Something went wrong. Please try again.")
		return
	}

	name := mg.FormatItemName(itemRaw)
	if !removed {
		b.respond(s, i, fmt.Sprintf("ℹ️ You weren't subscribed to **%s**.", name))
		return
	}
	b.respond(s, i, fmt.Sprintf("🗑️ Unsubscribed from **%s**.", name))
}

func (b *Bot) cmdWatchlist(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID := interactionUserID(i)
	subs, err := b.store.GetUserSubscriptions(userID)
	if err != nil {
		b.logger.Error("watchlist failed", "error", err)
		b.respond(s, i, "❌ Something went wrong. Please try again.")
		return
	}

	var items []WatchlistItem
	shops := b.mgState.GetAllShops()
	for _, sub := range subs {
		stock := 0
		if shop, ok := shops[sub.ShopType]; ok {
			for _, item := range shop.Inventory {
				if strings.ToLower(item.ItemID()) == sub.ItemID {
					stock = item.InitialStock
					break
				}
			}
		}
		items = append(items, WatchlistItem{
			ItemID:       sub.ItemID,
			ShopType:     sub.ShopType,
			CurrentStock: stock,
		})
	}

	embed := BuildWatchlistEmbed(items)
	b.respondEmbed(s, i, embed)
}

func (b *Bot) cmdStock(s *discordgo.Session, i *discordgo.InteractionCreate) {
	shopFilter := ""
	if len(i.ApplicationCommandData().Options) > 0 {
		shopFilter = i.ApplicationCommandData().Options[0].StringValue()
	}

	if shopFilter != "" {
		shop, ok := b.mgState.GetShop(shopFilter)
		if !ok {
			b.respond(s, i, fmt.Sprintf("❌ Unknown shop: **%s**", shopFilter))
			return
		}
		embed := BuildStockEmbed(shopFilter, shop)
		b.respondEmbed(s, i, embed)
		return
	}

	// Show all shops — send restock timers as the main embed
	shops := b.mgState.GetAllShops()
	embed := BuildRestockEmbed(shops)
	embed.Title = "📊 Shop Overview"
	embed.Description = "Use `/stock seed`, `/stock tool`, `/stock egg`, or `/stock decor` for details."

	var fields []*discordgo.MessageEmbedField
	for _, shopType := range []string{"seed", "tool", "egg", "decor"} {
		shop, ok := shops[shopType]
		if !ok {
			continue
		}
		inStock := 0
		for _, item := range shop.Inventory {
			if item.InitialStock > 0 {
				inStock++
			}
		}
		emoji := shopEmoji[shopType]
		min := int(shop.SecondsUntilRestock) / 60
		sec := int(shop.SecondsUntilRestock) % 60
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("%s %s", emoji, cases.Title(language.English).String(shopType)),
			Value:  fmt.Sprintf("%d/%d in stock\nRestock in %dm %ds", inStock, len(shop.Inventory), min, sec),
			Inline: true,
		})
	}
	embed.Fields = fields
	b.respondEmbed(s, i, embed)
}

func (b *Bot) cmdRestock(s *discordgo.Session, i *discordgo.InteractionCreate) {
	shops := b.mgState.GetAllShops()
	embed := BuildRestockEmbed(shops)
	b.respondEmbed(s, i, embed)
}

func (b *Bot) respond(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func (b *Bot) respondEmbed(s *discordgo.Session, i *discordgo.InteractionCreate, embed *discordgo.MessageEmbed) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	})
}

// resolveItem finds the canonical item ID and shop type from user input.
func (b *Bot) resolveItem(input string) (itemID string, shopType string) {
	inputLower := strings.ToLower(input)
	shops := b.mgState.GetAllShops()
	for st, shop := range shops {
		for _, item := range shop.Inventory {
			if strings.ToLower(item.ItemID()) == inputLower {
				return strings.ToLower(item.ItemID()), st
			}
		}
	}
	return "", ""
}

func (b *Bot) handleAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	var focused *discordgo.ApplicationCommandInteractionDataOption
	for _, opt := range data.Options {
		if opt.Focused {
			focused = opt
			break
		}
	}
	if focused == nil {
		return
	}

	query := strings.ToLower(focused.StringValue())
	var choices []*discordgo.ApplicationCommandOptionChoice

	shops := b.mgState.GetAllShops()
	for _, shopType := range []string{"seed", "tool", "egg", "decor"} {
		shop, ok := shops[shopType]
		if !ok {
			continue
		}
		emoji := shopEmoji[shopType]
		for _, item := range shop.Inventory {
			id := item.ItemID()
			name := mg.FormatItemName(id)
			if query != "" && !strings.Contains(strings.ToLower(name), query) && !strings.Contains(strings.ToLower(id), query) {
				continue
			}
			stock := "❌"
			if item.InitialStock > 0 {
				stock = fmt.Sprintf("✅ x%d", item.InitialStock)
			}
			label := fmt.Sprintf("%s %s [%s]", emoji, name, stock)
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
				Name:  label,
				Value: strings.ToLower(id),
			})
			if len(choices) >= 25 {
				break
			}
		}
		if len(choices) >= 25 {
			break
		}
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: choices,
		},
	})
}
