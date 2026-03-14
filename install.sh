#!/bin/bash
# install-magic-guardian.sh
# Download and install the latest magic-guardian release

set -e

REPO="kaylincoded/magic-guardian"
BINARY_NAME="magic-guardian"

# Detect OS and architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$OS" in
  linux*)  OS="Linux" ;;
  darwin*) OS="macOS" ;;
  mingw*|cygwin*|msys*) OS="Windows" ;;
  *)
    echo "Unsupported operating system: $OS"
    exit 1
    ;;
esac

case "$ARCH" in
  x86_64*)  ARCH="amd64" ;;
  aarch64*|arm64*) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

# For Windows, we need a different approach
if [ "$OS" = "Windows" ]; then
    ARCH="amd64"  # Simplified for now
fi

echo "Detected: $OS ($ARCH)"

# Get latest release version
LATEST=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | sed 's/.*": "\(.*\)"/\1/')

if [ -z "$LATEST" ]; then
    echo "Could not determine latest release version"
    exit 1
fi

echo "Latest version: $LATEST"

# Download URL
FILENAME="${BINARY_NAME}_${OS}_${ARCH}.zip"
URL="https://github.com/$REPO/releases/download/$LATEST/$FILENAME"

echo "Downloading: $URL"
curl -L -o "$FILENAME" "$URL"

echo "Extracting..."
unzip -o "$FILENAME"

# Cleanup
rm "$FILENAME"

# Make executable (Linux/macOS only)
if [ "$OS" != "Windows" ]; then
    chmod +x "$BINARY_NAME"
fi

echo ""
echo "✓ Installed ${BINARY_NAME} $LATEST for $OS ($ARCH)"
echo ""
echo "Next steps:"
echo "1. Copy .env.example to .env and add your Discord token/app ID"
echo "2. Invite the bot with required permissions:"
echo "   https://discord.com/oauth2/authorize?client_id=YOUR_APP_ID&scope=bot&permissions=93200"
echo "3. Run: ./$BINARY_NAME (or $BINARY_NAME.exe on Windows)"
echo ""
echo "Required permissions: Manage Channels, Manage Messages, Embed Links, View Channel, Read Message History"