# Magic Guardian - Architecture

## High-Level Architecture

```
                                    ┌─────────────────────────────────────┐
                                    │      Magic Garden WebSocket         │
                                    │         (wss://magicgarden.gg)      │
                                    └─────────────────┬───────────────────┘
                                                      │
                                                      │ WebSocket
                                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           magic-guardian Bot                                 │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                     internal/mg (WebSocket Layer)                    │    │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐  │    │
│  │  │   Client     │  │   ShopState  │  │   Message Types          │  │    │
│  │  │  (connect,   │  │  (in-memory  │  │   - ServerMessage        │  │    │
│  │  │   heartbeat, │  │   inventory  │  │   - WelcomeState         │  │    │
│  │  │   reconnect) │  │   caching)   │  │   - Patch (JSON Patch)   │  │    │
│  │  └──────┬───────┘  └──────┬───────┘  └──────────────────────────┘  │    │
│  │         │                 │                                            │    │
│  │         └────────┬────────┘                                            │    │
│  │                  │                                                     │    │
│  │         ┌────────▼────────┐                                            │    │
│  │         │  Event Callbacks │                                           │    │
│  │         │ - OnRestock      │                                           │    │
│  │         │ - OnStockChange  │                                           │    │
│  │         │ - OnConnect      │                                           │    │
│  │         └────────┬────────┘                                            │    │
│  └──────────────────┼─────────────────────────────────────────────────────┘    │
│                     │                                                          │
│         ┌───────────┼───────────┐                                            │
│         │           │           │                                            │
│         ▼           ▼           ▼                                            │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────────────────────────┐          │
│  │   notify/   │ │  discord/   │ │           store/                 │          │
│  │   Engine    │ │   Bot       │ │          SQLite                  │          │
│  │             │ │             │ │                                  │          │
│  │  - Match    │ │  - Slash    │ │  ┌────────────────────────────┐  │          │
│  │    subs to  │ │    commands │ │  │      subscriptions         │  │          │
│  │    changes  │ │  - DM sends │ │  │      (user_id, item_id)     │  │          │
│  │  - Batch    │ │  - Board    │ │  └────────────────────────────┘  │          │
│  │    alerts   │ │    updates  │ │  ┌────────────────────────────┐  │          │
│  └─────────────┘ └──────┬──────┘ │  │      board_messages        │  │          │
│                         │        │  │      (guild, channel, msg) │  │          │
│                         │        │  └────────────────────────────┘  │          │
│                         │        └──────────────────────────────────┘          │
│                         │                                                       │
│                         ▼                                                       │
│              ┌──────────────────────┐                                          │
│              │   Discord API        │                                          │
│              │   (discordgo)        │                                          │
│              └──────────────────────┘                                          │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Component Architecture

### Entry Point (`cmd/magic-guardian/main.go`)

The main function orchestrates initialization:

1. Loads environment variables from `.env`
2. Discovers Magic Garden version and room ID
3. Initializes SQLite store (`store.New()`)
4. Creates MG WebSocket client (`mg.NewClient()`)
5. Creates Discord bot (`discord.NewBot()`)
6. Creates notification engine (`notify.NewEngine()`)
7. Wires event callbacks between components
8. Starts Discord bot and MG client concurrently
9. Waits for shutdown signal (SIGINT/SIGTERM)

```go
// Dependency wiring
mgClient := mg.NewClient(cfg, logger)
bot, _ := discord.NewBot(token, appID, db, mgClient.State(), logger)
engine := notify.NewEngine(db, bot, logger)

