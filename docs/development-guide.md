# Magic Guardian - Development Guide

## Prerequisites

### Required Tools

- **Go 1.25+** - Language runtime and compiler
- **Git** - Version control
- **Discord Developer Account** - For bot credentials

### For Android Development

- **Android NDK r27+** - Cross-compilation toolchain for CGO
- **Android SDK** (platform-tools, build-tools, platform 35)
- **JDK 21** - Required by Android Gradle Plugin 8.7.3
- **Gradle 8.11+** - Build system (wrapper included in `android/`)

### Recommended Tools

- **GoLand** or **VS Code** - IDE with Go support
- **TablePlus** or **DB Browser for SQLite** - Database inspection
- **Android Studio** or **Android Emulator** - For Android testing

---

## Development Setup

### 1. Clone and Navigate

```bash
git clone https://github.com/kaylincoded/magic-guardian.git
cd magic-guardian
```

### 2. Install Dependencies

```bash
go mod download
go mod tidy
```

### 3. Configure Environment

Copy the example environment file:

```bash
cp .env.example .env
```

Edit `.env` with your credentials:

```bash
# Required
DISCORD_TOKEN=your-bot-token-here
DISCORD_APP_ID=your-app-id-here

# Optional (auto-discovered)
MG_ROOM_ID=8TP8
MG_VERSION=117
```

**Getting Discord Credentials:**

