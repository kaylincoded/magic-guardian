# Magic Guardian - Project Overview

## Executive Summary

Magic Guardian is a Discord bot that monitors Magic Garden game shop inventory and sends real-time "in stock" notifications to subscribed users via Discord DM. It is designed to comply with Magic Garden's bot policy as a read-only observer. The bot connects to the Magic Garden WebSocket server as an anonymous observer, tracking all 4 shop categories (Seed, Tool, Egg, Decor) and detecting when watched items become available.

## Key Features

- **Real-time shop monitoring** via WebSocket connection to magicgarden.gg
- **Stock alert notifications** delivered as Discord DMs with batched updates
- **Slash command interface** with autocomplete for subscriptions
- **Live stock boards** - configurable server channels showing current inventory
- **Persistent subscriptions** stored in SQLite database
- **Auto-reconnect** with exponential backoff for WebSocket reliability
- **Policy compliant** - read-only operation, anonymous observer

## Technology Stack

| Category | Technology | Version |
|----------|-----------|---------|
| Language | Go | 1.25 |
| Discord API | discordgo | v0.29.0 |
| WebSocket | gorilla/websocket | v1.5.3 |
| Database | SQLite (mattn/go-sqlite3) | v1.14.34 |
| Structured Logging | slog (stdlib) | - |
| Configuration | godotenv | v1.5.1 |
| Text Formatting | golang.org/x/text | v0.35.0 |
| Web UI | Embedded HTML/CSS/JS via go:embed | - |
| Android | Kotlin 2.1.0, AGP 8.7.3, Gradle 8.11.1 | - |
| Android SDK | compileSdk 35, minSdk 26, targetSdk 35 | - |
| Build/Release | GoReleaser | - |
| CI/CD | GitHub Actions | - |
| Testing | go test, race detector | 112 tests |

## Architecture Type

Multi-platform application with event-driven architecture. The bot supports three deployment modes:

1. **Headless CLI** (default) -- reads credentials from `.env`, no UI
2. **Web UI mode** (`-ui`) -- embedded HTTP server with REST API and dashboard
3. **Android app** -- Kotlin wrapper running the Go binary as a foreground service

Core engine behavior across all modes:

1. Maintains a persistent WebSocket connection to the game server
2. Processes incoming JSON Patch messages for inventory changes
3. Matches stock changes against user subscriptions
4. Dispatches notifications via Discord API

## Repository Structure

```
magic-guardian/
├── cmd/
│   └── magic-guardian/       # Entry point (headless + web UI modes)
├── internal/
│   ├── discord/              # Discord bot, embeds, and stock board
│   │   ├── bot.go            # Bot session and slash commands
│   │   ├── embeds.go         # Rich embed builders
│   │   └── board.go          # Live stock board management
│   ├── mg/                   # Magic Garden WebSocket client
│   │   ├── client.go         # WebSocket connection and handlers
│   │   ├── messages.go       # Protocol message types
│   │   ├── shop.go           # Shop state management
│   │   └── discover.go       # Version/room discovery
│   ├── notify/               # Notification matching engine
│   │   └── engine.go         # Subscription matching
│   ├── store/                # SQLite persistence
│   │   └── sqlite.go         # Subscriptions, board config, settings
│   └── webui/                # Embedded web dashboard
│       ├── server.go         # HTTP server, REST API, SSE logs
│       ├── controller.go     # Bot lifecycle management
│       ├── loghandler.go     # Multi-handler slog (stdout + web)
│       └── static/           # Embedded assets (go:embed)
├── android/                  # Android wrapper app (Kotlin)
│   ├── app/src/main/
│   │   ├── java/gg/magicguardian/  # MainActivity, GuardianService, BootReceiver
│   │   └── jniLibs/                # Cross-compiled Go binaries
│   └── build.gradle.kts
├── .github/workflows/        # CI/CD pipelines
│   ├── ci.yml                # Test + build on push/PR
│   └── release.yml           # Build + publish on version tag
├── releases/                 # Pre-built binaries and APK
├── docs/                     # This documentation
├── Makefile                  # Build, test, lint, android targets
├── go.mod / go.sum           # Go modules
└── magic-guardian.db         # SQLite database (runtime)
```

## How It Works

1. **Discovery**: On startup, fetches current game version and room ID from magicgarden.gg
2. **Connection**: Establishes authenticated WebSocket connection as anonymous player
3. **State Sync**: Receives Welcome message with full shop inventory
4. **Change Detection**: Applies PartialState JSON Patch messages every ~1 second
5. **Restock Detection**: Identifies items transitioning from 0 to N stock
6. **Notification**: Matches restocked items against subscriptions and sends batched DMs
7. **Persistence**: Stores subscriptions and board configurations in SQLite

## Policy Compliance

> [!IMPORTANT]
> This bot is designed to comply with Magic Garden's bot policy. It operates as a **read-only observer** — connects anonymously, receives shop data, and does nothing else.

> [!NOTE]
> **Policy compliance means:**
>
> - Operates as anonymous observer (no authentication)
> - Read-only — never sends game commands
> - Does not interact with game objects
> - Does not automate purchasing or gameplay
>
> **Official policy:** [Magic Circle Discord](https://discord.com/invite/magiccircle) | [Bot & Tool Policy](https://ptb.discord.com/channels/808935495543160852/1428205518278885457) (Discord login required)

## Performance Characteristics

- **WebSocket heartbeat**: 2-second intervals
- **State patch frequency**: ~1 per second
- **Reconnect backoff**: 2-60 seconds with jitter
- **Database**: Single-file SQLite, negligible latency
- **Memory footprint**: Minimal, only stores in-memory shop state

## License

MIT