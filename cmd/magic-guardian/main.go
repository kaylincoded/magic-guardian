package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/kaylincoded/magic-guardian/internal/discord"
	"github.com/kaylincoded/magic-guardian/internal/mg"
	"github.com/kaylincoded/magic-guardian/internal/notify"
	"github.com/kaylincoded/magic-guardian/internal/store"
	"github.com/kaylincoded/magic-guardian/internal/webui"
)

func main() {
	// Flags
	uiMode := flag.Bool("ui", false, "start in web UI mode (for Android/mobile)")
	listenAddr := flag.String("listen", "127.0.0.1:8090", "web UI listen address")
	dbPath := flag.String("db", "magic-guardian.db", "path to SQLite database")
	autoStart := flag.Bool("auto-start", false, "auto-start bot on launch (ui mode only)")
	flag.Parse()

	// Initialize database first (needed by both modes)
	db, err := store.New(*dbPath)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if *uiMode {
		runUIMode(db, *listenAddr, *autoStart)
	} else {
		runHeadlessMode(db)
	}
}

// runUIMode starts the web UI server and manages the bot via HTTP API.
func runUIMode(db *store.Store, listenAddr string, autoStart bool) {
	// Create controller and web UI server
	controller := webui.NewController(db, nil)  // logger set below
	srv := webui.NewServer(db, controller, nil) // logger set below

	// Create logger that writes to both stdout and the web UI log buffer
	baseHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	multiHandler := webui.NewMultiHandler(baseHandler, srv.LogBuffer())
	logger := slog.New(multiHandler)

	// Now inject the logger into controller and server
	controller.SetLogger(logger)
	srv.SetLogger(logger)

	// Start web UI server
	if err := srv.Start(listenAddr); err != nil {
		logger.Error("failed to start web UI", "error", err)
		os.Exit(1)
	}
	defer srv.Stop()

	logger.Info("magic-guardian web UI ready", "url", "http://"+listenAddr)

	// Auto-start if the flag is set AND the user enabled it in settings
	startOnBoot, _ := db.GetConfig("start_on_boot")
	if autoStart && startOnBoot == "true" {
		token, _ := db.GetConfig("discord_token")
		appID, _ := db.GetConfig("app_id")
		if token != "" && appID != "" {
			logger.Info("auto-starting bot from saved config")
			if err := controller.Start(token, appID); err != nil {
				logger.Error("auto-start failed", "error", err)
			}
		} else {
			logger.Warn("auto-start requested but no saved config found")
		}
	}

	// Wait for shutdown signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	logger.Info("shutting down...")
	controller.Stop()
}

// runHeadlessMode is the original CLI behavior (reads from .env or env vars).
func runHeadlessMode(db *store.Store) {
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

	// Wire the restock callback (DM alerts for 0->N)
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