1. Go to [Discord Developer Portal](https://discord.com/developers/applications)
2. Create new application
3. Copy Application ID → `DISCORD_APP_ID`
4. Go to "Bot" section → "Reset Token" → Copy → `DISCORD_TOKEN`
5. Enable "Message Content Intent" in Bot settings

### 4. Build the Bot

```bash
# Development build
go build -o magic-guardian-dev ./cmd/magic-guardian/

# Production build
go build -o magic-guardian ./cmd/magic-guardian/
```

### 5. Run the Bot

```bash
./magic-guardian
```

Or with custom environment:

```bash
DISCORD_TOKEN=xxx ./magic-guardian
```

---

## Project Structure

```
magic-guardian/
├── cmd/
│   └── magic-guardian/
│       └── main.go              # Entry point (headless + web UI modes)
├── internal/
│   ├── discord/                 # Discord interface
│   │   ├── bot.go
│   │   ├── embeds.go
│   │   └── board.go
│   ├── mg/                      # Magic Garden protocol
│   │   ├── client.go
│   │   ├── messages.go
│   │   ├── shop.go
│   │   └── discover.go
│   ├── notify/                  # Notification engine
│   │   └── engine.go
│   ├── store/                   # Persistence
│   │   └── sqlite.go
│   └── webui/                   # Web UI server
│       ├── server.go
│       ├── controller.go
│       ├── loghandler.go
│       └── static/index.html
├── android/                     # Android wrapper (Kotlin)
├── docs/                        # Documentation
├── go.mod / go.sum
├── .env.example
└── README.md
```

---

## Development Workflow

### Using the Makefile

The project includes a Makefile for common tasks:

```bash
make              # Run tests + build
make test         # Run all tests
make test-race    # Run tests with race detector
make test-cover   # Run tests with coverage report
make test-verbose # Run tests with verbose output + race detector
make build        # Build the binary
make vet          # Run go vet
make lint         # Run golangci-lint (requires: brew install golangci-lint)
make android-build # Cross-compile Go binary for Android arm64
make android-apk   # Build the Android APK (requires NDK + JDK 21)
make release-binaries # Build macOS binaries
make clean        # Remove build artifacts
```

### Running Tests

The project has 112 tests across 5 packages covering the core engine, notification dispatch, SQLite persistence, and web UI HTTP handlers.

```bash
# Run all tests
make test

# Run with race detector (recommended)
make test-race

# Run with coverage
make test-cover
# Then open: go tool cover -html=coverage.out

# Run specific package
go test ./internal/mg/... -v

# Run specific test
go test ./internal/mg/... -run TestHandleWelcome -v
```

**Test coverage by package:**

| Package | Coverage | Tests | What's covered |
|---------|----------|-------|----------------|
| `internal/mg` | 51% | 55 | Restock detection, reconnect diff, message routing, state management, concurrency |
| `internal/notify` | 86% | 8 | Subscription matching, batching, error isolation, case-insensitivity |
| `internal/store` | 85% | 21 | CRUD operations, unique constraints, case handling, config upsert |
| `internal/webui` | 47% | 29 | All HTTP handlers, token masking, port conflict, SSE streaming, LogBuffer |
| `internal/discord` | 0% | 0 | Requires Discord API (integration test territory) |

### CI Pipeline

Every push to `main` and every PR triggers the CI pipeline (`.github/workflows/ci.yml`):

1. `go vet` -- static analysis
2. `go test -race` -- full test suite with race detector
3. Coverage report uploaded as artifact
4. Cross-compilation build matrix (5 platforms)

Releases are automated via `.github/workflows/release.yml`:
- Triggered by pushing a version tag (`git tag v0.3.0 && git push --tags`)
- Builds all desktop binaries + Android APK
- Creates GitHub Release with all artifacts attached

### Debug Mode

Enable debug logging via environment:

```bash
DEBUG=1 ./magic-guardian
```

Or modify log level in code:

```go
logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))
```

---

## Code Conventions

### Naming

- **Go packages:** Short, lowercase, no underscores (e.g., `discord`, `notify`)
- **Types:** PascalCase (e.g., `Bot`, `ShopState`)
- **Functions:** camelCase for unexported, PascalCase for exported
- **Variables:** camelCase for local, camelCase for package-level
- **Constants:** SCREAMING_SNAKE_CASE or PascalCase

### Error Handling

- Errors are returned, not logged-and-ignored
- Wrap errors with context: `fmt.Errorf("operation: %w", err)`
- Log errors with contextual fields: `logger.Error("failed", "error", err)`

```go
// Good
if err := doSomething(); err != nil {
    return fmt.Errorf("do something: %w", err)
}

// Good
logger.Error("operation failed", "error", err, "item", itemID)

// Bad - lost error
doSomething()

// Bad - no context
if err != nil {
    return err
}
```

### Concurrency

- Use `sync.RWMutex` for shared state
- Pass logger with component context
- Clean up goroutines with context cancellation

```go
// Component-scoped logger
logger := parentLogger.With("component", "board")

// RWMutex for ShopState
type ShopState struct {
    mu    sync.RWMutex
    shops map[string]*Shop
}
```

---

## Adding New Commands

### Step 1: Define Command

In `internal/discord/bot.go`, add to the `commands` slice in `Start()`:

```go
{
    Name:        "mycommand",
    Description: "Description of what it does",
    Options: []*discordgo.ApplicationCommandOption{
        {
            Type:        discordgo.ApplicationCommandOptionString,
            Name:        "optionname",
            Description: "What this option does",
            Required:    true,
        },
    },
},
```

### Step 2: Add Handler

In `internal/discord/bot.go`, extend `handleCommand()`:

```go
func (b *Bot) handleCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
    data := i.ApplicationCommandData()
    switch data.Name {
    case "subscribe":
        b.cmdSubscribe(s, i)
    // ... existing cases ...
    case "mycommand":
        b.cmdMyCommand(s, i)  // Add this
    }
}
```

### Step 3: Implement Handler

```go
func (b *Bot) cmdMyCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
    option := i.ApplicationCommandData().Options[0].StringValue()
    
    // Do work...
    
    // Respond
    b.respond(s, i, "Command completed!")
}
```

### Step 4: Register Commands

Commands register on bot start. Restart the bot to pick up changes:

```bash
# Stop bot (Ctrl+C)
# Rebuild
go build ./cmd/magic-guardian/
# Restart
./magic-guardian
```

---

## Testing

### Test Architecture

Tests live alongside their source files (`*_test.go`). The project uses Go's built-in testing framework with no external test libraries.

**Test files:**

| File | Tests | What it covers |
|------|-------|----------------|
| `internal/mg/client_test.go` | 29 | `diffShopState`, `handleWelcome`, `handlePartialState`, `handleMessage` routing, concurrent access |
| `internal/mg/shop_test.go` | 18 | `ApplyPatches`, timer handling, `FormatItemName`, `FormatStock`, `ShopItem.ItemID` |
| `internal/mg/messages_test.go` | 5 | JSON unmarshaling of protocol types |
| `internal/mg/discover_test.go` | 2 | Room ID and version regex extraction |
| `internal/notify/engine_test.go` | 8 | Subscription matching, batching (call count verification), error isolation |
| `internal/store/sqlite_test.go` | 21 | All CRUD operations, unique constraints, re-subscribe, board config, key-value config |
| `internal/webui/server_test.go` | 29 | All HTTP endpoints, token masking, port conflict, SSE streaming, LogBuffer |

### Testing Conventions

**Verify outcomes, not just counts.** Every assertion checks actual field values:

```go
// Good: verifies the full payload
if ch.ShopType != "seed" || ch.OldStock != 0 || ch.NewStock != 5 {
    t.Errorf("got shop=%q %d→%d, want seed 0→5", ch.ShopType, ch.OldStock, ch.NewStock)
}

// Bad: only checks count
if len(changes) != 1 {
    t.Error("wrong count")
}
```

**Test production code paths, not reimplementations.** Integration tests call `handleWelcome()`, `handlePartialState()`, and `handleMessage()` directly rather than reimplementing their logic.

**Use mock interfaces for external dependencies.** The `webui` tests use a `mockController` implementing `BotController`. The `notify` tests use a `mockSender` implementing `AlertSender` that tracks per-call payloads for batch verification.

### Adding New Tests

```bash
# Create test file alongside implementation
# internal/mypackage/mycode_test.go

# Run it
go test ./internal/mypackage/... -v -run TestMyNewFunction

# Run with race detector
go test ./internal/mypackage/... -race
```

---

## Database Management

### Inspecting the Database

```bash
# Using sqlite3 CLI
sqlite3 magic-guardian.db ".schema"
sqlite3 magic-guardian.db "SELECT * FROM subscriptions;"
sqlite3 magic-guardian.db "SELECT * FROM board_messages;"
```

### Migration Strategy

The code uses `CREATE TABLE IF NOT EXISTS`, so:
- Tables are created on first run
- New columns require migration code
- For schema changes, implement migration in `migrate()`

Example migration pattern:

```go
func migrate(db *sql.DB) error {
    // Existing tables
    _, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS subscriptions (...)
    `)
    if err != nil {
        return err
    }
    
    // Add new column if not exists
    _, err = db.Exec(`
        ALTER TABLE subscriptions ADD COLUMN new_column TEXT DEFAULT ''
    `)
    // Ignore error if column exists
    
    return nil
}
```

---

## Logging

The project uses structured logging with `log/slog`:

```go
// Create logger
logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))

