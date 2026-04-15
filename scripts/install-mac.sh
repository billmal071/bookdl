#!/bin/bash

# Quick install script for macOS
# Usage: curl -fsSL https://raw.githubusercontent.com/billmal071/bookdl/main/scripts/install-mac.sh | bash

set -e

echo "📥 Installing bookdl for macOS..."

# Detect architecture
ARCH=$(uname -m)
if [ "$ARCH" = "arm64" ]; then
    PLATFORM="darwin-arm64"
    echo "✓ Detected Apple Silicon (M1/M2/M3)"
else
    PLATFORM="darwin-amd64"
    echo "✓ Detected Intel Mac"
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
