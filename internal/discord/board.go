package discord

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/kaylin/magic-guardian/internal/mg"
	"github.com/kaylin/magic-guardian/internal/store"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Board manages the live stock board in Discord channels.
type Board struct {
	session     *discordgo.Session
	store       *store.Store
	mgState     *mg.ShopState
	logger      *slog.Logger
	appID       string
	boardIDs    sync.Map // guild:channel:message
	lastUpdates sync.Map // guild:channel -> last update time
}

// NewBoard creates a new Board manager.
func NewBoard(session *discordgo.Session, st *store.Store, mgState *mg.ShopState, logger *slog.Logger, appID string) *Board {
	bd := &Board{
		session: session,
		store:   st,
		mgState: mgState,
		logger:  logger,
		appID:   appID,
	}

	// Start ticker to update timestamps every minute
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			bd.updateTimestamps()
		}
	}()

	return bd
}

var shopOrder = []string{"seed", "tool", "egg", "decor"}

var shopChannelNames = map[string]string{
	"seed":  "🌱・seed-shop",
	"tool":  "🔧・tool-shop",
	"egg":   "🥚・egg-shop",
	"decor": "🎨・decor-shop",
}

// SetupBoard creates (or recreates) the stock board in a guild.
// It creates a "Stock Board" category with one read-only channel per shop,
// each containing a live embed with select menus for subscribing.
func (bd *Board) SetupBoard(guildID, categoryName string) (string, error) {
	// Delete old board config if exists
	bd.store.DeleteBoardConfig(guildID)

	botUser, _ := bd.session.User("@me")

	// Shared permission overwrites for all channels
	perms := []*discordgo.PermissionOverwrite{
		{
			ID:    guildID,
			Type:  discordgo.PermissionOverwriteTypeRole,
			Deny:  discordgo.PermissionSendMessages | discordgo.PermissionCreatePublicThreads | discordgo.PermissionCreatePrivateThreads,
			Allow: discordgo.PermissionViewChannel | discordgo.PermissionReadMessageHistory,
		},
		{
			ID:    botUser.ID,
			Type:  discordgo.PermissionOverwriteTypeMember,
			Allow: discordgo.PermissionSendMessages | discordgo.PermissionManageMessages | discordgo.PermissionViewChannel | discordgo.PermissionEmbedLinks,
		},
	}

	// Create category
	category, err := bd.session.GuildChannelCreateComplex(guildID, discordgo.GuildChannelCreateData{
		Name:                 categoryName,
		Type:                 discordgo.ChannelTypeGuildCategory,
		PermissionOverwrites: perms,
	})
	if err != nil {
		return "", fmt.Errorf("create category: %w", err)
	}

	// Create one channel per shop under the category
	shops := bd.mgState.GetAllShops()
	var firstChannelID string
	for _, shopType := range shopOrder {
		shop, ok := shops[shopType]
		if !ok {
			continue
		}

		channelName := shopChannelNames[shopType]
		ch, err := bd.session.GuildChannelCreateComplex(guildID, discordgo.GuildChannelCreateData{
			Name:                 channelName,
			Type:                 discordgo.ChannelTypeGuildText,
			Topic:                fmt.Sprintf("Live %s shop inventory — use the menu below to subscribe to restock alerts!", shopType),
			ParentID:             category.ID,
			PermissionOverwrites: perms,
		})
		if err != nil {
			bd.logger.Error("failed to create shop channel", "shop", shopType, "error", err)
			continue
		}

		if firstChannelID == "" {
			firstChannelID = ch.ID
		}

		embed := bd.buildEmbed(shopType, shop, time.Now())
		components := bd.buildUpdateButton(shopType)

		msg, err := bd.session.ChannelMessageSendComplex(ch.ID, &discordgo.MessageSend{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: components,
		})
		if err != nil {
			bd.logger.Error("failed to post board embed", "shop", shopType, "error", err)
			continue
		}

		// Store the message ID and update time for updates
		boardKey := fmt.Sprintf("%s:%s:%s", guildID, ch.ID, shopType)
		bd.boardIDs.Store(boardKey, msg.ID)
		bd.lastUpdates.Store(boardKey, time.Now())

		if err := bd.store.SetBoardMessage(guildID, ch.ID, shopType, msg.ID); err != nil {
			bd.logger.Error("failed to persist board message", "shop", shopType, "error", err)
		}
	}

	bd.logger.Info("stock board created", "guild", guildID, "category", category.ID)
	return firstChannelID, nil
}

