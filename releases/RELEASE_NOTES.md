# Magic Guardian v0.1.0 - Initial Release

A Discord bot that monitors Magic Garden game shop stock in real-time and notifies users when items are back in stock.

## Features

- **Stock Monitoring**: Tracks seed, tool, egg, and decor shops
- **Discord Notifications**: DM alerts when watched items restock
- **Interactive Commands**:
  - `/subscribe <item>` - Get notified when an item is in stock
  - `/unsubscribe <item>` - Stop notifications for an item
  - `/watchlist` - Show your current subscriptions
  - `/stock [shop]` - Show current shop inventory
  - `/restock` - Show time until next restock
  - `/setup-stock-board` - Create a live stock board channel

## Quick Start

### Prerequisites

- Go 1.25+
- Discord Bot Token
- Discord Application ID

### Setup

1. **Create a Discord Bot**:
   - Go to [Discord Developer Portal](https://discord.com/developers/applications)
   - Create a new application and bot
   - Copy the Bot Token and Application ID
   - Enable "Message Content Intent" in Bot settings

2. **Run the bot**:

```bash
# Linux/macOS
./magic-guardian-linux-amd64
./magic-guardian-darwin-arm64  # for Apple Silicon Macs

# Windows
magic-guardian-windows-amd64.exe
```

3. **Configure Environment Variables** (or use `.env` file):

```bash
DISCORD_TOKEN=your_bot_token_here
DISCORD_APP_ID=your_app_id_here
```

4. **Invite the Bot**:
   - Generate an invite URL with appropriate permissions
   - The bot needs: Send Messages, Manage Channels, Embed Links

## Downloads

| Platform | Architecture | File |
|----------|-------------|------|
| macOS | Intel (x86_64) | `magic-guardian-darwin-amd64` |
| macOS | Apple Silicon (arm64) | `magic-guardian-darwin-arm64` |
| Linux | amd64 | `magic-guardian-linux-amd64` |
| Linux | arm64 | `magic-guardian-linux-arm64` |
| Windows | amd64 | `magic-guardian-windows-amd64` |