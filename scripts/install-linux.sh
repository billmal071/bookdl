#!/bin/bash

# Quick install script for Linux
# Usage: curl -fsSL https://raw.githubusercontent.com/billmal071/bookdl/main/scripts/install-linux.sh | bash

set -e

echo "📥 Installing bookdl for Linux..."

# Detect architecture
ARCH=$(uname -m)
if [ "$ARCH" = "x86_64" ]; then
    PLATFORM="linux-amd64"
    echo "✓ Detected x86_64"
elif [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
    PLATFORM="linux-arm64"
    echo "✓ Detected ARM64"
else
    echo "❌ Unsupported architecture: $ARCH"
    exit 1
fi

# Get latest release
LATEST_URL="https://github.com/billmal071/bookdl/releases/latest/download/bookdl-${PLATFORM}"
DOWNLOAD_URL=$(curl -fsSL -w "%{url_effective}" "$LATEST_URL" 2>/dev/null || echo "$LATEST_URL")

# Download
echo "⬇️  Downloading..."
curl -fsSL "$DOWNLOAD_URL" -o /tmp/bookdl
chmod +x /tmp/bookdl

# Install
echo "📦 Installing to /usr/local/bin..."
if [ -w /usr/local/bin ]; then
    mv /tmp/bookdl /usr/local/bin/bookdl
else
    sudo mv /tmp/bookdl /usr/local/bin/bookdl
fi

# Verify
echo "✅ Verifying installation..."
bookdl --version

echo ""
echo "✨ Installation complete!"
echo ""
echo "Usage:"
echo "  bookdl search \"clean code\""
echo ""
echo "For more information: bookdl --help"
