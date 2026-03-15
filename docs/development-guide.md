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

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Running with Hot Reload

Use `fresh` for auto-rebuilding:

```bash
go install github.com/pilu/fresh@latest
fresh
```

Or use `air`:

```bash
go install github.com/cosmtrek/air@latest
air
```

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

## Testing New Features

### Manual Testing

1. Invite bot to test server
2. Use the new command
3. Check logs for errors
4. Verify expected behavior

### Adding Unit Tests

Create test file alongside implementation:

```go
// internal/mg/shop_test.go
package mg

import "testing"

func TestFormatItemName(t *testing.T) {
    tests := []struct {
        input    string
        expected string
    }{
        {"Bamboo", "Bamboo Seed"},
        {"MythicalEgg", "Mythical Egg"},
    }
    
    for _, tt := range tests {
        result := FormatItemName(tt.input)
        if result != tt.expected {
            t.Errorf("FormatItemName(%q) = %q, want %q", tt.input, result, tt.expected)
        }
    }
}
```

Run tests:

```bash
go test ./internal/mg/...
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