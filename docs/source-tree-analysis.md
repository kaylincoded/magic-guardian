# Magic Guardian - Source Tree Analysis

## Project Root Structure

```
magic-guardian/                          # Module: github.com/kaylincoded/magic-guardian
├── cmd/                                 # Entry points
│   └── magic-guardian/
│       └── main.go                      # → Entry point (headless + web UI modes)
├── internal/                            # Private application code
│   ├── discord/                         # Discord bot interface
│   │   ├── bot.go                       # → Bot session, slash commands
│   │   ├── embeds.go                    # → Embed builders
│   │   └── board.go                     # → Stock board management
│   ├── mg/                              # Magic Garden protocol
│   │   ├── client.go                    # → WebSocket client
│   │   ├── client_test.go              # → 29 tests: diffShopState, handleWelcome, handleMessage
│   │   ├── messages.go                  # → Protocol types
│   │   ├── messages_test.go            # → 5 tests: JSON unmarshaling
│   │   ├── shop.go                      # → State management
│   │   ├── shop_test.go                # → 18 tests: ApplyPatches, timers, format
│   │   ├── discover.go                  # → Version discovery
│   │   └── discover_test.go            # → 2 tests: regex extraction
│   ├── notify/                          # Notification matching
│   │   ├── engine.go                    # → Subscription matching
│   │   └── engine_test.go              # → 8 tests: batching, errors, case handling
│   ├── store/                           # Persistence
│   │   ├── sqlite.go                    # → SQLite store (subs, board, config)
│   │   └── sqlite_test.go              # → 21 tests: CRUD, constraints, upsert
│   └── webui/                           # Web UI (ui mode only)
│       ├── server.go                    # → HTTP server, API routes, SSE logs
│       ├── server_test.go              # → 29 tests: all endpoints, token masking, SSE
│       ├── controller.go                # → Bot lifecycle management
│       ├── loghandler.go                # → Multi-handler slog
│       └── static/
│           ├── index.html               # → Single-page dashboard (go:embed)
│           └── logo.png
├── android/                             # Android wrapper app
│   ├── build.gradle.kts                 # Root: AGP 8.7.3, Kotlin 2.1.0
│   ├── settings.gradle.kts
│   └── app/
│       ├── build.gradle.kts             # compileSdk 35, minSdk 26
│       └── src/main/
│           ├── AndroidManifest.xml
│           ├── java/gg/magicguardian/
│           │   ├── MainActivity.kt      # → WebView → localhost:8090
│           │   ├── GuardianService.kt   # → Foreground service
│           │   └── BootReceiver.kt      # → Auto-start on boot
│           ├── jniLibs/arm64-v8a/       # Cross-compiled Go binary
│           └── res/                     # Icons, strings, themes
├── .github/workflows/                   # CI/CD
│   ├── ci.yml                           # → Test + build on push/PR
│   └── release.yml                      # → Build all platforms + release on tag
├── releases/                            # Pre-built binaries + APK
├── docs/                                # Documentation
├── Makefile                             # Build, test, lint, android targets
├── go.mod                               # Module: go 1.25
├── go.sum
├── .env.example                         # Environment template
├── .goreleaser.yaml                     # Cross-compilation config
├── magic-guardian.db                    # SQLite database (runtime)
└── README.md
```

## Directory Purpose Analysis

### `cmd/magic-guardian/` - Application Entry Point

**Purpose:** Dependency injection, application lifecycle management, and mode selection

**Key Responsibilities:**
- Parse flags (`-ui`, `-listen`, `-db`, `-auto-start`)
- Initialize SQLite store
- Route to headless or web UI mode
- Wire event callbacks between components
- Handle graceful shutdown

**Entry Point Flow (Headless Mode):**
```
main()
├── flag.Parse()                         # Parse CLI flags
├── store.New(dbPath)                    # Open DB
├── runHeadlessMode(db)
│   ├── godotenv.Load()                  # Load .env
│   ├── mg.DiscoverParams()              # Get version/room
│   ├── mg.NewClient(cfg, logger)        # Create MG client
│   ├── discord.NewBot()                 # Create bot
│   ├── notify.NewEngine()               # Create notification engine
│   ├── Wire callbacks:
│   │   ├── mgClient.OnRestock(engine)
│   │   ├── mgClient.OnStockChange(board)
│   │   └── mgClient.OnConnect(board)
│   ├── bot.Start()                      # Start Discord
│   ├── mgClient.Run(ctx)               # Run MG client
│   └── Wait for SIGINT/SIGTERM
```

