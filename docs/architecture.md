# Magic Guardian - Architecture

## Overview

Multi-platform application with event-driven architecture. The core is a Go binary that maintains a persistent WebSocket connection to Magic Garden, processes inventory changes, and dispatches notifications via Discord. It supports three deployment modes: headless CLI, web UI, and Android app.

## Deployment Modes

```
┌─────────────────────────────────────────────┐
│           Android Shell (Kotlin)             │
│  MainActivity → WebView → localhost:8090     │
│  GuardianService → ProcessBuilder → Go binary│
│  BootReceiver → auto-start on boot           │
└────────────────────┬────────────────────────┘
                     │ spawns
┌────────────────────▼────────────────────────┐
│            Go Binary (magic-guardian)         │
│  ┌───────────┐  ┌────────────┐               │
│  │  Web UI   │  │  Headless  │               │
│  │  Server   │  │  Mode      │               │
│  │  :8090    │  │  (.env)    │               │
│  └─────┬─────┘  └─────┬─────┘               │
│        └───────┬───────┘                     │
│          Core Engine                          │
│  mg/Client ↔ discord/Bot ↔ notify/Engine     │
│                    ↕                          │
│               store/SQLite                    │
└──────────────────────────────────────────────┘
```

| Mode | Flag | Description |
|------|------|-------------|
| Headless | (default) | Reads credentials from `.env`, no UI |
| Web UI | `-ui` | Embedded HTTP server at `localhost:8090` with REST API and SSE logs |
| Android | `-ui -auto-start` | Launched by `GuardianService`, UI rendered in WebView |

## Component Diagram

```
                                    ┌─────────────────────────────┐
                                    │   Magic Garden WebSocket    │
                                    │      wss://magicgarden.gg   │
                                    └─────────────┬───────────────┘
                                                  │
                                                  │ WebSocket
                                                  ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                          magic-guardian Bot                              │
│                                                                          │
│  ┌─────────────────────┐        ┌─────────────────────────────────────┐ │
│  │   mg/Client         │◄───────│       Event Callbacks              │ │
│  │  • Connection mgmt  │        │ • OnRestock → notify.Engine        │ │
│  │  • Heartbeat        │        │ • OnStockChange → Board.Update     │ │
│  │  • Reconnect        │        │ • OnConnect → Board.Update         │ │
│  │  • State management │        └─────────────────────────────────────┘ │
│  └──────────┬──────────┘                                              │
│             │                                                         │
│  ┌──────────┴──────────┐  ┌──────────────────────────────────────┐   │
│  │   ShopState         │  │   webui/Server (UI mode only)        │   │
│  │  (RWMutex protected)│  │  • HTTP server :8090                 │   │
│  └──────────┬──────────┘  │  • REST API                          │   │
│             │             │  • SSE log streaming                  │   │
│             │             │  • Embedded static assets (go:embed)  │   │
│             │             └──────────────────────────────────────────┘ │
└─────────────┼──────────────────────────────────────────────────────────┘
              │
      ┌───────┼───────┬───────────────┐
      │       │       │               │
      ▼       ▼       ▼               ▼
┌────────┐ ┌────────┐ ┌─────────────────────────────┐
│ notify │ │discord │ │          store/              │
│Engine  │ │ Bot    │ │         SQLite              │
│        │ │        │ │                              │
│• Match │ │• CMDS  │ │ • subscriptions table       │
│• Batch │ │• DMs   │ │ • board_messages table      │
│• Alert │ │• Boards│ │ • config table (web UI)     │
└────────┘ └────┬───┘ └──────────────────────────────┘
                │
                ▼
         ┌──────────────┐
         │ Discord API  │
         └──────────────┘
```

## Components

### Entry Point (`cmd/magic-guardian/main.go`)

Supports two execution paths based on the `-ui` flag:

**Headless mode** (default):
1. Loads `.env` config
2. Discovers MG version/room
3. Initializes SQLite store
4. Creates MG WebSocket client, Discord bot, notification engine
5. Wires event callbacks
6. Starts Discord + WebSocket clients
7. Waits for SIGINT/SIGTERM

**Web UI mode** (`-ui`):
1. Initializes SQLite store
2. Creates `webui.Controller` and `webui.Server`
3. Sets up multi-handler logger (stdout + web buffer)
4. Starts HTTP server on `-listen` address (default `127.0.0.1:8090`)
5. Optionally auto-starts bot if `-auto-start` flag and saved config exist
6. Waits for SIGINT/SIGTERM

```go
// Headless mode wiring
mgClient := mg.NewClient(cfg, logger)
bot, _ := discord.NewBot(token, appID, db, mgClient.State(), logger)
engine := notify.NewEngine(db, bot, logger)
mgClient.OnRestock(engine.HandleRestocks)
mgClient.OnStockChange(func(ch) { bot.Board().UpdateAllBoards() })
mgClient.OnConnect(func() { bot.Board().UpdateAllBoards() })
```

### Web UI Layer (`internal/webui/`)

| File | Responsibility |
|------|----------------|
| `server.go` | HTTP server, API routes, SSE log streaming, embedded static assets |
| `controller.go` | Bot lifecycle management (start/stop/status via REST API) |
| `loghandler.go` | Multi-handler for `slog` (stdout + web log buffer with pub/sub) |
| `static/index.html` | Single-page dashboard (vanilla HTML/CSS/JS, shadcn-inspired dark theme) |

