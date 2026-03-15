# Magic Guardian - Data Models

## Database Overview

Magic Guardian uses SQLite for persistent storage with three tables:
- `subscriptions` - User watchlist data
- `board_messages` - Stock board configuration
- `config` - Key-value settings (web UI mode)

Database file: `magic-guardian.db` (created on first run)

## Schema

### Subscriptions Table

Stores user subscription records for stock alerts.

```sql
CREATE TABLE subscriptions (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id   TEXT NOT NULL,
    guild_id  TEXT NOT NULL DEFAULT '',
    item_id   TEXT NOT NULL,
    shop_type TEXT NOT NULL,
    UNIQUE(user_id, item_id)
);
```

**Indexes:**
```sql
CREATE INDEX IF NOT EXISTS idx_subscriptions_item ON subscriptions(item_id);
```

**Column Details:**

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | INTEGER | PRIMARY KEY AUTOINCREMENT | Unique subscription ID |
| user_id | TEXT | NOT NULL | Discord user ID (snowflake) |
| guild_id | TEXT | NOT NULL DEFAULT '' | Discord guild ID (for board subs) |
| item_id | TEXT | NOT NULL | Canonical item ID (lowercase) |
| shop_type | TEXT | NOT NULL | Shop category: seed/tool/egg/decor |

**Constraints:**
- `UNIQUE(user_id, item_id)` - User can only subscribe once per item
- Item IDs are stored lowercase for consistent lookup

**Relationships:**
- `user_id` → Discord user
- `item_id` → ShopItem in Magic Garden inventory
- `shop_type` → Shop in ShopState

---

### Board Messages Table

Stores stock board channel and message IDs per guild.

```sql
CREATE TABLE board_messages (
    guild_id   TEXT NOT NULL,
    channel_id TEXT NOT NULL,
    shop_type  TEXT NOT NULL,
    message_id TEXT NOT NULL,
    PRIMARY KEY(guild_id, shop_type)
);
```

**Column Details:**

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| guild_id | TEXT | NOT NULL, PK part | Discord guild/server ID |
| channel_id | TEXT | NOT NULL | Channel ID for shop display |
| shop_type | TEXT | NOT NULL, PK part | Shop category (seed/tool/egg/decor) |
| message_id | TEXT | NOT NULL | Message ID for live updates |

**Constraints:**
- `PRIMARY KEY(guild_id, shop_type)` - One board message per shop per guild

**Relationships:**
- `guild_id` → Discord server
- `channel_id` → Discord text channel
- `shop_type` → Shop in ShopState

---

### Config Table

Stores key-value settings for the web UI mode.

```sql
CREATE TABLE config (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL DEFAULT ''
);
```

**Known Keys:**

| Key | Description |
|-----|-------------|
| `discord_token` | Discord bot token |
| `app_id` | Discord application ID |
| `start_on_boot` | `"true"` or `"false"` -- auto-start bot on launch |

**Storage Operations:**

| Operation | Method | Returns |
|-----------|--------|---------|
| Get value | `GetConfig(key)` | `string, error` |
| Set value | `SetConfig(key, value)` | `error` |
| Get all | `GetAllConfig()` | `map[string]string, error` |

---

## Go Type Definitions

### Subscription Type

```go
type Subscription struct {
    ID       int64
    UserID   string
    GuildID  string
    ItemID   string
    ShopType string
}
```

**Field Mappings:**
- `ID` → `subscriptions.id`
- `UserID` → `subscriptions.user_id`
- `GuildID` → `subscriptions.guild_id`
- `ItemID` → `subscriptions.item_id`
- `ShopType` → `subscriptions.shop_type`

---

### BoardConfig Type

```go
type BoardConfig struct {
    GuildID  string
    Channels map[string]string  // shop_type → channel_id
    Messages map[string]string  // shop_type → message_id
}
```

**Maps:**
- `Channels["seed"]` → Seed shop channel ID
- `Channels["tool"]` → Tool shop channel ID
- `Channels["egg"]` → Egg shop channel ID
- `Channels["decor"]` → Decor shop channel ID
- `Messages["seed"]` → Seed shop message ID
- etc.

---

## Shop State Models

In-memory state maintained by `ShopState`.

### Shop Type

```go
type Shop struct {
    Inventory           []ShopItem
    SecondsUntilRestock float64
    RestockCycle        float64
}
```

**Fields:**
- `Inventory` - All items in the shop
- `SecondsUntilRestock` - Countdown to next restock (updated via patches)
- `RestockCycle` - Full cycle length (set after first timer reset detected)

---

### ShopItem Type

```go
type ShopItem struct {
    ItemType     string  // "Seed" | "Tool" | "Egg" | "Decor"
    Species      string  // For seeds (e.g., "Bamboo")
    ToolID       string  // For tools (e.g., "Shovel")
    EggID        string  // For eggs (e.g., "MythicalEgg")
    DecorID      string  // For decor (e.g., "MiniFairyCottage")
    InitialStock int     // Current stock quantity
}
```

**ItemID() Method:**
```go
func (si ShopItem) ItemID() string {
    switch si.ItemType {
    case "Seed":   return si.Species
    case "Tool":   return si.ToolID
    case "Egg":    return si.EggID
    case "Decor":  return si.DecorID
    default:       return ""
    }
}
```

---

### StockChange Type

```go
type StockChange struct {
    ShopType string
    Item     ShopItem
    OldStock int
    NewStock int
}
```

**Fields:**
- `ShopType` - Which shop (seed/tool/egg/decor)
- `Item` - Item that changed
- `OldStock` - Previous quantity
- `NewStock` - New quantity

