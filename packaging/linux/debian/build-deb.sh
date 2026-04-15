#!/bin/bash

# Build .deb package for Debian/Ubuntu
# Usage: ./build-deb.sh <version> <arch> <binary-path>

set -e

VERSION=$1
ARCH=$2
BINARY_PATH=$3

if [ -z "$VERSION" ] || [ -z "$ARCH" ] || [ -z "$BINARY_PATH" ]; then
    echo "Usage: $0 <version> <arch> <binary-path>"
    echo "Example: $0 1.0.0 amd64 ./bookdl-linux-amd64"
    exit 1
fi

# Create package structure
PKG_DIR=$(mktemp -d)
PACKAGE_NAME="bookdl_${VERSION}_${ARCH}"

mkdir -p "$PKG_DIR/DEBIAN"
mkdir -p "$PKG_DIR/usr/bin"
mkdir -p "$PKG_DIR/usr/share/doc/bookdl"
mkdir -p "$PKG_DIR/usr/share/man/man1"

# Copy binary
cp "$BINARY_PATH" "$PKG_DIR/usr/bin/bookdl"
chmod 755 "$PKG_DIR/usr/bin/bookdl"

# Copy documentation
cp README.md "$PKG_DIR/usr/share/doc/bookdl/"
cp LICENSE "$PKG_DIR/usr/share/doc/bookdl/"

# Create control file
cat > "$PKG_DIR/DEBIAN/control" <<EOF
Package: bookdl
Version: $VERSION
Section: utils
Priority: optional
Architecture: $ARCH
Maintainer: billmal071 <https://github.com/billmal071>
Description: Multi-source book downloader
 bookdl is a command-line tool for searching and downloading
 books from multiple sources: Anna's Archive, Z-Library, and Liber3.
 .
 Features:
  - Multi-source search
  - Resumable downloads
  - Cloudflare bypass
  - Interactive TUI
Homepage: https://github.com/billmal071/bookdl
EOF

# Create postinst script
cat > "$PKG_DIR/DEBIAN/postinst" <<'EOF'
#!/bin/bash
set -e
echo "bookdl has been installed successfully."
echo ""
echo "To verify installation, run:"
echo "  bookdl --version"
echo ""
echo "For usage information, run:"
echo "  bookdl --help"
EOF
chmod 755 "$PKG_DIR/DEBIAN/postinst"

# Build package
dpkg-deb --build "$PKG_DIR" "${PACKAGE_NAME}.deb"

# Cleanup
rm -rf "$PKG_DIR"

echo "Package created: ${PACKAGE_NAME}.deb"
