# Magic Guardian - Architecture

## Overview

Service-oriented backend with event-driven architecture. The bot maintains a persistent WebSocket connection to Magic Garden, processes inventory changes, and dispatches notifications via Discord.

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
│  ┌──────────┴──────────┐                                              │
│  │   ShopState         │                                              │
│  │  (RWMutex protected)│                                              │
│  └──────────┬──────────┘                                              │
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
│• Alert │ │• Boards│ │                              │
└────────┘ └────┬───┘ └──────────────────────────────┘
                │
                ▼
         ┌──────────────┐
         │ Discord API  │
         └──────────────┘
```

## Components

### Entry Point (`cmd/magic-guardian/main.go`)

Wires all components together:

1. Loads `.env` config
2. Discovers MG version/room
3. Initializes SQLite store
4. Creates MG WebSocket client
5. Creates Discord bot
6. Creates notification engine
7. Wires event callbacks
8. Starts Discord + WebSocket clients
9. Waits for SIGINT/SIGTERM

```go
mgClient := mg.NewClient(cfg, logger)
bot, _ := discord.NewBot(token, appID, db, mgClient.State(), logger)
engine := notify.NewEngine(db, bot, logger)

mgClient.OnRestock(engine.HandleRestocks)
mgClient.OnStockChange(func(ch) { bot.Board().UpdateAllBoards() })
mgClient.OnConnect(func() { bot.Board().UpdateAllBoards() })
```

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
WebSocket → PartialState patch
    ↓
ShopState.ApplyPatches() detects 0→N change
    ↓
OnRestock callback → notify.Engine
    ↓
store.GetSubscribersForItem(itemID)
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