// UpdateAllBoards edits all board embeds across all guilds with current shop state.
func (bd *Board) UpdateAllBoards() {
	configs, err := bd.store.GetAllBoardConfigs()
	if err != nil {
		bd.logger.Error("failed to load board configs", "error", err)
		return
	}

	shops := bd.mgState.GetAllShops()
	for _, cfg := range configs {
		for _, shopType := range shopOrder {
			msgID, ok := cfg.Messages[shopType]
			if !ok {
				continue
			}
			channelID, ok := cfg.Channels[shopType]
			if !ok {
				continue
			}
			shop, ok := shops[shopType]
			if !ok {
				continue
			}

			updateTime := time.Now()
			embed := bd.buildEmbed(shopType, shop, updateTime)

			// We don't need to fetch the message anymore since we rebuild the button

			// Update embed but keep the same update button
			components := bd.buildUpdateButton(shopType)
			_, err = bd.session.ChannelMessageEditComplex(&discordgo.MessageEdit{
				Channel:    channelID,
				ID:         msgID,
				Embed:      embed,
				Components: &components,
			})

			// Store the update time
			boardKey := fmt.Sprintf("%s:%s:%s", cfg.GuildID, channelID, shopType)
			bd.lastUpdates.Store(boardKey, updateTime)
			if err != nil {
				bd.logger.Debug("failed to edit board embed", "shop", shopType, "guild", cfg.GuildID, "error", err)
			}
		}
	}
}

// HandleSelectMenu processes a select menu interaction from the stock board.
func (bd *Board) HandleSelectMenu(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.MessageComponentData()
	customID := data.CustomID

	// Parse custom ID: "board_sub_seed", "board_sub_seed_1", etc.
	if !strings.HasPrefix(customID, "board_sub_") {
		return
	}

	userID := interactionUserID(i)
	if userID == "" {
		return
	}

	guildID := i.GuildID

	if strings.HasPrefix(customID, "board_sub_") {
		// Extract shop type: "board_sub_seed" → "seed", "board_sub_seed_1" → "seed"
		rest := strings.TrimPrefix(customID, "board_sub_")
		shopType := rest
		for _, st := range shopOrder {
			if strings.HasPrefix(rest, st) {
				shopType = st
				break
			}
		}
		selected := data.Values
		if len(selected) == 0 {
			bd.respondEphemeral(s, i, "No items selected.")
			return
		}

		var subscribed []string
		for _, itemID := range selected {
			created, err := bd.store.Subscribe(userID, guildID, itemID, shopType)
			if err != nil {
				bd.logger.Error("board subscribe failed", "user", userID, "item", itemID, "error", err)
				continue
			}
			if created {
				subscribed = append(subscribed, mg.FormatItemName(itemID))
			}
		}

		if len(subscribed) == 0 {
			bd.respondEphemeral(s, i, "ℹ️ You're already subscribed to all selected items.")
		} else {
			bd.respondEphemeral(s, i, fmt.Sprintf("✅ Subscribed to: **%s**\nYou'll be DM'd when they restock!", strings.Join(subscribed, ", ")))
		}
	}
}

