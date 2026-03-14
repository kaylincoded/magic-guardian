package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/kaylin/magic-guardian/internal/discord"
	"github.com/kaylin/magic-guardian/internal/mg"
	"github.com/kaylin/magic-guardian/internal/notify"
	"github.com/kaylin/magic-guardian/internal/store"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if err := godotenv.Load(); err != nil {
		logger.Warn("no .env file found, using environment variables")
	}

	discordToken := os.Getenv("DISCORD_TOKEN")
	appID := os.Getenv("DISCORD_APP_ID")

	if discordToken == "" || appID == "" {
		logger.Error("DISCORD_TOKEN and DISCORD_APP_ID are required")
		os.Exit(1)
	}

	// Auto-discover current game version and room ID
	logger.Info("discovering MG game version and room...")
	version, roomID, err := mg.DiscoverParams(context.Background())
	if err != nil {
		logger.Error("failed to discover MG params", "error", err)
		os.Exit(1)
	}
	logger.Info("discovered MG params", "version", version, "room", roomID)

	// Initialize SQLite store
	db, err := store.New("magic-guardian.db")
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Initialize MG WebSocket client
	mgClient := mg.NewClient(mg.ClientConfig{
		RoomID:  roomID,
		Version: version,
	}, logger.With("component", "mg"))

	// Initialize Discord bot
	bot, err := discord.NewBot(discordToken, appID, db, mgClient.State(), logger.With("component", "discord"))
	if err != nil {
		logger.Error("failed to create discord bot", "error", err)
		os.Exit(1)
	}

	// Initialize notification engine
	engine := notify.NewEngine(db, bot, logger.With("component", "notify"))

	// Wire the restock callback (DM alerts for 0→N)
	mgClient.OnRestock(engine.HandleRestocks)

	// Wire the stock change callback (board updates for any change)
	mgClient.OnStockChange(func(changes []mg.StockChange) {
		bot.Board().UpdateAllBoards()
	})

	// Refresh boards on every connect/reconnect with fresh state
	mgClient.OnConnect(func() {
		bot.Board().UpdateAllBoards()
	})

	// Start Discord bot
	if err := bot.Start(); err != nil {
		logger.Error("failed to start discord bot", "error", err)
		os.Exit(1)
	}
	defer bot.Stop()

	// Start MG WebSocket client
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := mgClient.Run(ctx); err != nil && ctx.Err() == nil {
			logger.Error("mg client error", "error", err)
		}
	}()

	logger.Info("magic-guardian is running", "room", roomID, "version", version)

	// Wait for shutdown signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	logger.Info("shutting down...")
	cancel()
}
