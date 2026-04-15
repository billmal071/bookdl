# Installation Guide

This guide covers all installation methods for bookdl.

## Table of Contents

- [Quick Install](#quick-install)
- [Download Binaries](#download-binaries)
- [Package Managers](#package-managers)
  - [Homebrew (macOS/Linux)](#homebrew-macoslinux)
- [From Source](#from-source)
- [Post-Installation](#post-installation)
- [Troubleshooting](#troubleshooting)

## Quick Install

### macOS

```bash
curl -fsSL https://raw.githubusercontent.com/billmal071/bookdl/main/scripts/install-mac.sh | bash
```

This script will:
- Detect your Mac architecture (Intel or Apple Silicon)
- Download the latest release
- Install to `/usr/local/bin/bookdl`
- Verify the installation

### Linux

```bash
curl -fsSL https://raw.githubusercontent.com/billmal071/bookdl/main/scripts/install-linux.sh | bash
```

This script will:
- Detect your Linux architecture (x86_64 or ARM64)
- Download the latest release
- Install to `/usr/local/bin/bookdl`
- Verify the installation

### Windows (PowerShell)

Run in PowerShell as Administrator:

```powershell
iwr -useb https://raw.githubusercontent.com/billmal071/bookdl/main/scripts/install-windows.ps1 | iex
```

This script will:
- Download the latest release
- Install to `%USERPROFILE%\AppData\Local\bookdl`
- Add the directory to your PATH
- Verify the installation

## Download Binaries

Download pre-built binaries from the [Releases page](https://github.com/billmal071/bookdl/releases/latest).

### Available Platforms

| Platform | Architecture | Binary Name |
|----------|--------------|-------------|
| macOS | Intel (x86_64) | `bookdl-darwin-amd64` |
| macOS | Apple Silicon (M1/M2/M3) | `bookdl-darwin-arm64` |
| Linux | x86_64 | `bookdl-linux-amd64` |
| Linux | ARM64 | `bookdl-linux-arm64` |
| Windows | x86_64 | `bookdl-windows-amd64.exe` |

### Manual Installation

1. Download the binary for your platform
2. Make it executable (macOS/Linux only):
   ```bash
   chmod +x bookdl-*
   ```
3. Move to a directory in your PATH:
   ```bash
   # macOS/Linux
   sudo mv bookdl-* /usr/local/bin/bookdl

   # Windows - move to a directory in PATH or add directory to PATH
   ```

## Package Managers

### Homebrew (macOS/Linux)

```bash
brew tap billmal071/bookdl
brew install bookdl
```

Or install directly from the formula:

```bash
brew install https://raw.githubusercontent.com/billmal071/bookdl/main/homebrew/bookdl.rb
```

## From Source

### Prerequisites

- Go 1.21 or later
- Git
- Make (optional)

### Steps

```bash
# Clone the repository
git clone https://github.com/billmal071/bookdl.git
cd bookdl

# Install dependencies
make deps

# Build
make build

# Install to GOPATH/bin
make install
```

Or using Go directly:

```bash
go install github.com/billmal071/bookdl/cmd/bookdl@latest
```

### Building for Multiple Platforms

```bash
# Build for all platforms
make build-all

# Platform-specific builds
make build-linux    # Linux x86_64
make build-darwin   # macOS (both architectures)
make build-windows  # Windows x86_64
```

## Post-Installation

### Verify Installation

```bash
bookdl --version
```

### Enable Shell Completions

#### Bash

```bash
# Add to ~/.bashrc
bookdl completion bash > /etc/bash_completion.d/bookdl
source ~/.bashrc
```

#### Zsh

```bash
# Add to ~/.zshrc
bookdl completion zsh > "${fpath[1]}/_bookdl"
source ~/.zshrc
```

#### Fish

```bash
bookdl completion fish > ~/.config/fish/completions/bookdl.fish
```

#### PowerShell

```powershell
bookdl completion powershell | Out-File -Encoding utf8 (New-Item -Type Directory -Force $PROFILE.Substring(0, $PROFILE.LastIndexOf('\')))
```

### Optional Dependencies

- **Chrome/Chromium**: Required for Cloudflare bypass and Z-Library searches. Install via:
  - macOS: `brew install --cask google-chrome`
  - Linux: `sudo apt install chromium-browser` or download from https://www.chromium.org/
  - Windows: Download from https://www.google.com/chrome/

## Troubleshooting

### Permission Denied

If you get "permission denied" errors:

```bash
# Make the binary executable
chmod +x $(which bookdl)

# Or reinstall with sudo
curl -fsSL https://raw.githubusercontent.com/billmal071/bookdl/main/scripts/install-linux.sh | sudo bash
```

### Command Not Found

If `bookdl` command is not found:

1. Check if it's in your PATH:
   ```bash
   which bookdl
   # or
   where bookdl  # Windows
   ```

2. Add the install directory to PATH (add to your shell config):
   ```bash
   # Bash/Zsh - add to ~/.bashrc or ~/.zshrc
   export PATH="$PATH:/usr/local/bin"

   # PowerShell - add to $PROFILE
   $env:PATH += ";$env:USERPROFILE\AppData\Local\bookdl"
   ```

3. Restart your terminal or reload shell config:
   ```bash
   source ~/.bashrc  # or ~/.zshrc
   ```

### macOS: "Cannot be opened because it is from an unidentified developer"

```bash
# Allow the binary to run
xattr -cr $(which bookdl)
```

Or right-click the binary and select "Open" from the context menu.

### Windows: SmartScreen Warning

Click "More info" → "Run anyway" to bypass the warning. This happens because the binary isn't signed with a Microsoft certificate.

### Installation Updates

To update to the latest version:

```bash
# Re-run the install script
curl -fsSL https://raw.githubusercontent.com/billmal071/bookdl/main/scripts/install-linux.sh | bash

# Or with Homebrew
brew upgrade bookdl
```

### Uninstallation

```bash
# Remove binary
sudo rm $(which bookdl)

# Remove config (optional)
rm -rf ~/.config/bookdl

# Remove data (optional)
rm -rf ~/.local/share/bookdl
```

## Next Steps

After installation:

1. Try searching for a book:
   ```bash
   bookdl search "clean code"
   ```

2. Configure your preferences:
   ```bash
   bookdl config set downloads.path ~/Books
   ```

3. See all available commands:
   ```bash
   bookdl --help
   ```

For more usage examples, see the [README](../README.md#usage).