**Entry Point Flow (Web UI Mode):**
```
main()
├── flag.Parse()                         # -ui flag set
├── store.New(dbPath)                    # Open DB
├── runUIMode(db, listenAddr, autoStart)
│   ├── webui.NewController(db)          # Bot lifecycle controller
│   ├── webui.NewServer(db, controller)  # HTTP server
│   ├── webui.NewMultiHandler()          # Dual-output logger
│   ├── srv.Start(listenAddr)            # Start HTTP on :8090
│   ├── Auto-start bot (if config exists)
│   └── Wait for SIGINT/SIGTERM
```

**Key File:**
- `main.go` - Single file, no subpackages

### `internal/discord/` - Discord Interface

**Purpose:** All Discord API interactions

**Subcomponents:**

| File | Responsibility |
|------|----------------|
| `bot.go` | Session management, slash command registration, interaction routing |
| `embeds.go` | Rich embed builders for all response types |
| `board.go` | Live stock board channel management and updates |

**Key Types:**
```go
type Bot struct {
    session *discordgo.Session
    store   *store.Store
    mgState *mg.ShopState
    logger  *slog.Logger
    appID   string
    board   *Board
}

type Board struct {
    session  *discordgo.Session
    store    *store.Store
    mgState  *mg.ShopState
    logger   *slog.Logger
    appID    string
    boardIDs sync.Map  // guild:channel:message → msg_id
}

type WatchlistItem struct {
    ItemID       string
    ShopType     string
    CurrentStock int
}
```

**Exported Functions:**
- `BuildStockAlertEmbed(changes)` - Creates DM alert embed
- `BuildStockEmbed(shopType, shop)` - Creates shop inventory embed
- `BuildWatchlistEmbed(items)` - Creates subscription list embed
- `BuildRestockEmbed(shops)` - Creates timer display embed

### `internal/mg/` - Magic Garden Protocol

**Purpose:** WebSocket client and game state management

**Subcomponents:**

| File | Responsibility |
|------|----------------|
| `client.go` | WebSocket connection, heartbeat, reconnect, message dispatch |
| `messages.go` | Protocol type definitions (ServerMessage, Patch, Shop, etc.) |
| `shop.go` | Thread-safe shop state, patch application, item name formatting |
| `discover.go` | HTTP discovery of game version and room ID |

**Key Types:**
```go
type Client struct {
    cfg           ClientConfig
    state         *ShopState
    conn          *websocket.Conn
    onRestock     func([]StockChange)
    onStockChange func([]StockChange)
    onConnect     func()
}

type ShopState struct {
    mu    sync.RWMutex
    shops map[string]*Shop  // "seed" | "tool" | "egg" | "decor"
}

type StockChange struct {
    ShopType string
    Item     ShopItem
    OldStock int
    NewStock int
}
```

**State Machine:**
```
disconnected → authenticating → connected → ready
                                    ↓
                              disconnect → reconnect
```

### `internal/notify/` - Notification Engine

**Purpose:** Match stock changes to user subscriptions

**Structure:** Single file, simple component

**Key Type:**
```go
type Engine struct {
    store  *store.Store
    sender AlertSender  // interface for sending alerts
    logger *slog.Logger
}
```

**Flow:**
```
HandleRestocks(changes)
├── Group changes by itemID
├── For each item:
│   └── GetSubscribersForItem(itemID)
├── Group subscribers by userID
└── For each user:
    └── SendStockAlert(userID, batchedChanges)
```

### `internal/store/` - Persistence

**Purpose:** SQLite storage for subscriptions and board configurations

**Structure:** Single file

**Key Types:**
```go
type Store struct {
    db *sql.DB
}

type Subscription struct {
    ID       int64
    UserID   string
    GuildID  string
    ItemID   string
    ShopType string
}

type BoardConfig struct {
    GuildID  string
    Channels map[string]string  // shop_type → channel_id
    Messages map[string]string  // shop_type → message_id
}
```

**Database Schema:**
```
subscriptions:
  - id (PK, auto)
  - user_id
  - guild_id
  - item_id
  - shop_type
  - UNIQUE(user_id, item_id)

board_messages:
  - guild_id (PK part)
  - channel_id
  - shop_type (PK part)
  - message_id

config:
  - key (PK)           # discord_token, app_id, start_on_boot
  - value
```

### `internal/webui/` - Web Dashboard

**Purpose:** Embedded HTTP server for browser-based bot management (ui mode only)

**Subcomponents:**

| File | Responsibility |
|------|----------------|
| `server.go` | HTTP server, API routes, SSE log streaming, embedded static assets via `go:embed` |
| `controller.go` | Bot lifecycle: start/stop/status, wraps Discord + MG client creation |
| `loghandler.go` | `slog.Handler` that writes to both stdout and an in-memory ring buffer with pub/sub |
| `static/index.html` | Single-page dashboard: setup form, bot status, guild management, live logs |