// HandleUpdateButton processes the "Update Subscriptions" button click.
func (bd *Board) HandleUpdateButton(s *discordgo.Session, i *discordgo.InteractionCreate) {
	customID := i.MessageComponentData().CustomID

	// Extract shop type: "board_update_seed" → "seed"
	if !strings.HasPrefix(customID, "board_update_") {
		bd.respondEphemeral(s, i, "Invalid button.")
		return
	}

	shopType := strings.TrimPrefix(customID, "board_update_")
	userID := interactionUserID(i)

	// Get current subscriptions for this user
	subs, err := bd.store.GetUserSubscriptions(userID)
	if err != nil {
		bd.logger.Error("failed to get user subscriptions", "error", err)
		bd.respondEphemeral(s, i, "Failed to load your subscriptions.")
		return
	}

	// Build a map of subscribed item IDs for quick lookup (lowercase to match dropdown values)
	subscribedMap := make(map[string]bool)
	for _, sub := range subs {
		if sub.ShopType == shopType {
			subscribedMap[strings.ToLower(sub.ItemID)] = true
		}
	}

	// Get shop data to build the dropdown
	shops := bd.mgState.GetAllShops()
	shop, ok := shops[shopType]
	if !ok {
		bd.respondEphemeral(s, i, "Shop not found.")
		return
	}

	// Build dropdown options with subscription status, paginated to 25 items
	var allOptions []discordgo.SelectMenuOption
	for _, item := range shop.Inventory {
		id := item.ItemID()
		name := mg.FormatItemName(id)
		value := strings.ToLower(id)

		// Show subscription status in the description
		desc := "❌ Not subscribed"
		if subscribedMap[value] {
			desc = "✅ Subscribed"
		}

		// Add stock info
		if item.InitialStock > 0 {
			desc += fmt.Sprintf(" • x%d in stock", item.InitialStock)
		} else {
			desc += " • Out of stock"
		}

		allOptions = append(allOptions, discordgo.SelectMenuOption{
			Label:       name,
			Value:       value,
			Description: desc,
			Emoji: &discordgo.ComponentEmoji{
				Name: shopEmoji[shopType],
			},
		})
	}

	// Split into pages of 25 (Discord limit)
	const pageSize = 25
	var components []discordgo.MessageComponent
	for page := 0; page*pageSize < len(allOptions) && page < 5; page++ {
		start := page * pageSize
		end := start + pageSize
		if end > len(allOptions) {
			end = len(allOptions)
		}
		opts := allOptions[start:end]

		placeholder := fmt.Sprintf("🔔 Manage %s subscriptions...", shopType)
		customID := fmt.Sprintf("update_sub_%s", shopType)
		if page > 0 {
			placeholder = fmt.Sprintf("🔔 Manage %s subscriptions (more)...", shopType)
			customID = fmt.Sprintf("update_sub_%s_%d", shopType, page)
		}

		components = append(components, discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    customID,
					Placeholder: placeholder,
					Options:     opts,
					MaxValues:   len(opts), // Allow selecting multiple items
				},
			},
		})
	}

	// Send ephemeral response with the dropdown
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    fmt.Sprintf("**%s %s Shop** — Select items to toggle subscriptions:", shopEmoji[shopType], cases.Title(language.English).String(shopType)),
			Components: components,
			Flags:      discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		bd.logger.Error("failed to respond to update button", "error", err)
		// Fallback
		bd.respondEphemeral(s, i, "Failed to open subscription manager.")
		return
	}
}