**API Routes:**

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Web UI dashboard |
| GET | `/api/status` | Bot status (running, uptime, room, shops) |
| GET | `/api/config` | Get saved config (token masked) |
| POST | `/api/config` | Save Discord credentials |
| POST | `/api/bot/start` | Start the bot engine |
| POST | `/api/bot/stop` | Stop the bot engine |
| POST | `/api/config/boot` | Toggle start-on-boot setting |
| GET | `/api/guilds` | List connected Discord servers |
| POST | `/api/guilds/leave` | Leave a Discord server |
| GET | `/api/logs` | SSE stream of log lines |

### Android Layer (`android/`)

The Android app is a thin Kotlin wrapper that runs the Go binary as a foreground service:

| File | Responsibility |
|------|----------------|
| `MainActivity.kt` | Full-screen WebView pointing at `http://127.0.0.1:8090` |
| `GuardianService.kt` | Foreground service; launches Go binary via `ProcessBuilder` with `-ui -listen 127.0.0.1:8090 -auto-start`; acquires `PARTIAL_WAKE_LOCK` |
| `BootReceiver.kt` | `BOOT_COMPLETED` receiver; auto-starts `GuardianService` on device boot |

**Android build details:**
- Package: `gg.magicguardian`
- minSdk: 26, targetSdk: 35, compileSdk: 35
- AGP 8.7.3, Kotlin 2.1.0, Gradle 8.11.1
- Go binary cross-compiled with `GOOS=android GOARCH=arm64` using NDK clang
- Binary packaged as `libguardian.so` in `jniLibs/` (PIE executable, not a shared library)

### WebSocket Layer (`internal/mg/`)

| File | Responsibility |
|------|----------------|
| `client.go` | Connection, heartbeat (2s), reconnect (2-60s backoff) |
| `shop.go` | ShopState with RWMutex, ApplyPatches() |
| `messages.go` | Protocol types (ServerMessage, Patch, WelcomeState) |
| `discover.go` | HTTP discovery of version and room ID |

**JSON Patch Paths:**
- Inventory: `/child/data/shops/{shop}/inventory/{index}/initialStock`
- Timer: `/child/data/shops/{shop}/secondsUntilRestock`

### Discord Layer (`internal/discord/`)

| File | Responsibility |
|------|----------------|
| `bot.go` | Session, slash commands, interactions |
| `embeds.go` | Rich embed builders (stock alerts, inventory) |
| `board.go` | Live stock board management |

**Slash Commands:**

| Command | Description | Options |
|---------|-------------|---------|
| `/subscribe` | Get notified when item is in stock | `item` (autocomplete) |
| `/unsubscribe` | Stop notifications for item | `item` (autocomplete) |
| `/watchlist` | Show current subscriptions | - |
| `/stock` | Show shop inventory | `shop` (seed/tool/egg/decor) |
| `/restock` | Show time until next restock | - |
| `/setup-stock-board` | Create live stock board | `name` (optional) |
| `/delete-stock-board` | Remove stock board | - |

### Notification Engine (`internal/notify/engine.go`)

Matches stock changes to subscriptions and batches alerts.

**Batching Logic:** All items that restock at once → one DM per user

```go
HandleRestocks(changes)
├── Group changes by itemID
├── For each item:
│   └── GetSubscribersForItem(itemID)
├── Group subscribers by userID
└── For each user:
    └── SendStockAlert(userID, batchedChanges)
```

### Persistence (`internal/store/sqlite.go`)

| Operation | Returns |
|-----------|---------|
| `Subscribe(user, guild, item, shop)` | `bool` (created vs existing) |
| `Unsubscribe(user, item)` | `bool` (found vs not found) |
| `GetUserSubscriptions(user)` | `[]Subscription` |
| `GetSubscribersForItem(item)` | `[]Subscription` |
| `GetBoardConfig(guild)` | `*BoardConfig` |
| `GetAllBoardConfigs()` | `[]BoardConfig` |

## Concurrency

| Component | Model |
|-----------|-------|
| WebSocket reader | Separate goroutine |
| Heartbeat | Separate goroutine (2s interval) |
| ShopState | RWMutex for thread-safe access |
| Board timestamps | Periodic ticker (1 minute) |
| Discord events | Handled in discordgo's goroutine |

## Data Flow

### Restock Notification

```
WebSocket → PartialState patch (timer reset detected)
    ↓
ShopState.ApplyPatches() includes ALL in-stock items from shop
    ↓
OnRestock callback → notify.Engine
    ↓
store.GetSubscribersForItem(itemID) for each item
    ↓
For each subscriber:
    bot.SendStockAlert(userID, batchedChanges)
    ↓
Discord DM with embed + unsubscribe buttons
```

### Stock Board Update

```
WebSocket → Any patch
    ↓
ShopState.ApplyPatches()
    ↓
OnStockChange callback → Board.UpdateAllBoards()
    ↓
store.GetAllBoardConfigs()
    ↓
For each guild/shop:
    ChannelMessageEditComplex(newEmbed)
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| WebSocket disconnect | Auto-reconnect with exponential backoff |
| Discord API error | Logged, minimal impact |
| Database error | Logged, operation fails gracefully |
| Interaction timeout | Deferred response as fallback |