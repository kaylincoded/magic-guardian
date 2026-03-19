package discord

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/kaylincoded/magic-guardian/internal/mg"
	"github.com/kaylincoded/magic-guardian/internal/store"
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
	board   *Board
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

	b.board = NewBoard(session, st, mgState, logger.With("component", "board"), appID)

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
		{
			Name:                     "setup-stock-board",
			Description:              "Create a live stock board channel with subscribe menus",
			DefaultMemberPermissions: &adminPerm,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "name",
					Description: "Category name (default: 📦 Magic Garden Stock)",
					Required:    false,
				},
			},
		},
		{
			Name:                     "delete-stock-board",
			Description:              "Delete the stock board channels and category",
			DefaultMemberPermissions: &adminPerm,
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

// GuildInfo holds basic information about a Discord guild the bot is in.
type GuildInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon"`
}

// Guilds returns a list of guilds the bot is currently in.
func (b *Bot) Guilds() []GuildInfo {
	var guilds []GuildInfo
	for _, g := range b.session.State.Guilds {
		guilds = append(guilds, GuildInfo{
			ID:   g.ID,
			Name: g.Name,
			Icon: g.IconURL("64"),
		})
	}
	return guilds
}

// LeaveGuild removes the bot from the specified guild.
func (b *Bot) LeaveGuild(guildID string) error {
	return b.session.GuildLeave(guildID)
}

// SendStockAlert sends a DM to a user with stock alert information
// and per-item unsubscribe buttons + a "stop all" button.
func (b *Bot) SendStockAlert(userID string, changes []mg.StockChange) error {
	// DEBUG: Log what we're about to send
	fmt.Printf("[DEBUG] SendStockAlert: user=%s, changes=%d\n", userID, len(changes))
	for _, ch := range changes {
		fmt.Printf("[DEBUG]   - %s %s: x%d\n", ch.ShopType, mg.FormatItemName(ch.Item.ItemID()), ch.NewStock)
	}

	ch, err := b.session.UserChannelCreate(userID)
	if err != nil {
		return fmt.Errorf("create DM channel: %w", err)
	}
	embed := BuildStockAlertEmbed(changes)

	// DEBUG: Log embed details
	fmt.Printf("[DEBUG] Embed has %d fields\n", len(embed.Fields))
	for i, field := range embed.Fields {
		fmt.Printf("[DEBUG]   Field %d: %s - %d chars\n", i, field.Name, len(field.Value))
	}

	components := buildDMUnsubButtons(changes)
	_, err = b.session.ChannelMessageSendComplex(ch.ID, &discordgo.MessageSend{
		Embeds:     []*discordgo.MessageEmbed{embed},
		Components: components,
	})
	if err != nil {
		fmt.Printf("[DEBUG] Discord send error: %v\n", err)
	} else {
		fmt.Printf("[DEBUG] Discord send successful\n")
	}
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

var adminPerm int64 = discordgo.PermissionManageChannels

// Board returns the board manager for external callers.
func (b *Bot) Board() *Board {
	return b.board
}

func (b *Bot) handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Defer a fallback acknowledgment in case the handler doesn't respond in time
	acknowledged := false
	defer func() {
		if !acknowledged {
			b.logger.Warn("interaction was not acknowledged by handler, sending fallback")
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Something went wrong. Please try again.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
		}
	}()

	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		b.handleCommand(s, i)
		acknowledged = true
	case discordgo.InteractionApplicationCommandAutocomplete:
		b.handleAutocomplete(s, i)
		acknowledged = true
	case discordgo.InteractionMessageComponent:
		customID := i.MessageComponentData().CustomID
		if strings.HasPrefix(customID, "dm_unsub_") {
			b.handleDMUnsub(s, i)
		} else if strings.HasPrefix(customID, "board_update_") {
			b.board.HandleUpdateButton(s, i)
		} else if strings.HasPrefix(customID, "update_sub_") {
			b.board.HandleUpdateSubscriptions(s, i)
		} else {
			b.board.HandleSelectMenu(s, i)
		}
		acknowledged = true
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
	case "setup-stock-board":
		b.cmdSetupStockBoard(s, i)
	case "delete-stock-board":
		b.cmdDeleteStockBoard(s, i)
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

	// Add exclusivity warning if applicable
	exclusivityNote := mg.FormatExclusivityDetail(itemID, guildID)
	if exclusivityNote != "" {
		b.respond(s, i, fmt.Sprintf("✅ Subscribed to **%s**! You'll be DM'd when it's in stock.\n\n%s", name, exclusivityNote))
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

func (b *Bot) cmdSetupStockBoard(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.GuildID == "" {
		b.respond(s, i, "❌ This command can only be used in a server.")
		return
	}

	// Defer the response since channel creation takes a moment
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	categoryName := "📦 Magic Garden Stock"
	if opts := i.ApplicationCommandData().Options; len(opts) > 0 {
		categoryName = opts[0].StringValue()
	}

	channelID, err := b.board.SetupBoard(i.GuildID, categoryName)
	if err != nil {
		b.logger.Error("setup-stock-board failed", "error", err)
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: strPtr("❌ Failed to create stock board. Make sure I have Manage Channels permission."),
		})
		return
	}

	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: strPtr(fmt.Sprintf("✅ Stock board created! Check <#%s>", channelID)),
	})
}