// Log with levels
logger.Debug("detailed info", "key", value)
logger.Info("operation completed", "count", count)
logger.Warn("something unexpected", "value", value)
logger.Error("operation failed", "error", err)

// Component-scoped logger
componentLogger := logger.With("component", "bot")
```

---

## Common Tasks

### Adding a New Shop Type

1. Add emoji to `shopEmoji` map in `embeds.go`:
    ```go
    var shopEmoji = map[string]string{
        "seed":  "🌱",
        // Add new type
        "newshop": "🔮",
    }
    ```

2. Update `shopOrder` in `board.go`:
    ```go
    var shopOrder = []string{"seed", "tool", "egg", "decor", "newshop"}
    ```

3. Update `shopChannelNames` in `board.go`:
    ```go
    var shopChannelNames = map[string]string{
        // Add new shop
        "newshop": "🔮・newshop",
    }
    ```

4. Test that board setup creates correct channels.

### Modifying Embed Appearance

Edit functions in `internal/discord/embeds.go`:

```go
func BuildStockEmbed(shopType string, shop *mg.Shop) *discordgo.MessageEmbed {
    return &discordgo.MessageEmbed{
        Title:       fmt.Sprintf("%s %s Shop", emoji, shopType),
        Description: fmt.Sprintf("Next restock..."),
        Color:       colorBlue,  // Change color here
        // ... rest of fields
    }
}
```

### Changing Notification Behavior

Modify `internal/notify/engine.go`:

```go
func (e *Engine) HandleRestocks(changes []mg.StockChange) {
    // Change how notifications are batched or sent
    for userID, alerts := range userAlerts {
        // Modify alert delivery logic
    }
}
```

---

## Deployment

### Build for Production

```bash
# Linux amd64
GOOS=linux GOARCH=amd64 go build -o magic-guardian-linux ./cmd/magic-guardian/

# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o magic-guardian-darwin-arm64 ./cmd/magic-guardian/

# macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -o magic-guardian-darwin-amd64 ./cmd/magic-guardian/

# Windows
GOOS=windows GOARCH=amd64 go build -o magic-guardian-windows-amd64.exe ./cmd/magic-guardian/
```

### Build for Android

Requires the Android NDK:

```bash
export ANDROID_HOME=/path/to/android-sdk
export NDK=$ANDROID_HOME/ndk/27.2.12479018
export CC=$NDK/toolchains/llvm/prebuilt/darwin-x86_64/bin/aarch64-linux-android26-clang
export CXX=$NDK/toolchains/llvm/prebuilt/darwin-x86_64/bin/aarch64-linux-android26-clang++

# Cross-compile Go binary for Android arm64
CGO_ENABLED=1 GOOS=android GOARCH=arm64 CC=$CC CXX=$CXX \
  go build -ldflags="-s -w" \
  -o android/app/src/main/jniLibs/arm64-v8a/libguardian.so \
  ./cmd/magic-guardian/

# Build the APK (requires JDK 21)
export JAVA_HOME=/path/to/jdk-21
cd android && ./gradlew assembleDebug
# Output: android/app/build/outputs/apk/debug/app-debug.apk
```

**Important notes:**
- Use `GOOS=android` (not `linux`) to avoid `-lpthread` linker errors
- Do **not** use `-tags "netgo"` -- it breaks DNS resolution on Android (no `/etc/resolv.conf`)
- The binary must be a PIE executable (default), **not** `-buildmode=c-shared`
- The binary is named `libguardian.so` for Android packaging but is a standalone executable

### Running as Service (Linux)

Create systemd service at `/etc/systemd/system/magic-guardian.service`:

```ini
[Unit]
Description=Magic Guardian Discord Bot
After=network.target

[Service]
Type=simple
User=bot
WorkingDirectory=/path/to/magic-guardian
ExecStart=/path/to/magic-guardian/magic-guardian
Restart=always
RestartSec=10
Environment=DISCORD_TOKEN=xxx

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable magic-guardian
sudo systemctl start magic-guardian
```

### Running with Web UI

```bash
# Start with web UI on default port (127.0.0.1:8090)
./magic-guardian -ui

# Custom listen address
./magic-guardian -ui -listen "0.0.0.0:8080"

# Auto-start bot if saved credentials exist
./magic-guardian -ui -auto-start

# Custom database path
./magic-guardian -ui -db /data/magic-guardian.db
```

---

## Troubleshooting

### Bot Not Responding

1. Check logs for errors
2. Verify Discord token is valid
3. Check bot has correct permissions

```bash
# View logs
tail -f /var/log/magic-guardian.log  # If logging to file
```

### Commands Not Registering

1. Bot must be in server to register commands
2. Check `DISCORD_APP_ID` is correct
3. Restart bot after code changes

### Database Locked

Only one process can write to SQLite at a time. Ensure only one bot instance is running.

### WebSocket Disconnecting

- Check network connectivity
- Increase reconnect backoff in `client.go` if needed
- Verify magicgarden.gg is accessible