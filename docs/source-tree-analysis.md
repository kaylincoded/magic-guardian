# Magic Guardian - Source Tree Analysis

## Project Root Structure

```
magic-guardian/                          # Module: github.com/kaylincoded/magic-guardian
├── cmd/                                 # Entry points
│   └── magic-guardian/
│       └── main.go                      # → Entry point (main package)
├── internal/                            # Private application code
│   ├── discord/                         # Discord bot interface
│   │   ├── bot.go                       # → Bot session, slash commands
│   │   ├── embeds.go                    # → Embed builders
│   │   └── board.go                     # → Stock board management
│   ├── mg/                              # Magic Garden protocol
│   │   ├── client.go                    # → WebSocket client
│   │   ├── messages.go                  # → Protocol types
│   │   ├── shop.go                      # → State management
│   │   └── discover.go                  # → Version discovery
│   ├── notify/                          # Notification matching
│   │   └── engine.go                    # → Subscription matching
│   └── store/                           # Persistence
│       └── sqlite.go                    # → SQLite store
├── docs/                                # Documentation
├── go.mod                               # Module: go 1.25
├── go.sum
├── .env                                 # Runtime config
├── .env.example
├── magic-guardian                       # Compiled binary
├── magic-guardian.db                    # SQLite database
└── README.md
```

## Directory Purpose Analysis

### `cmd/magic-guardian/` - Application Entry Point

**Purpose:** Dependency injection and application lifecycle management

**Key Responsibilities:**
- Load configuration from environment
- Initialize all components with dependencies
- Wire event callbacks between components
- Start concurrent services (Discord + WebSocket)
- Handle graceful shutdown

**Entry Point Flow:**
```
main()
├── godotenv.Load()                      # Load .env
├── mg.DiscoverParams()                  # Get version/room
├── store.New("magic-guardian.db")       # Open DB
├── mg.NewClient(cfg, logger)            # Create MG client
├── discord.NewBot()                     # Create bot
├── notify.NewEngine()                   # Create notification engine
├── Wire callbacks:
│   ├── mgClient.OnRestock(engine)
│   ├── mgClient.OnStockChange(board)
│   └── mgClient.OnConnect(board)
├── bot.Start()                          # Start Discord
├── mgClient.Run(ctx)                    # Run MG client (blocking)
└── Wait for SIGINT/SIGTERM
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
```

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
├── discord.NewBot()
│   ├── discord.Board.NewBoard()
│   └── discordgo.New()
├── mg.NewClient()
├── notify.NewEngine()
└── store.New()

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
│   └── modernc.org/sqlite
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
```

## Build Output

```
go build -o magic-guardian ./cmd/magic-guardian/
```

Produces standalone binary with:
- No external runtime dependencies
- SQLite embedded via cgo
- Static linking for most dependencies