func (bd *Board) HandleUpdateSubscriptions(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.MessageComponentData()
	customID := data.CustomID

	// Extract shop type: "update_sub_seed" → "seed", "update_sub_seed_1" → "seed"
	if !strings.HasPrefix(customID, "update_sub_") {
		bd.respondEphemeral(s, i, "Invalid dropdown.")
		return
	}

	rest := strings.TrimPrefix(customID, "update_sub_")
	shopType := rest
	for _, st := range shopOrder {
		if strings.HasPrefix(rest, st) {
			shopType = st
			break
		}
	}
	userID := interactionUserID(i)
	guildID := i.GuildID

	selected := data.Values
	if len(selected) == 0 {
		bd.respondEphemeral(s, i, "No items selected.")
		return
	}

	// Get current subscriptions to determine what to toggle
	subs, err := bd.store.GetUserSubscriptions(userID)
	if err != nil {
		bd.logger.Error("failed to get user subscriptions", "error", err)
		bd.respondEphemeral(s, i, "Failed to load your subscriptions.")
		return
	}

	// Build map of current subscriptions (lowercase to match dropdown values)
	subscribedMap := make(map[string]bool)
	for _, sub := range subs {
		if sub.ShopType == shopType {
			subscribedMap[strings.ToLower(sub.ItemID)] = true
		}
	}

	// Toggle subscriptions for selected items
	var subscribed, unsubscribed []string
	for _, itemID := range selected {
		// Convert back to uppercase for storage
		upperItemID := strings.ToUpper(itemID)

		if subscribedMap[itemID] {
			// User is already subscribed - unsubscribe them
			if _, err := bd.store.Unsubscribe(userID, upperItemID); err != nil {
				bd.logger.Error("failed to unsubscribe", "user", userID, "item", upperItemID, "error", err)
				continue
			}
			unsubscribed = append(unsubscribed, mg.FormatItemName(upperItemID))
		} else {
			// User is not subscribed - subscribe them
			created, err := bd.store.Subscribe(userID, guildID, upperItemID, shopType)
			if err != nil {
				bd.logger.Error("failed to subscribe", "user", userID, "item", upperItemID, "error", err)
				continue
			}
			if created {
				subscribed = append(subscribed, mg.FormatItemName(upperItemID))
			}
		}
	}

	// Build response message
	var parts []string
	if len(subscribed) > 0 {
		parts = append(parts, fmt.Sprintf("✅ Subscribed to: **%s**", strings.Join(subscribed, ", ")))
	}
	if len(unsubscribed) > 0 {
		parts = append(parts, fmt.Sprintf("❌ Unsubscribed from: **%s**", strings.Join(unsubscribed, ", ")))
	}

	if len(parts) == 0 {
		parts = append(parts, "ℹ️ No changes made.")
	} else if len(subscribed) > 0 {
		parts = append(parts, "You'll be DM'd when subscribed items restock!")
	}

	response := strings.Join(parts, "\n")

	// Update the original dropdown message with the response
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    response,
			Components: []discordgo.MessageComponent{}, // Remove the dropdown
			Flags:      discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		bd.logger.Error("failed to update subscription response", "error", err)
	}
}

func (bd *Board) updateTimestamps() {
	shops := bd.mgState.GetAllShops()

	bd.boardIDs.Range(func(key, value interface{}) bool {
		keyStr := key.(string)
		parts := strings.Split(keyStr, ":")
		if len(parts) != 3 {
			return true
		}

		channelID := parts[1]
		shopType := parts[2]

		shop, ok := shops[shopType]
		if !ok {
			return true
		}

		// Get the stored update time
		updateTimeInterface, ok := bd.lastUpdates.Load(keyStr)
		if !ok {
			return true
		}
		updateTime := updateTimeInterface.(time.Time)

		// Rebuild embed with updated timestamp
		embed := bd.buildEmbed(shopType, shop, updateTime)
		components := bd.buildUpdateButton(shopType)

		_, err := bd.session.ChannelMessageEditComplex(&discordgo.MessageEdit{
			Channel:    channelID,
			Embed:      embed,
			Components: &components,
		})
		if err != nil {
			bd.logger.Debug("failed to update timestamp", "channel", channelID, "shop", shopType, "error", err)
		}

		return true
	})
}

func (bd *Board) respondEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		bd.logger.Error("failed to send ephemeral response", "error", err)
		// Fallback: try a follow-up message
		_, followErr := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		if followErr != nil {
			bd.logger.Error("failed to send follow-up", "error", followErr)
		}
	}
}

