# Magic Guardian - API Contracts

This document describes the Discord bot's slash command API, interaction patterns, and the Web UI REST API.

## Slash Commands

All commands are registered via Application Command Bulk Overwrite on bot startup.

### `/subscribe`

Get notified when an item is in stock.

**Options:**
| Name | Type | Required | Autocomplete |
|------|------|----------|--------------|
| item | String | Yes | Yes |

**Behavior:**
1. Resolves item name to canonical item ID
2. Creates subscription in `subscriptions` table
3. Returns confirmation embed

**Response:** Ephemeral success/failure message

**Database Operation:**
```sql
INSERT OR IGNORE INTO subscriptions (user_id, guild_id, item_id, shop_type)
VALUES (?, ?, ?, ?)
```

**Example Usage:** `/subscribe MythicalEgg`

---

### `/unsubscribe`

Stop notifications for a specific item.

**Options:**
| Name | Type | Required | Autocomplete |
|------|------|----------|--------------|
| item | String | Yes | Yes |

**Behavior:**
1. Resolves item name to item ID
2. Removes subscription from database
3. Returns confirmation message

**Response:** Ephemeral success/failure message

**Database Operation:**
```sql
DELETE FROM subscriptions WHERE user_id = ? AND item_id = ?
```

**Example Usage:** `/unsubscribe Bamboo`

---

### `/watchlist`

Show your current subscriptions with stock status.

**Options:** None

**Behavior:**
1. Queries all subscriptions for user
2. Enriches with current stock data from ShopState
3. Builds watchlist embed

**Response:** Rich embed with subscription list

**Database Query:**
```sql
SELECT id, user_id, guild_id, item_id, shop_type
FROM subscriptions WHERE user_id = ?
ORDER BY shop_type, item_id
```

**Embed Fields:**
- Per-item with emoji, name, and stock status
- Summary footer with total items watched

---

### `/stock`

Show current shop inventory.

**Options:**
| Name | Type | Required | Choices |
|------|------|----------|---------|
| shop | String | No | seed, tool, egg, decor |

**Behavior:**

**Without shop argument:**
- Shows overview of all 4 shops
- Embed fields: each shop with in-stock count and restock timer

**With shop argument:**
- Shows detailed inventory for that shop
- Split into "In Stock" and "Out of Stock" fields
- Includes restock countdown

**Response:** Rich embed with inventory data

**Shop Types:**
- `🌱 seed` - Seed shop
- `🔧 tool` - Tool shop
- `🥚 egg` - Egg shop
- `🎨 decor` - Decor shop

**Example Usage:** `/stock egg`

---

### `/restock`

Show time until next restock for each shop.

**Options:** None

**Behavior:**
1. Gets all shops from ShopState
2. Calculates remaining time from `SecondsUntilRestock`
3. Builds embed with formatted countdown

**Response:** Rich embed with timer display

**Embed Format:**
```
⏱️ Restock Timers
🌱 Seed — X minutes Y seconds
🔧 Tool — X minutes Y seconds
🥚 Egg — X minutes Y seconds
🎨 Decor — X minutes Y seconds
```

---

### `/setup-stock-board`

Create a live stock board channel with subscribe menus.

**Options:**
| Name | Type | Required | Default |
|------|------|----------|---------|
| name | String | No | "📦 Magic Garden Stock" |

**Behavior:**
1. Validates command is used in guild (not DM)
2. Creates category channel
3. Creates 4 text channels (one per shop type)
4. Posts embedded inventory in each channel
5. Stores board configuration in database

**Permissions:**
- Bot needs: Send Messages, Manage Channels, Embed Links
- Members can read channels but not send messages

**Database Operations:**
```sql
INSERT OR REPLACE INTO board_messages
(guild_id, channel_id, shop_type, message_id)
VALUES (?, ?, ?, ?)
```

**Response:** Ephemeral confirmation with channel link

