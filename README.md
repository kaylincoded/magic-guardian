# magic-guardian

A policy-compliant Discord bot that monitors Magic Garden shop inventory and sends "in stock" notifications to subscribed users.

## Features

- **Real-time shop monitoring** — connects to the Magic Garden WebSocket and tracks all 4 shops (Seed, Tool, Egg, Decor)
- **Stock alerts via DM** — get notified instantly when items you're watching come back in stock
- **Slash commands** with autocomplete:
  - `/subscribe <item>` — watch an item for stock alerts
  - `/unsubscribe <item>` — stop watching an item
  - `/watchlist` — view your subscriptions with current stock status
  - `/stock [shop]` — browse current shop inventory
  - `/restock` — see time until next restock for each shop
- **Batched notifications** — one DM per restock event, not per item
- **SQLite persistence** — subscriptions survive restarts
- **Auto-reconnect** — recovers from WebSocket disconnects

## Policy Compliance

This bot is **read-only**. It connects as an anonymous player, receives shop state updates, and does nothing else. No automation, no buying, no game actions. Fully compliant with the [Magic Garden Modding Policy](https://magicgarden.gg).

## Setup

### Prerequisites

- Go 1.21+
- A Discord bot token ([Discord Developer Portal](https://discord.com/developers/applications))

### Configuration

Copy `.env.example` to `.env` and fill in your values:

```bash
cp .env.example .env
```

| Variable | Description | Default |
|---|---|---|
| `DISCORD_TOKEN` | Bot token from Discord Developer Portal | *required* |
| `DISCORD_APP_ID` | Application ID from Discord Developer Portal | *required* |
| `MG_ROOM_ID` | Magic Garden room ID to monitor | `8TP8` |
| `MG_VERSION` | Game client version | `117` |

### Build & Run

```bash
go build -o magic-guardian ./cmd/magic-guardian/
./magic-guardian
```

### Invite the Bot

Use this URL template to invite the bot to your server (replace `APP_ID`):

```
https://discord.com/oauth2/authorize?client_id=APP_ID&scope=bot+applications.commands&permissions=2048
```

Permissions needed: **Send Messages** (2048)

## Architecture

```
cmd/magic-guardian/main.go     Entry point, wires all components
internal/mg/client.go          WebSocket client (connect, heartbeat, reconnect)
internal/mg/messages.go        Protocol types (Welcome, PartialState, Patch)
internal/mg/shop.go            Shop state management, patch application
internal/notify/engine.go      Matches stock events to subscriptions
internal/discord/bot.go        Discord session, slash commands, autocomplete
internal/discord/embeds.go     Rich embed builders for all responses
internal/store/sqlite.go       SQLite subscription persistence
```

## How It Works

1. Connects to `wss://magicgarden.gg` as an anonymous player
2. Receives `Welcome` message with full shop inventory + restock timers
3. Monitors `PartialState` patches (JSON Patch RFC 6902) every ~1s
4. When `initialStock` changes from 0 → N, the item is "now in stock"
5. Matches against user subscriptions and sends batched Discord DMs

## License

MIT