// formatRelativeTime returns a human-readable relative time string.
func formatRelativeTime(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		minutes := int(d.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	}
	hours := int(d.Hours())
	if hours == 1 {
		return "1 hour ago"
	}
	return fmt.Sprintf("%d hours ago", hours)
}

// buildEmbed creates a rich embed for a shop inventory.
func (bd *Board) buildEmbed(shopType string, shop *mg.Shop, updateTime time.Time) *discordgo.MessageEmbed {
	emoji := shopEmoji[shopType]
	title := cases.Title(language.English).String(shopType)

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
		value := strings.Join(inStockLines, "\n")
		if len(value) > 1024 {
			value = value[:1020] + "..."
		}
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:  "In Stock",
			Value: value,
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

	inStock := len(inStockLines)
	total := len(inStockLines) + len(outOfStockLines)

	// Build restock description based on whether we've observed a full cycle
	var restockDesc string
	if shop.RestockCycle > 0 {
		cycleMin := (int(shop.RestockCycle) + 30) / 60
		if cycleMin < 1 {
			cycleMin = 1
		}
		restockDesc = fmt.Sprintf("Stock updates every ~%d minutes", cycleMin)
	} else {
		countdownMin := (int(shop.SecondsUntilRestock) + 30) / 60
		if countdownMin < 1 {
			countdownMin = 1
		}
		restockDesc = fmt.Sprintf("Stock updates in ~%d minutes", countdownMin)
	}

	color := colorBlue
	if inStock == total {
		color = colorGreen
	} else if inStock == 0 {
		color = colorRed
	}

	return &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("%s %s Shop", emoji, title),
		Description: fmt.Sprintf("**%d/%d** in stock • %s", inStock, total, restockDesc),
		Color:       color,
		Fields:      fields,
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("magic-guardian • updated %s", formatRelativeTime(updateTime)),
		},
	}
}

func (bd *Board) buildSelectMenu(shopType string, shop *mg.Shop) []discordgo.MessageComponent {
	// Build all options
	var allOptions []discordgo.SelectMenuOption
	for _, item := range shop.Inventory {
		id := item.ItemID()
		name := mg.FormatItemName(id)
		stock := "❌ Out of stock"
		if item.InitialStock > 0 {
			stock = fmt.Sprintf("✅ x%d in stock", item.InitialStock)
		}
		allOptions = append(allOptions, discordgo.SelectMenuOption{
			Label:       name,
			Value:       strings.ToLower(id),
			Description: stock,
			Emoji: &discordgo.ComponentEmoji{
				Name: shopEmoji[shopType],
			},
		})
	}

	// Split into pages of 25 (Discord limit)
	// Max 5 action rows per message, so max 5 pages
	const pageSize = 25
	var components []discordgo.MessageComponent
	for page := 0; page*pageSize < len(allOptions) && page < 5; page++ {
		start := page * pageSize
		end := start + pageSize
		if end > len(allOptions) {
			end = len(allOptions)
		}
		opts := allOptions[start:end]

		placeholder := fmt.Sprintf("🔔 Subscribe to %s alerts...", shopType)
		customID := fmt.Sprintf("board_sub_%s", shopType)
		if page > 0 {
			placeholder = fmt.Sprintf("🔔 Subscribe to %s alerts (more)...", shopType)
			customID = fmt.Sprintf("board_sub_%s_%d", shopType, page)
		}

		components = append(components, discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    customID,
					Placeholder: placeholder,
					MinValues:   intPtr(1),
					MaxValues:   len(opts),
					Options:     opts,
				},
			},
		})
	}

	return components
}

func intPtr(v int) *int {
	return &v
}

// buildUpdateButton creates a single "Update Subscriptions" button for the board.
func (bd *Board) buildUpdateButton(shopType string) []discordgo.MessageComponent {
	customID := fmt.Sprintf("board_update_%s", shopType)
	return []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "🔔 Update Subscriptions",
					Style:    discordgo.PrimaryButton,
					CustomID: customID,
				},
			},
		},
	}
}