**Board Structure:**
```
📦 Magic Garden Stock (category)
├── 🌱・seed-shop
├── 🔧・tool-shop
├── 🥚・egg-shop
└── 🎨・decor-shop
```

---

### `/delete-stock-board`

Remove the stock board channels and category from the server.

**Options:** None

**Behavior:**
1. Validates command is used in guild (not DM)
2. Retrieves board configuration from database
3. Deletes all shop channels
4. Deletes the category channel
5. Removes board configuration from database
6. Clears in-memory board tracking

**Permissions:**
- Bot needs: Manage Channels

**Database Operations:**
```sql
DELETE FROM board_messages WHERE guild_id = ?
```

**Response:** Ephemeral confirmation with count of deleted channels

---

## Autocomplete

Items for `/subscribe` and `/unsubscribe` support autocomplete.

**Handler:** `handleAutocomplete()`

**Logic:**
1. Uses static item catalog (`internal/mg/items.go`) for all subscribable items
2. For `/subscribe`: shows all items, marks already-subscribed items with "(subscribed)"
3. For `/unsubscribe`: filters to only show items the user is subscribed to
4. When no query: shows balanced mix from each shop type (seeds, tools, eggs, decor)
5. Filter items matching query string
6. Limit to 25 choices (Discord limit)

**Response Format:**
```json
{
  "name": "🥚 Mythical Egg",
  "value": "mythicalegg"
}
```

**Labels include:**
- Shop emoji prefix
- Item display name
- "(subscribed)" suffix for already-subscribed items in `/subscribe`
- Exclusivity badge for server/platform-exclusive items

---

## Message Components

### DM Unsubscribe Buttons

Sent with stock alert DMs.

**Component Type:** Button (ActionsRow)

**Custom IDs:**
- `dm_unsub_{itemID}` - Unsubscribe from specific item
- `dm_unsub_all` - Unsubscribe from all items

**Response:** Ephemeral confirmation message

---

### Board Update Button

On stock board embeds.

**Component Type:** Button (ActionsRow)

**Custom ID:** `board_update_{shopType}`

**Behavior:**
- Opens ephemeral dropdown with all items in shop
- Shows subscription status per item
- Allows toggling multiple subscriptions

---

### Board Select Menu

Subscription management on stock boards.

**Component Type:** Select Menu (ActionsRow)

**Custom ID Patterns:**
- `board_sub_{shopType}` - Initial subscribe dropdown
- `board_sub_{shopType}_{page}` - Paginated dropdowns
- `update_sub_{shopType}` - Update subscriptions dropdown
- `update_sub_{shopType}_{page}` - Paginated update dropdown

**Discord Limits:**
- Max 25 options per dropdown
- Max 5 pages (125 items max)

**Behavior:**
- Toggle subscription for selected items
- Show success/failure message

---

## Event Callbacks

### Restock Callback (`OnRestock`)

Fires when a shop restocks (timer resets) for ALL in-stock items in that shop.

**Trigger:** `client.handlePartialState()` detects timer reset (new value > old value + 10s)

**Receiver:** `notify.Engine.HandleRestocks()`

**Batch Behavior:**
- All in-stock items from the restocking shop are included
- Multiple shops restocking → single DM per user
- User receives list of all subscribed items currently in stock

---

### Stock Change Callback (`OnStockChange`)

Fires on ANY stock change.

**Trigger:** `client.handlePartialState()` detects `OldStock != NewStock`

**Receiver:** `bot.Board().UpdateAllBoards()`

**Behavior:**
- Edits all board embeds with current state
- Updates timestamp footer

---

### Connect Callback (`OnConnect`)

Fires after successful WebSocket reconnect.

**Trigger:** `client.handleWelcome()` receives full state

**Receiver:** `bot.Board().UpdateAllBoards()`

**Purpose:**
- Ensures boards show fresh state after reconnect
- Handles initial state sync

---

## API Response Patterns

### Ephemeral Responses

All slash command responses use `MessageFlagsEphemeral`:
- Only visible to the invoking user
- Doesn't clutter channel
- Standard for subscription management