func (b *Bot) cmdDeleteStockBoard(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.GuildID == "" {
		b.respond(s, i, "❌ This command can only be used in a server.")
		return
	}

	// Defer the response since channel deletion takes a moment
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	deleted, err := b.board.DeleteBoard(i.GuildID)
	if err != nil {
		b.logger.Error("delete-stock-board failed", "error", err)
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: strPtr("❌ Failed to delete stock board. Make sure I have Manage Channels permission."),
		})
		return
	}

	if deleted == 0 {
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: strPtr("ℹ️ No stock board found in this server."),
		})
		return
	}

	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: strPtr(fmt.Sprintf("✅ Stock board deleted! Removed %d channel(s).", deleted)),
	})
}

func strPtr(s string) *string {
	return &s
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
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{Choices: nil},
		})
		return
	}

	query := strings.ToLower(focused.StringValue())
	var choices []*discordgo.ApplicationCommandOptionChoice
	commandName := data.Name
	userID := interactionUserID(i)

	// Get user's current subscriptions
	userSubs, _ := b.store.GetUserSubscriptions(userID)
	subscribedItems := make(map[string]bool)
	for _, sub := range userSubs {
		subscribedItems[sub.ItemID] = true
	}

	// For unsubscribe: only show items user is subscribed to
	if commandName == "unsubscribe" {
		for _, sub := range userSubs {
			item := mg.GetItemByID(sub.ItemID)
			if item == nil {
				continue
			}
			if query != "" && !strings.Contains(strings.ToLower(item.Name), query) && !strings.Contains(strings.ToLower(item.ID), query) {
				continue
			}
			emoji := shopEmoji[item.ShopType]
			label := fmt.Sprintf("%s %s", emoji, item.Name)
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
				Name:  label,
				Value: strings.ToLower(item.ID),
			})
			if len(choices) >= 25 {
				break
			}
		}

		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{
				Choices: choices,
			},
		}); err != nil {
			b.logger.Error("autocomplete respond failed", "error", err)
		}
		return
	}

	// For subscribe: show all items, mark already-subscribed ones
	buildChoice := func(item mg.Item) *discordgo.ApplicationCommandOptionChoice {
		emoji := shopEmoji[item.ShopType]
		exclusivity := mg.FormatExclusivityBadgeShort(item.ID)
		label := fmt.Sprintf("%s %s", emoji, item.Name)
		if exclusivity != "" {
			label = fmt.Sprintf("%s %s %s", emoji, item.Name, exclusivity)
		}
		if subscribedItems[strings.ToLower(item.ID)] {
			label += " (subscribed)"
		}
		return &discordgo.ApplicationCommandOptionChoice{
			Name:  label,
			Value: strings.ToLower(item.ID),
		}
	}

	allItems := mg.GetAllItems()

	if query == "" {
		// No query: show a balanced mix from each shop type
		perShop := 6 // ~6 per shop type = 24 total, leaving room
		shopCounts := map[string]int{"seed": 0, "tool": 0, "egg": 0, "decor": 0}

		for _, item := range allItems {
			if len(choices) >= 25 {
				break
			}
			if shopCounts[item.ShopType] >= perShop {
				continue
			}
			choices = append(choices, buildChoice(item))
			shopCounts[item.ShopType]++
		}
	} else {
		// With query: filter and show matching items
		for _, item := range allItems {
			if !strings.Contains(strings.ToLower(item.Name), query) && !strings.Contains(strings.ToLower(item.ID), query) {
				continue
			}
			choices = append(choices, buildChoice(item))
			if len(choices) >= 25 {
				break
			}
		}
	}

	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: choices,
		},
	}); err != nil {
		b.logger.Error("autocomplete respond failed", "error", err)
	}
}

func (b *Bot) handleDMUnsub(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID := interactionUserID(i)
	customID := i.MessageComponentData().CustomID

	if customID == "dm_unsub_all" {
		count, err := b.store.UnsubscribeAll(userID)
		if err != nil {
			b.logger.Error("dm unsub all failed", "error", err)
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "❌ Something went wrong.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("🗑️ Removed all **%d** subscriptions. You won't receive any more alerts.", count),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	// dm_unsub_<itemID>
	itemID := strings.TrimPrefix(customID, "dm_unsub_")
	removed, err := b.store.Unsubscribe(userID, itemID)
	if err != nil {
		b.logger.Error("dm unsub failed", "error", err)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "❌ Something went wrong.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	name := mg.FormatItemName(itemID)
	msg := fmt.Sprintf("🗑️ Unsubscribed from **%s**.", name)
	if !removed {
		msg = fmt.Sprintf("ℹ️ You weren't subscribed to **%s**.", name)
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: msg,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}
