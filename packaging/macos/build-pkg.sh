#!/bin/bash

# Build macOS .pkg installer
# Usage: ./build-pkg.sh <version> <binary-path>

set -e

VERSION=$1
BINARY_PATH=$2

if [ -z "$VERSION" ] || [ -z "$BINARY_PATH" ]; then
    echo "Usage: $0 <version> <binary-path>"
    exit 1
fi

# Create package structure
PKG_ROOT=$(mktemp -d)
APP_DIR="$PKG_ROOT/usr/local/bin"
RESOURCES_DIR="$PKG_ROOT/Resources"

mkdir -p "$APP_DIR"
mkdir -p "$RESOURCES_DIR"

# Copy binary
cp "$BINARY_PATH" "$APP_DIR/bookdl"
chmod +x "$APP_DIR/bookdl"

# Create postinstall script
cat > "$RESOURCES_DIR/postinstall" <<'EOF'
#!/bin/bash
echo "bookdl has been installed to /usr/local/bin/bookdl"
echo ""
echo "You may need to reload your shell or run:"
echo "  hash -r"
echo ""
echo "To verify installation, run:"
echo "  bookdl --version"
echo ""
echo "For usage information, run:"
echo "  bookdl --help"
EOF
chmod +x "$RESOURCES_DIR/postinstall"

# Build package
pkgbuild \
    --root "$PKG_ROOT" \
    --version "$VERSION" \
    --identifier "com.github.bookdl" \
    --install-location "/" \
    --scripts "$RESOURCES_DIR" \
    "bookdl-$VERSION.pkg"

# Cleanup
rm -rf "$PKG_ROOT"

echo "Package created: bookdl-$VERSION.pkg"