**Detection Logic:**
```go
// Restock event: 0 → N
if ch.OldStock == 0 && ch.NewStock > 0 {
    // Trigger notifications
}

// Any change
if ch.OldStock != ch.NewStock {
    // Trigger board updates
}
```

---

## Item Registry

Maps game item IDs to display names (`shop.go`).

### Seeds (~80 items)

| Item ID | Display Name |
|---------|-------------|
| Bamboo | Bamboo Seed |
| Carrot | Carrot Seed |
| Tomato | Tomato Seed |
| ... | ... |

**Full List in Code:**
```go
var itemDisplayNames = map[string]string{
    "Carrot":        "Carrot Seed",
    "Bamboo":        "Bamboo Seed",
    "MythicalEgg":   "Mythical Egg",
    // ... 80+ items
}
```

---

### Tools (~13 items)

| Item ID | Display Name |
|---------|-------------|
| Shovel | Garden Shovel |
| WateringCan | Watering Can |
| PlanterPot | Planter Pot |
| ... | ... |

---

### Eggs (~8 items)

| Item ID | Display Name |
|---------|-------------|
| CommonEgg | Common Egg |
| UncommonEgg | Uncommon Egg |
| RareEgg | Rare Egg |
| LegendaryEgg | Legendary Egg |
| MythicalEgg | Mythical Egg |
| ... | ... |

---

### Decor (~50 items)

| Item ID | Display Name |
|---------|-------------|
| SmallRock | Small Garden Rock |
| MiniFairyCottage | Mini Fairy Cottage |
| StoneGnome | Stone Gnome |
| ... | ... |

---

## Shop Categories

Four shop types with emoji identifiers:

```go
var shopEmoji = map[string]string{
    "seed":  "🌱",
    "tool":  "🔧",
    "egg":   "🥚",
    "decor": "🎨",
}
```

**Shop Channel Names (Stock Board):**
```go
var shopChannelNames = map[string]string{
    "seed":  "🌱・seed-shop",
    "tool":  "🔧・tool-shop",
    "egg":   "🥚・egg-shop",
    "decor": "🎨・decor-shop",
}
```

---

## Protocol Types

### ServerMessage (WebSocket)

```go
type ServerMessage struct {
    Type      string          `json:"type"`
    FullState json.RawMessage `json:"fullState,omitempty"`
    Patches   []Patch         `json:"patches,omitempty"`
    Config    json.RawMessage `json:"config,omitempty"`
}
```

**Message Types:**
- `Welcome` - Full state on connect
- `PartialState` - Incremental updates (JSON Patch)
- `Config` - Configuration data

---

### Patch (RFC 6902)

```go
type Patch struct {
    Op    string      `json:"op"`
    Path  string      `json:"path"`
    Value json.Number `json:"value"`
}
```

**Patch Operations:**
- `replace` - Update existing value (only op used)

**Path Formats:**
- Inventory: `/child/data/shops/{shop}/inventory/{index}/initialStock`
- Timer: `/child/data/shops/{shop}/secondsUntilRestock`

---

## WelcomeState Structure

```go
type WelcomeState struct {
    Scope string
    Data  struct {
        RoomID string
    }
    Child ChildState
}

type ChildState struct {
    Scope string
    Data  QuinoData
}

type QuinoData struct {
    CurrentTime float64
    Shops       map[string]*Shop
}
```

---

## Configuration Types

### ClientConfig

```go
type ClientConfig struct {
    RoomID  string
    Version string
}
```

**Environment Variables:**
```go
DISCORD_TOKEN    // Bot token from Discord Developer Portal
DISCORD_APP_ID   // Application ID from Discord Developer Portal
MG_ROOM_ID       // Magic Garden room ID (default: "8TP8")
MG_VERSION       // Game client version (auto-discovered)
```

---

## Storage Operations

### Subscription Operations

| Operation | Method | Returns |
|-----------|--------|---------|
| Add subscription | `Subscribe(userID, guildID, itemID, shopType)` | `bool` (created vs existing) |
| Remove subscription | `Unsubscribe(userID, itemID)` | `bool` (found vs not found) |
| Remove all | `UnsubscribeAll(userID)` | `int64` (count removed) |
| List user subs | `GetUserSubscriptions(userID)` | `[]Subscription` |
| Find subscribers | `GetSubscribersForItem(itemID)` | `[]Subscription` |

---

### Board Operations

| Operation | Method | Returns |
|-----------|--------|---------|
| Store message | `SetBoardMessage(guildID, channelID, shopType, messageID)` | `error` |
| Load config | `GetBoardConfig(guildID)` | `*BoardConfig` |
| Load all | `GetAllBoardConfigs()` | `[]BoardConfig` |
| Delete config | `DeleteBoardConfig(guildID)` | `error` |

---

## Data Flow

### Subscription Creation
```
1. User runs /subscribe <item>
2. bot.resolveItem() → itemID, shopType
3. store.Subscribe(userID, guildID, itemID, shopType)
4. INSERT OR IGNORE INTO subscriptions
5. Return confirmation
```

### Notification Delivery
```
1. WebSocket patch: Item X 0 → 5
2. notify.Engine.HandleRestocks([change])
3. store.GetSubscribersForItem("itemx")
4. SELECT * FROM subscriptions WHERE item_id = 'itemx'
5. For each subscriber:
   bot.SendStockAlert(userID, [change])
```

### Board Update
```
1. Stock change detected
2. board.UpdateAllBoards()
3. store.GetAllBoardConfigs()
4. SELECT * FROM board_messages
5. For each guild/shop:
   ChannelMessageEditComplex(newEmbed)
```