**Key Design Decisions:**
- Single HTML file with inline CSS/JS (no build toolchain, works via `go:embed`)
- Shadcn-inspired dark theme (green preset)
- Early bind failure detection: `ListenAndServe` errors propagated within 100ms
- SSE (Server-Sent Events) for real-time log streaming to the browser
- Token masking: API never returns full Discord token to frontend

### `android/` - Android Wrapper

**Purpose:** Native Android app that runs the Go binary as a foreground service

**Key Files:**

| File | Responsibility |
|------|----------------|
| `MainActivity.kt` | Full-screen `WebView` loading `http://127.0.0.1:8090`, dark status bar theming |
| `GuardianService.kt` | `Service` subclass; extracts `libguardian.so` via `applicationInfo.nativeLibraryDir`; launches it with `ProcessBuilder`; acquires `PARTIAL_WAKE_LOCK`; posts foreground notification |
| `BootReceiver.kt` | `BroadcastReceiver` for `BOOT_COMPLETED`; starts `GuardianService` on device boot |

**Build Configuration:**
- `GOOS=android GOARCH=arm64` with NDK clang for CGO (sqlite3)
- Go binary compiled as PIE executable (not c-shared), renamed to `libguardian.so` for Android packaging
- `useLegacyPackaging = true` keeps `.so` uncompressed in the APK

## Critical Paths

### Notification Path (Hot Path)

Most frequent execution path - runs on every stock change:

```
WebSocket message
    ↓
client.handleMessage()
    ↓
client.handlePartialState()
    ↓
state.ApplyPatches() → returns []StockChange
    ↓
onRestock callback
    ↓
notify.Engine.HandleRestocks()
    ↓
store.GetSubscribersForItem()
    ↓
bot.SendStockAlert()
    ↓
Discord API
```

### Board Update Path

Runs on every stock change (less critical):

```
WebSocket message
    ↓
onStockChange callback
    ↓
board.UpdateAllBoards()
    ↓
store.GetAllBoardConfigs()
    ↓
session.ChannelMessageEditComplex()
```

### Slash Command Path

User-triggered:

```
Discord interaction
    ↓
bot.handleInteraction()
    ↓
bot.handleCommand()
    ↓
cmd{Subscribe,Unsubscribe,Watchlist,Stock,Restock,SetupStockBoard}()
    ↓
store operation or mgState query
    ↓
respond with embed
```

## File Dependencies

```
main.go
├── discord.NewBot()            # Headless mode
│   ├── discord.Board.NewBoard()
│   └── discordgo.New()
├── mg.NewClient()              # Headless mode
├── notify.NewEngine()          # Headless mode
├── webui.NewController()       # UI mode
├── webui.NewServer()           # UI mode
└── store.New()                 # Both modes

discord/bot.go
├── discord/embeds.go (embed builders)
├── discord/board.go (board management)
├── store (subscription queries)
└── mg.ShopState (inventory queries)

mg/client.go
├── mg/messages.go (types)
├── mg/shop.go (ShopState)
└── mg/discover.go (DiscoverParams)

notify/engine.go
└── store (subscription queries)

webui/server.go
├── webui/controller.go (bot lifecycle)
├── webui/loghandler.go (multi-handler)
├── webui/static/ (go:embed)
└── store (config operations)
```

## Import Graph

```
main
├── github.com/joho/godotenv
├── internal/discord
│   └── github.com/bwmarrin/discordgo
├── internal/mg
│   └── github.com/gorilla/websocket
├── internal/notify
├── internal/store
│   └── github.com/mattn/go-sqlite3
├── internal/webui
└── golang.org/x/text/cases

internal/discord
├── internal/mg
├── internal/store
└── golang.org/x/text

internal/mg (standalone, no internal deps)

internal/notify
├── internal/mg
└── internal/store

internal/store (standalone, no internal deps)

internal/webui
├── internal/store
├── internal/discord
├── internal/mg
└── internal/notify
```

## Build Output

```
# Desktop (macOS/Linux/Windows)
go build -o magic-guardian ./cmd/magic-guardian/

# Android (arm64, requires NDK)
CGO_ENABLED=1 GOOS=android GOARCH=arm64 \
  CC=$NDK/.../aarch64-linux-android26-clang \
  go build -ldflags="-s -w" -o libguardian.so ./cmd/magic-guardian/
```

Desktop binary:
- No external runtime dependencies
- SQLite embedded via CGO
- Web UI assets embedded via `go:embed`

Android binary:
- PIE executable compiled with `GOOS=android`
- NDK clang for CGO (sqlite3)
- Packaged as `libguardian.so` in APK jniLibs