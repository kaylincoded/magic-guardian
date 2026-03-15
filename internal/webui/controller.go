package webui

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/kaylincoded/magic-guardian/internal/discord"
	"github.com/kaylincoded/magic-guardian/internal/mg"
	"github.com/kaylincoded/magic-guardian/internal/notify"
	"github.com/kaylincoded/magic-guardian/internal/store"
)

// DefaultController manages the bot engine lifecycle.
type DefaultController struct {
	mu        sync.Mutex
	db        *store.Store
	logger    *slog.Logger
	running   bool
	status    string
	startTime time.Time
	room      string
	version   string
	shopCount int
	lastError string
	cancel    context.CancelFunc
	bot       *discord.Bot
	mgClient  *mg.Client
}

// NewController creates a new bot controller.
func NewController(db *store.Store, logger *slog.Logger) *DefaultController {
	return &DefaultController{
		db:     db,
		logger: logger,
		status: "stopped",
	}
}

// SetLogger sets the logger after construction (needed for circular init with MultiHandler).
func (c *DefaultController) SetLogger(logger *slog.Logger) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.logger = logger
}

// Start launches the bot engine with the given credentials.
func (c *DefaultController) Start(discordToken, appID string) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return fmt.Errorf("bot is already running")
	}
	c.running = false
	c.status = "starting"
	c.lastError = ""
	c.mu.Unlock()

	// Run startup in a goroutine so the HTTP handler returns immediately
	go c.startAsync(discordToken, appID)
	return nil
}

func (c *DefaultController) startAsync(discordToken, appID string) {
	c.logger.Info("discovering MG game version and room...")

	ctx, cancel := context.WithCancel(context.Background())

	version, roomID, err := mg.DiscoverParams(ctx)
	if err != nil {
		c.mu.Lock()
		c.status = "stopped"
		c.lastError = fmt.Sprintf("failed to discover MG params: %v", err)
		c.mu.Unlock()
		c.logger.Error("failed to discover MG params", "error", err)
		cancel()
		return
	}
	c.logger.Info("discovered MG params", "version", version, "room", roomID)

	// Initialize MG WebSocket client
	mgClient := mg.NewClient(mg.ClientConfig{
		RoomID:  roomID,
		Version: version,
	}, c.logger.With("component", "mg"))

	// Initialize Discord bot
	bot, err := discord.NewBot(discordToken, appID, c.db, mgClient.State(), c.logger.With("component", "discord"))
	if err != nil {
		c.mu.Lock()
		c.status = "stopped"
		c.lastError = fmt.Sprintf("failed to create discord bot: %v", err)
		c.mu.Unlock()
		c.logger.Error("failed to create discord bot", "error", err)
		cancel()
		return
	}

	// Initialize notification engine
	engine := notify.NewEngine(c.db, bot, c.logger.With("component", "notify"))

	// Wire callbacks
	mgClient.OnRestock(engine.HandleRestocks)
	mgClient.OnStockChange(func(changes []mg.StockChange) {
		bot.Board().UpdateAllBoards()
	})
	mgClient.OnConnect(func() {
		// Update shop count on connect
		shops := mgClient.State().GetAllShops()
		c.mu.Lock()
		c.shopCount = len(shops)
		c.mu.Unlock()
		bot.Board().UpdateAllBoards()
	})

	// Start Discord bot
	if err := bot.Start(); err != nil {
		c.mu.Lock()
		c.status = "stopped"
		c.lastError = fmt.Sprintf("failed to start discord bot: %v", err)
		c.mu.Unlock()
		c.logger.Error("failed to start discord bot", "error", err)
		cancel()
		return
	}

	// Start MG WebSocket client
	go func() {
		if err := mgClient.Run(ctx); err != nil && ctx.Err() == nil {
			c.logger.Error("mg client error", "error", err)
		}
	}()

	c.mu.Lock()
	c.running = true
	c.status = "running"
	c.startTime = time.Now()
	c.room = roomID
	c.version = version
	c.cancel = cancel
	c.bot = bot
	c.mgClient = mgClient
	c.mu.Unlock()

	c.logger.Info("magic-guardian is running", "room", roomID, "version", version)
}

// Stop gracefully shuts down the bot engine.
func (c *DefaultController) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	if c.bot != nil {
		c.bot.Stop()
		c.bot = nil
	}
	c.mgClient = nil
	c.running = false
	c.status = "stopped"
	c.shopCount = 0
	c.room = ""
	c.version = ""
	c.logger.Info("bot stopped")
}

// Status returns the current bot status.
func (c *DefaultController) Status() BotStatus {
	c.mu.Lock()
	defer c.mu.Unlock()

	s := BotStatus{
		Running:   c.running,
		Status:    c.status,
		Room:      c.room,
		Version:   c.version,
		Error:     c.lastError,
		ShopCount: c.shopCount,
	}

	if c.running && !c.startTime.IsZero() {
		d := time.Since(c.startTime)
		if d < time.Minute {
			s.Uptime = fmt.Sprintf("%ds", int(d.Seconds()))
		} else if d < time.Hour {
			s.Uptime = fmt.Sprintf("%dm", int(d.Minutes()))
		} else {
			s.Uptime = fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
		}
	}

	return s
}

// Guilds returns the list of guilds the bot is currently in.
func (c *DefaultController) Guilds() []GuildInfo {
	c.mu.Lock()
	bot := c.bot
	c.mu.Unlock()

	if bot == nil {
		return nil
	}

	discordGuilds := bot.Guilds()
	guilds := make([]GuildInfo, len(discordGuilds))
	for i, g := range discordGuilds {
		guilds[i] = GuildInfo{
			ID:   g.ID,
			Name: g.Name,
			Icon: g.Icon,
		}
	}
	return guilds
}

// LeaveGuild removes the bot from the specified guild.
func (c *DefaultController) LeaveGuild(guildID string) error {
	c.mu.Lock()
	bot := c.bot
	c.mu.Unlock()

	if bot == nil {
		return fmt.Errorf("bot is not running")
	}

	return bot.LeaveGuild(guildID)
}
