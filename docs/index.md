# Magic Guardian - Documentation Index

> [!IMPORTANT]
> This bot is designed to comply with Magic Garden's bot policy as a **read-only observer**.

**Last Updated:** March 2026

---

## Quick Reference

| | |
|---|---|
| **Entry Point** | `cmd/magic-guardian/main.go` |
| **Tech Stack** | Go 1.25, discordgo, gorilla/websocket, SQLite |
| **Architecture** | Event-driven service with WebSocket client |
| **Build Command** | `go build -o magic-guardian ./cmd/magic-guardian/` |

## Documentation

### Core Documentation

| Doc | Description |
|-----|-------------|
| [Project Overview](./project-overview.md) | Executive summary, features, tech stack |
| [Architecture](./architecture.md) | Component diagrams, data flow |
| [Source Tree Analysis](./source-tree-analysis.md) | Directory structure, dependencies |

### Technical Details

| Doc | Description |
|-----|-------------|
| [API Contracts](./api-contracts.md) | Slash commands, message components |
| [Data Models](./data-models.md) | Database schema, Go types |
| [Development Guide](./development-guide.md) | Setup, coding conventions, deployment |

---

## Key Files

| File | Purpose | Lines |
|------|---------|-------|
| `cmd/magic-guardian/main.go` | Entry point and DI | 105 |
| `internal/discord/bot.go` | Bot session and commands | 568 |
| `internal/discord/embeds.go` | Rich embed builders | 236 |
| `internal/discord/board.go` | Stock board management | 692 |
| `internal/mg/client.go` | WebSocket client | 411 |
| `internal/mg/shop.go` | State management | 277 |
| `internal/mg/messages.go` | Protocol types | 88 |
| `internal/mg/discover.go` | Version discovery | 84 |
| `internal/notify/engine.go` | Notification matching | 56 |
| `internal/store/sqlite.go` | SQLite persistence | 221 |

**Total:** ~2,738 lines of Go code

---

## Getting Started

### For Users

1. Add bot to server via OAuth2
2. Subscribe: `/subscribe <item-name>`
3. Check stock: `/stock`
4. Setup board: `/setup-stock-board` (admin)

### For Developers

1. Follow [Development Guide](./development-guide.md)
2. Build: `go build ./cmd/magic-guardian/`
3. Run: `./magic-guardian`
4. Test: `go test ./...`

---

## Resources

- [Magic Circle Discord](https://discord.com/invite/magiccircle)
- [Discord Developer Portal](https://discord.com/developers/applications)
- [discordgo Docs](https://pkg.go.dev/github.com/bwmarrin/discordgo)
- [gorilla/websocket](https://pkg.go.dev/github.com/gorilla/websocket)

---

*Last updated: March 2026*