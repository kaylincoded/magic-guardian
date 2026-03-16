# magic-guardian

<div align="center">
  <img src="magic-guardian.png" alt="Magic Guardian Bot" width="400">
</div>

<br>

A Discord bot that monitors Magic Garden shop inventory and sends "in stock" notifications to subscribed users. Runs on Linux, macOS, Windows, and Android.

## Features

- **Real-time shop monitoring** — connects to the Magic Garden WebSocket and tracks all 4 shops (Seed, Tool, Egg, Decor)
- **Stock alerts via DM** — get notified instantly when items you're watching come back in stock
- **Slash commands** with autocomplete:
  - `/subscribe <item>` — watch an item for stock alerts
  - `/unsubscribe <item>` — stop watching an item
  - `/watchlist` — view your subscriptions with stock status
  - `/stock [shop]` — browse current shop inventory
  - `/restock` — see time until next restock
  - `/setup-stock-board` — create a live stock board for your server
- **Batched notifications** — one DM per restock event, not per item
- **SQLite persistence** — subscriptions survive restarts
- **Auto-reconnect** — recovers from WebSocket disconnects
- **Web UI** — browser-based dashboard for configuration and monitoring
- **Android app** — run the bot from your phone with a native APK

## Screenshots

| Watchlist | Stock Alert |
|-----------|-------------|
| ![Watchlist](./watchlist.png) | ![Stock Alert](./stockalert.png) |

## Running the Bot

The bot can run in two modes:

- **Headless mode** (default) — CLI-based, reads from `.env` file
- **Web UI mode** — Starts an embedded web server for browser-based setup and control

### Headless Mode (CLI)

```bash
# Configure credentials
echo "DISCORD_TOKEN=your_bot_token_here" > .env
echo "DISCORD_APP_ID=your_app_id_here" >> .env

# Run
./magic-guardian
```

### Web UI Mode

```bash
# Start with the -ui flag (listens on localhost:8090 by default)
./magic-guardian -ui

# Or with custom options
./magic-guardian -ui -listen "0.0.0.0:8080" -db "mydata.db" -auto-start
```

The web UI lets you configure the bot through your browser without needing a `.env` file. Access it at `http://localhost:8090` after starting.

---

## Getting Started

