#!/bin/sh
set -e

# feeds installer
# Usage: curl -fsSL https://raw.githubusercontent.com/erik/feeds/main/install.sh | sh

REPO="erik/feeds"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="feeds"

# Detect OS and architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
    x86_64|amd64)
        ARCH="amd64"
        ;;
    arm64|aarch64)
        ARCH="arm64"
        ;;
    *)
        echo "Error: Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

case "$OS" in
    darwin|linux)
        ;;
    mingw*|msys*|cygwin*|windows*)
        OS="windows"
        ;;
    *)
        echo "Error: Unsupported OS: $OS"
        exit 1
        ;;
esac

# Build download filename
if [ "$OS" = "windows" ]; then
    FILENAME="${BINARY_NAME}-${OS}-${ARCH}.exe"
else
    FILENAME="${BINARY_NAME}-${OS}-${ARCH}"
fi

echo "Detected: $OS/$ARCH"

# Get latest release tag
echo "Fetching latest release..."
LATEST=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST" ]; then
    echo "Error: Could not determine latest release"
    exit 1
fi

echo "Latest version: $LATEST"

# Download URL
URL="https://github.com/${REPO}/releases/download/${LATEST}/${FILENAME}"

# Create temp directory
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

echo "Downloading ${FILENAME}..."
curl -fsSL "$URL" -o "$TMP_DIR/$BINARY_NAME"

# Make executable
chmod +x "$TMP_DIR/$BINARY_NAME"

# Install
echo "Installing to $INSTALL_DIR (may require sudo)..."
if [ -w "$INSTALL_DIR" ]; then
    mv "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
else
    sudo mv "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
fi

echo ""
echo "Successfully installed $BINARY_NAME to $INSTALL_DIR/$BINARY_NAME"
echo ""
echo "Run '$BINARY_NAME --help' to get started."
echo ""
echo "Note: yt-dlp is required for video downloads."
echo "      Install it via: brew install yt-dlp (macOS) or pip install yt-dlp"