### Rich Embeds

All data responses use `discordgo.MessageEmbed`:

| Response Type | Color | Content |
|---------------|-------|---------|
| Stock Alert | Green (0x2ecc71) | Grouped by shop, items × quantity |
| Shop Inventory | Blue (0x3498db) | In Stock / Out of Stock sections |
| Watchlist | Purple (0x9b59b6) | List with status, count footer |
| Restock Timer | Gold (0xf1c40f) | Per-shop countdowns |

### Error Handling

Interaction acknowledgment fallback:
```go
// If handler doesn't respond in time
s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
    Type: discordgo.InteractionResponseChannelMessageWithSource,
    Data: &discordgo.InteractionResponseData{
        Content: "Something went wrong. Please try again.",
        Flags:   discordgo.MessageFlagsEphemeral,
    },
})
```

---

## Message Flow Examples

### Subscribe Flow
```
User: /subscribe Bamboo
  ↓
bot.handleCommand(cmdSubscribe)
  ↓
store.Subscribe(userID, "", "bamboo", "seed")
  ↓
bot.respond() with confirmation
```

### Stock Alert Flow
```
WebSocket: PartialState patch (Bamboo: 0 → 5)
  ↓
client.handlePartialState()
  ↓
ShopState.ApplyPatches()
  ↓
onRestock(BambooStockChange)
  ↓
store.GetSubscribersForItem("bamboo")
  ↓
For each subscriber:
  bot.SendStockAlert(userID, [BambooStockChange])
    ↓
    session.UserChannelCreate(userID)
    ↓
    ChannelMessageSendComplex(DM, embed + buttons)
```

### Board Update Flow
```
WebSocket: PartialState patch (any stock change)
  ↓
onStockChange(changes)
  ↓
board.UpdateAllBoards()
  ↓
store.GetAllBoardConfigs()
  ↓
For each guild:
  For each shop:
    ChannelMessageEditComplex(embed + button)
```

---

## Web UI REST API

The web UI server (started with `-ui` flag) exposes a REST API at the configured listen address (default `127.0.0.1:8090`).

### `GET /api/status`

Returns current bot status.

**Response:**
```json
{
  "running": true,
  "status": "running",
  "uptime": "2h 15m",
  "room": "8DML",
  "version": "117",
  "shopCount": 4
}
```

### `GET /api/config`

Returns saved configuration (token is masked).

**Response:**
```json
{
  "discord_token": "MTQ4...0543",
  "app_id": "1482540440413540543",
  "start_on_boot": "true"
}
```

### `POST /api/config`

Save Discord credentials.

**Request:**
```json
{
  "discord_token": "full-token-here",
  "app_id": "1482540440413540543"
}
```

**Response:** `{"status": "saved"}`

### `POST /api/bot/start`

Start the bot engine using saved credentials.

**Response:** `{"status": "started"}`

**Error (409):** `{"error": "bot is already running"}`

### `POST /api/bot/stop`

Stop the bot engine.

**Response:** `{"status": "stopped"}`

### `POST /api/config/boot`

Toggle start-on-boot setting.

**Request:** `{"enabled": true}`

**Response:** `{"status": "saved"}`

### `GET /api/guilds`

List Discord servers the bot is in (only when running).

**Response:**
```json
[
  {"id": "123456789", "name": "My Server", "icon": "https://..."}
]
```

### `POST /api/guilds/leave`

Remove bot from a Discord server.

**Request:** `{"guild_id": "123456789"}`

**Response:** `{"status": "left"}`

### `GET /api/logs`

Server-Sent Events (SSE) stream of log lines.

**Content-Type:** `text/event-stream`

**Event format:**
```
data: time=2026-03-15T09:14:50.731Z level=INFO msg="web UI server starting" addr=127.0.0.1:8090

data: time=2026-03-15T09:14:50.856Z level=INFO msg="magic-guardian web UI ready" url=http://127.0.0.1:8090
```

Sends buffered history on connect, then streams new lines in real time.