> [!IMPORTANT]
> You need valid credentials from the [Discord Developer Portal](https://discord.com/developers/home) before getting started:
> - `DISCORD_TOKEN` — Bot → Reset Token → Copy
> - `DISCORD_APP_ID` — General Information → Application ID

Choose how you want to run the bot:

---

### Option A: Download Binary (Recommended)

Pre-built binaries are available on the [releases page](https://github.com/kaylincoded/magic-guardian/releases).

| Platform | Download Command |
|----------|------------------|
| Linux amd64 | `wget https://github.com/kaylincoded/magic-guardian/releases/download/v0.2.1/magic-guardian-linux-amd64` |
| Linux arm64 | `wget https://github.com/kaylincoded/magic-guardian/releases/download/v0.2.1/magic-guardian-linux-arm64` |
| macOS Intel | `curl -O https://github.com/kaylincoded/magic-guardian/releases/download/v0.2.1/magic-guardian-darwin-amd64` |
| macOS Apple Silicon | `curl -O https://github.com/kaylincoded/magic-guardian/releases/download/v0.2.1/magic-guardian-darwin-arm64` |
| Windows | `curl -O https://github.com/kaylincoded/magic-guardian/releases/download/v0.2.1/magic-guardian-windows-amd64.exe` |
| Android (APK) | [Download from releases page](https://github.com/kaylincoded/magic-guardian/releases/download/v0.2.1/magic-guardian-android.apk) |

**Desktop (Linux/macOS/Windows):** configure and run:

```bash
# Make executable (Linux/macOS)
chmod +x magic-guardian-*

# Configure credentials
echo "DISCORD_TOKEN=your_bot_token_here" > .env
echo "DISCORD_APP_ID=your_app_id_here" >> .env

# Run
./magic-guardian-linux-amd64
```

**Android:** install the APK on your device, open the app, enter your credentials, and tap Start.

<div align="center">
  <img src="android-mg-demo.png" alt="Magic Guardian Android App" width="300">
</div>

The Android app runs the bot as a foreground service with a web-based dashboard. It keeps running in the background and can auto-start on boot. Requires Android 8.0+ (API 26).

> [!TIP]
> The web UI is available on **all platforms** (Linux, macOS, Windows). Use the `-ui` flag to start it:
>
> ```bash
> # Start with web UI (localhost:8090)
> ./magic-guardian -ui
>
> # With custom address, database path, and auto-start
> ./magic-guardian -ui -listen "0.0.0.0:8080" -db "mydata.db" -auto-start
> ```
>
> Without `-ui`, the bot runs in headless mode and reads credentials from a `.env` file or environment variables.

---

### Option B: Build from Source

```bash
# Clone and build
git clone https://github.com/kaylincoded/magic-guardian.git
cd magic-guardian
make build

# Run tests
make test-race

# Configure and run
echo "DISCORD_TOKEN=your_bot_token_here" > .env
echo "DISCORD_APP_ID=your_app_id_here" >> .env
./magic-guardian
```

#### Build the Android APK

Requires the [Android NDK](https://developer.android.com/ndk) and JDK 21:

```bash
make android-apk
# Output: android/app/build/outputs/apk/debug/app-debug.apk
```

See the [Development Guide](./docs/development-guide.md) for detailed setup and all Makefile targets.

---

### Invite the Bot

Use this URL template (replace `APP_ID`):

```
https://discord.com/oauth2/authorize?client_id=APP_ID&permissions=93200&scope=bot
```

Permissions needed: **Manage Channels, Manage Messages, Embed Links, View Channel, Read Message History** (integer `93200`)

## Policy Compliance

> [!IMPORTANT]
> This bot is designed to comply with Magic Garden's bot policy. It operates as a **read-only observer** — it connects anonymously, receives shop data, and does nothing else. No automation, no buying, no game actions.

> [!NOTE]
> **What does "policy-compliant" mean?**
>
> Magic Garden allows third-party tools that respect their boundaries. This bot makes a best-effort attempt to comply by:
>
> - Never sending game commands or automating player actions
> - Connecting as an anonymous observer (no authenticated player session)
> - Only reading shop inventory data — never interacting with game objects
> - Being transparent about what it does
>
> **Official policy:** [Magic Circle Discord](https://discord.com/invite/magiccircle) | [Bot & Tool Policy](https://ptb.discord.com/channels/808935495543160852/1428205518278885457/1428205518278885457) (Discord login required)

## Architecture

```
cmd/magic-guardian/main.go       Entry point (headless + web UI modes)
internal/mg/                     Magic Garden protocol (55 tests)
internal/notify/                 Notification engine (8 tests)
internal/discord/                Discord session, commands, stock boards
internal/store/                  SQLite persistence (21 tests)
internal/webui/                  Web UI server + REST API (29 tests)
android/                         Kotlin Android wrapper
.github/workflows/               CI (test + build) and release automation
Makefile                         Build, test, lint, android targets
```

## Documentation

| Doc | Description |
|-----|-------------|
| [📖 Architecture](./docs/architecture.md) | Component diagrams, data flow, concurrency model |
| [🔌 API Contracts](./docs/api-contracts.md) | Slash commands, message components, event callbacks |
| [🗃️ Data Models](./docs/data-models.md) | Database schema, Go types, protocol structures |
| [🛠️ Development Guide](./docs/development-guide.md) | Setup, coding conventions, testing, deployment |

Or browse the [full documentation index](./docs/index.md).

## How It Works

1. Connects to `wss://magicgarden.gg` as an anonymous player
2. Receives `Welcome` message with full shop inventory + restock timers
3. Monitors `PartialState` patches (JSON Patch RFC 6902) every ~1s
4. When `initialStock` changes from 0 → N, the item is "now in stock"
5. Matches against user subscriptions and sends batched Discord DMs

## License

MIT