// Event callbacks
mgClient.OnRestock(engine.HandleRestocks)
mgClient.OnStockChange(func(changes) { bot.Board().UpdateAllBoards() })
mgClient.OnConnect(func() { bot.Board().UpdateAllBoards() })
```

### WebSocket Layer (`internal/mg/`)

The `mg` package handles all communication with the Magic Garden server.

#### Client (`client.go`)

- Manages WebSocket connection lifecycle
- Implements auto-reconnect with exponential backoff
- Sends heartbeat every 2 seconds
- Handles server pings with pong responses
- Authenticates as anonymous player before connecting
- Dispatches messages to handlers

**Connection Flow:**
```
DiscoverParams() → authenticate() → Dial() → sendJoinMessages() → heartbeat()
```

**Reconnect Strategy:**
- Exponential backoff: 2s base + random jitter up to 60s max
- Rediscovered room/version on each reconnect

#### Shop State (`shop.go`)

Thread-safe in-memory representation of all shop inventory.

- `ShopState` - RWMutex-protected map of shop_type → Shop
- `Shop` - Contains inventory array and restock timer
- `ShopItem` - Individual item with stock count
- Supports `ApplyPatches()` for JSON Patch updates
- Returns immutable copies via `GetShop()` / `GetAllShops()`

**JSON Patch Paths:**
- Inventory: `/child/data/shops/{shop}/inventory/{index}/initialStock`
- Timer: `/child/data/shops/{shop}/secondsUntilRestock`

#### Message Types (`messages.go`)

Protocol structures for server-client communication:

- `ServerMessage` - Type envelope (Welcome, PartialState, Config)
- `WelcomeState` - Full state on connection
- `Patch` - RFC 6902 JSON Patch operation
- `Shop` / `ShopItem` - Data models

#### Discovery (`discover.go`)

Fetches current game version and room ID:

- Makes HTTP request to magicgarden.gg
- Extracts version from HTML/scripts (regex: `version/(\d+)`)
- Extracts room ID from redirect or HTML (regex: `/r/([A-Za-z0-9]+)`)

### Discord Layer (`internal/discord/`)

The `discord` package handles all Discord interactions.

#### Bot (`bot.go`)

Manages Discord session, slash commands, and interactions.

**Slash Commands:**
| Command | Description | Options |
|---------|-------------|---------|
| `/subscribe` | Get notified when item is in stock | `item` (required, autocomplete) |
| `/unsubscribe` | Stop notifications for item | `item` (required, autocomplete) |
| `/watchlist` | Show current subscriptions | - |
| `/stock` | Show current shop inventory | `shop` (seed/tool/egg/decor) |
| `/restock` | Show time until next restock | - |
| `/setup-stock-board` | Create live stock board channel | `name` (optional) |

**Interaction Handlers:**
- Application commands (slash commands)
- Autocomplete requests (item suggestions)
- Message components (buttons, select menus)

#### Embeds (`embeds.go`)

Rich embed builders for all bot responses:

- `BuildStockAlertEmbed()` - DM notification for restocks
- `BuildStockEmbed()` - Shop inventory display
- `BuildWatchlistEmbed()` - User subscription list
- `BuildRestockEmbed()` - Restock timer display

#### Board (`board.go`)

Live stock board management for server channels.

**Board Structure:**
- Category channel: "📦 Magic Garden Stock"
- 4 text channels: one per shop type
- Each channel has embed + "Update Subscriptions" button

**Features:**
- Real-time embed updates on stock changes
- Select menus for bulk subscription management
- Ephemeral responses for user privacy
- Per-guild configuration persistence

### Notification Engine (`internal/notify/`)

Matches stock changes against user subscriptions.

#### Engine (`engine.go`)

- `HandleRestocks()` - Processes batched stock changes
- Groups alerts by subscribed user
- Sends one DM per restock event (not per item)
- Uses `store.GetSubscribersForItem()` to find subscribers

**Notification Batching:**
```go
// All items that restock at once → single DM per user
userAlerts[userID] = append(userAlerts[userID], ch)
e.sender.SendStockAlert(userID, alerts)
```

### Persistence Layer (`internal/store/`)

SQLite storage for subscriptions and board configurations.

#### Store (`sqlite.go`)

**Schema:**
```sql
CREATE TABLE subscriptions (
    id        INTEGER PRIMARY KEY,
    user_id   TEXT NOT NULL,
    guild_id  TEXT NOT NULL DEFAULT '',
    item_id   TEXT NOT NULL,
    shop_type TEXT NOT NULL,
    UNIQUE(user_id, item_id)
);

CREATE TABLE board_messages (
    guild_id   TEXT NOT NULL,
    channel_id TEXT NOT NULL,
    shop_type  TEXT NOT NULL,
    message_id TEXT NOT NULL,
    PRIMARY KEY(guild_id, shop_type)
);
```

**Key Operations:**
- `Subscribe(user, guild, item, shop)` - Add subscription
- `Unsubscribe(user, item)` - Remove subscription
- `UnsubscribeAll(user)` - Clear all subscriptions
- `GetUserSubscriptions(user)` - List user's subs
- `GetSubscribersForItem(item)` - Find item watchers
- `GetBoardConfig(guild)` / `GetAllBoardConfigs()` - Board management

## Data Flow

### Restock Notification Flow

```
1. WebSocket receives PartialState patch
2. ShopState.ApplyPatches() detects 0→N stock change
3. Client calls onRestock callback
4. notify.Engine.HandleRestocks() receives changes
5. For each changed item:
   - store.GetSubscribersForItem(itemID)
   - Group subscribers by user
6. For each user:
   - bot.SendStockAlert(userID, batchedChanges)
   - Discord DM with embed + unsubscribe buttons
7. Subscribers receive DM notification
```

### Stock Board Update Flow

```
1. WebSocket receives patch (any change)
2. ShopState.ApplyPatches() processes patch
3. Client calls onStockChange callback
4. bot.Board().UpdateAllBoards() triggered
5. For each configured guild:
   - Load board config from store
   - Rebuild embed with current inventory
   - Edit channel message
```

## Concurrency Model

- **WebSocket reader**: Separate goroutine, reads messages
- **Heartbeat**: Separate goroutine, sends pings
- **ShopState**: RWMutex for thread-safe access
- **Board updates**: Periodic ticker (every minute for timestamps)
- **Discord session**: Handles events in discordgo's goroutine

## Error Handling

- **WebSocket disconnect**: Auto-reconnect with backoff
- **Discord API errors**: Logged, minimal impact
- **Database errors**: Logged, operation fails gracefully
- **Interaction acknowledgment**: Deferred response as fallback