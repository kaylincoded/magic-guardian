# Magic Guardian - Documentation Index

> [!IMPORTANT]
> This bot is designed to comply with Magic Garden's bot policy as a **read-only observer**.

**Last Updated:** March 2026

---

## Quick Reference

| | |
|---|---|
| **Entry Point** | `cmd/magic-guardian/main.go` |
| **Tech Stack** | Go 1.25, discordgo, gorilla/websocket, SQLite, Kotlin (Android) |
| **Architecture** | Multi-platform app: headless CLI, web UI, Android |
| **Build** | `make build` or `go build -o magic-guardian ./cmd/magic-guardian/` |
| **Test** | `make test-race` (112 tests with race detector) |
| **CI** | GitHub Actions: test + build on push/PR, release on tag |
| **Web UI** | `./magic-guardian -ui` (serves at `localhost:8090`) |
| **Android** | `make android-apk` (Kotlin wrapper + NDK cross-compiled Go binary) |

## Documentation

### Core Documentation

| Doc | Description |
|-----|-------------|
| [Project Overview](./project-overview.md) | Executive summary, features, tech stack, repo structure |
| [Architecture](./architecture.md) | Deployment modes, component diagrams, web UI API, Android layer |
| [Source Tree Analysis](./source-tree-analysis.md) | Directory structure, dependencies, build output |

### Technical Details

| Doc | Description |
|-----|-------------|
| [API Contracts](./api-contracts.md) | Slash commands, web UI REST API, message components |
| [Data Models](./data-models.md) | Database schema, Go types, config table |
| [Development Guide](./development-guide.md) | Setup, Android builds, coding conventions, deployment |

---

## Key Files

| File | Purpose |
|------|---------|
| `cmd/magic-guardian/main.go` | Entry point (headless + web UI modes) |
| `internal/discord/bot.go` | Bot session and commands |
| `internal/discord/embeds.go` | Rich embed builders |
| `internal/discord/board.go` | Stock board management |
| `internal/mg/client.go` | WebSocket client |
| `internal/mg/shop.go` | State management |
| `internal/mg/discover.go` | Version discovery |
| `internal/notify/engine.go` | Notification matching |
| `internal/store/sqlite.go` | SQLite persistence (subs, board, config) |
| `internal/webui/server.go` | HTTP server, REST API, SSE logs |
| `internal/webui/controller.go` | Bot lifecycle management |
| `internal/webui/static/index.html` | Web dashboard (go:embed) |
| `android/.../MainActivity.kt` | Android WebView shell |
| `android/.../GuardianService.kt` | Android foreground service |

---

## Getting Started

### For Users (Desktop)

1. Download binary from [releases](https://github.com/kaylincoded/magic-guardian/releases)
2. Configure `.env` with Discord credentials
3. Run: `./magic-guardian`
4. Add bot to server via OAuth2
5. Subscribe: `/subscribe <item-name>`

### For Users (Android)

1. Download APK from [releases](https://github.com/kaylincoded/magic-guardian/releases)
2. Install and open
3. Enter Bot Token and Application ID
4. Tap Start

### For Users (Web UI)

1. Run: `./magic-guardian -ui`
2. Open `http://localhost:8090`
3. Enter credentials and tap Save Configuration
4. Tap Start

### For Developers

1. Follow [Development Guide](./development-guide.md)
2. `make test-race` -- run 112 tests with race detector
3. `make build` -- build the binary
4. `make android-apk` -- build the Android APK
5. Push to `main` or open a PR -- CI runs automatically

---

## Resources

- [Magic Circle Discord](https://discord.com/invite/magiccircle)
- [Discord Developer Portal](https://discord.com/developers/applications)
- [discordgo Docs](https://pkg.go.dev/github.com/bwmarrin/discordgo)
- [gorilla/websocket](https://pkg.go.dev/github.com/gorilla/websocket)

---

*Last updated: March 2026*