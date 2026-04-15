#!/usr/bin/env bash

# bookdl installer script for macOS and Linux
# Usage: curl -fsSL https://raw.githubusercontent.com/billmal071/bookdl/main/scripts/install.sh | bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
REPO_URL="https://github.com/billmal071/bookdl"
BINARY_NAME="bookdl"
INSTALL_DIR="/usr/local/bin"
TEMP_DIR=$(mktemp -d)

# Cleanup function
cleanup() {
    echo -e "${BLUE}Cleaning up...${NC}"
    rm -rf "$TEMP_DIR"
}
trap cleanup EXIT

# Print functions
print_info() {
    echo -e "${BLUE}ℹ ${NC}$1"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}!${NC} $1"
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$OS" in
        darwin)
            OS="darwin"
            ;;
        linux)
            OS="linux"
            ;;
        *)
            print_error "Unsupported operating system: $OS"
            exit 1
            ;;
    esac

    case "$ARCH" in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        arm64|aarch64)
            ARCH="arm64"
            ;;
        *)
            print_error "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac

    print_info "Detected platform: $OS-$ARCH"
}

# Check prerequisites
check_prerequisites() {
    print_info "Checking prerequisites..."

    # Check for curl or wget
    if ! command_exists curl && ! command_exists wget; then
        print_error "Either curl or wget is required"
        exit 1
    fi

    if command_exists curl; then
        DOWNLOADER="curl"
    else
        DOWNLOADER="wget"
    fi

    print_success "Using $DOWNLOADER for downloads"
}

# Download file
download_file() {
    local url="$1"
    local output="$2"

    if [ "$DOWNLOADER" = "curl" ]; then
        curl -fsSL "$url" -o "$output"
    else
        wget -q "$url" -O "$output"
    fi
}

# Get latest release version
get_latest_version() {
    print_info "Fetching latest release information..."

    local api_url="https://api.github.com/repos/billmal071/bookdl/releases/latest"

    if [ "$DOWNLOADER" = "curl" ]; then
        VERSION=$(curl -fsSL "$api_url" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    else
        VERSION=$(wget -qO- "$api_url" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    fi

    if [ -z "$VERSION" ]; then
        print_warning "Could not determine latest version, using 'latest'"
        VERSION="latest"
    else
        print_success "Latest version: $VERSION"
    fi
}

# Download binary
download_binary() {
    print_info "Downloading bookdl..."

    local download_url
    local binary_file

    if [ "$VERSION" = "latest" ]; then
        download_url="$REPO_URL/releases/latest/download/bookdl-${OS}-${ARCH}"
        if [ "$OS" = "windows" ]; then
            download_url="${download_url}.exe"
        fi
    else
        download_url="$REPO_URL/releases/download/$VERSION/bookdl-${OS}-${ARCH}"
        if [ "$OS" = "windows" ]; then
            download_url="${download_url}.exe"
        fi
    fi

    binary_file="$TEMP_DIR/bookdl"
    if [ "$OS" = "windows" ]; then
        binary_file="${binary_file}.exe"
    fi

    if ! download_file "$download_url" "$binary_file"; then
        print_error "Failed to download bookdl binary"
        exit 1
    fi

    print_success "Download complete"
}

# Install binary
install_binary() {
    print_info "Installing bookdl to $INSTALL_DIR..."

    local binary_file="$TEMP_DIR/bookdl"
    if [ "$OS" = "windows" ]; then
        binary_file="${binary_file}.exe"
    fi

    # Make binary executable
    chmod +x "$binary_file"

    # Check if we need sudo
    if [ ! -w "$INSTALL_DIR" ]; then
        print_warning "Requires sudo to install to $INSTALL_DIR"
        sudo mv "$binary_file" "$INSTALL_DIR/$BINARY_NAME"
    else
        mv "$binary_file" "$INSTALL_DIR/$BINARY_NAME"
    fi

    print_success "Installation complete"
}

# Verify installation
verify_installation() {
    print_info "Verifying installation..."

    if ! command_exists bookdl; then
        print_error "bookdl command not found in PATH"
        exit 1
    fi

    local version
    version=$(bookdl --version 2>&1 || echo "unknown")
    print_success "bookdl installed successfully: $version"
}

# Main installation process
main() {
    echo -e "${BLUE}"
    echo "╔══════════════════════════════════════╗"
    echo "║      bookdl Installer                ║"
    echo "║  Multi-source book downloader        ║"
    echo "╚══════════════════════════════════════╝"
    echo -e "${NC}"

    detect_platform
    check_prerequisites
    get_latest_version
    download_binary
    install_binary
    verify_installation

    echo
    print_success "Installation completed successfully!"
    echo
    echo -e "${YELLOW}Usage:${NC}"
    echo "  bookdl search \"clean code\""
    echo
    echo -e "${YELLOW}For more information:${NC}"
    echo "  bookdl --help"
    echo "  https://github.com/billmal071/bookdl#usage"
    echo
}

# Run main function
main "$@"
