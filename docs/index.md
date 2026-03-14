# Magic Guardian - Documentation Index

**Project:** A Discord bot for monitoring Magic Garden shop inventory (designed to comply with Magic Garden's bot policy)

**Last Updated:** March 2026

---

## Project Overview

- **Type:** Single-part monolith (backend)
- **Primary Language:** Go 1.25
- **Architecture:** Service with event-driven architecture
- **Repository:** [github.com/kaylin/magic-guardian](https://github.com/kaylin/magic-guardian)

## Quick Reference

| | |
|---|---|
| **Entry Point** | `cmd/magic-guardian/main.go` |
| **Tech Stack** | Go, discordgo, gorilla/websocket, SQLite |
| **Architecture Pattern** | Event-driven service with WebSocket client |
| **Build Command** | `go build -o magic-guardian ./cmd/magic-guardian/` |

## Documentation

### Core Documentation

- [Project Overview](./project-overview.md) - Executive summary and feature list
- [Architecture](./architecture.md) - High-level and component architecture diagrams
- [Source Tree Analysis](./source-tree-analysis.md) - Directory structure and critical paths

### Technical Details

- [API Contracts](./api-contracts.md) - Discord slash commands and message components
- [Data Models](./data-models.md) - Database schema and Go type definitions
- [Development Guide](./development-guide.md) - Setup, coding conventions, and deployment

---

## Documentation Map

```
┌─────────────────────────────────────────────────────────────────┐
│                      Documentation Index                        │
└─────────────────────────────────────────────────────────────────┘
        │
        ├── project-overview.md
        │   ├── Executive Summary
        │   ├── Tech Stack Table
        │   ├── Architecture Type
        │   └── How It Works
        │
        ├── architecture.md
        │   ├── High-Level Architecture Diagram
        │   ├── Component Architecture
        │   │   ├── Entry Point (cmd/)
        │   │   ├── WebSocket Layer (internal/mg/)
        │   │   ├── Discord Layer (internal/discord/)
        │   │   ├── Notification Engine (internal/notify/)
        │   │   └── Persistence Layer (internal/store/)
        │   ├── Data Flow
        │   └── Concurrency Model
        │
        ├── source-tree-analysis.md
        │   ├── Project Root Structure
        │   ├── Directory Purpose Analysis
        │   │   ├── cmd/magic-guardian/
        │   │   ├── internal/discord/
        │   │   ├── internal/mg/
        │   │   ├── internal/notify/
        │   │   └── internal/store/
        │   ├── Critical Paths
        │   └── File Dependencies
        │
        ├── api-contracts.md
        │   ├── Slash Commands
        │   │   ├── /subscribe
        │   │   ├── /unsubscribe
        │   │   ├── /watchlist
        │   │   ├── /stock
        │   │   ├── /restock
        │   │   └── /setup-stock-board
        │   ├── Autocomplete
        │   ├── Message Components
        │   └── Event Callbacks
        │
        ├── data-models.md
        │   ├── Database Schema
        │   │   ├── subscriptions table
        │   │   └── board_messages table
        │   ├── Go Type Definitions
        │   ├── Shop State Models
        │   └── Protocol Types
        │
        └── development-guide.md
            ├── Prerequisites
            ├── Development Setup
            ├── Project Structure
            ├── Development Workflow
            ├── Code Conventions
            ├── Adding New Commands
            └── Deployment
```

---

## Key Files Reference

### Source Files

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

## Technology Stack Details

### Runtime Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| discordgo | v0.29.0 | Discord API client |
| gorilla/websocket | v1.5.3 | WebSocket client |
| modernc.org/sqlite | v1.46.1 | SQLite driver |
| golang.org/x/text | v0.35.0 | Text processing |
| github.com/joho/godotenv | v1.5.1 | Config loading |

### Build Configuration

```
go 1.25
module github.com/kaylin/magic-guardian
```

---

## Architecture Summary

```
┌─────────────────────────────────────────────────────────────────────┐
│                    Magic Guardian Architecture                       │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌───────────────┐     WebSocket      ┌───────────────────────────┐ │
│  │   Magic       │◄────────────────────│      mg/Client            │ │
│  │   Garden      │                     │  ┌─────────────────────┐  │ │
│  │   Server      │                     │  │ • Connection mgmt   │  │ │
│  └───────────────┘                     │  │ • Heartbeat         │  │ │
│          ▲                             │  │ • Reconnect         │  │ │
│          │                             │  │ • State management  │  │ │
│          │                             │  └─────────────────────┘  │ │
│          │                                   │                      │ │
│          │                     ┌─────────────┼─────────────┐        │ │
│          │                     │             │             │        │ │
│          │                     ▼             ▼             ▼        │ │
│          │             ┌───────────┐ ┌───────────┐ ┌───────────┐    │ │
│          │             │  notify/  │ │ discord/  │ │  store/   │    │ │
│          └─────────────│  Engine   │ │   Bot     │ │ SQLite    │    │ │
│                        │           │ │           │ │           │    │ │
│                        │ • Match   │ │ • Commands│ │ • Subs    │    │ │
│                        │ • Batch   │ │ • DMs     │ │ • Boards  │    │ │
│                        │ • Alert   │ │ • Boards  │ │           │    │ │
│                        └─────┬─────┘ └─────┬─────┘ └───────┬─────┘    │ │
│                              │             │               │          │ │
│                              └─────────────┼───────────────┘          │ │
│                                            │                          │ │
│                                            ▼                          │ │
│                              ┌─────────────────────────────┐          │ │
│                              │      Discord API            │          │ │
│                              └─────────────────────────────┘          │ │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Getting Started

### For Users

1. **Add to Server:** Use OAuth2 invite URL
2. **Subscribe:** `/subscribe <item-name>`
3. **Check Stock:** `/stock` or `/stock seed`
4. **Setup Board:** `/setup-stock-board` (admin)

### For Developers

1. **Setup:** Follow [Development Guide](./development-guide.md)
2. **Build:** `go build ./cmd/magic-guardian/`
3. **Run:** `./magic-guardian`
4. **Test:** `go test ./...`

---

## Related Documentation

- **Magic Garden Modding Policy:** https://magicgarden.gg
- **Discord Developer Portal:** https://discord.com/developers/applications
- **discordgo Docs:** https://pkg.go.dev/github.com/bwmarrin/discordgo
- **gorilla/websocket:** https://pkg.go.dev/github.com/gorilla/websocket

---

*Last updated: March 14, 2026*