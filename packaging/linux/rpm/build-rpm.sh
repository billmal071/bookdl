#!/bin/bash

# Build .rpm package for Fedora/CentOS/RHEL
# Usage: ./build-rpm.sh <version> <arch> <binary-path>

set -e

VERSION=$1
ARCH=$2
BINARY_PATH=$3

if [ -z "$VERSION" ] || [ -z "$ARCH" ] || [ -z "$BINARY_PATH" ]; then
    echo "Usage: $0 <version> <arch> <binary-path>"
    echo "Example: $0 1.0.0 x86_64 ./bookdl-linux-amd64"
    exit 1
fi

# Strip 'v' prefix if present for RPM version
VERSION=${VERSION#v}

# Create RPM build directories
TOP_DIR=$(mktemp -d)
mkdir -p "$TOP_DIR"/{BUILD,RPMS,SOURCES,SPECS,SRPMS}

# Update spec file with version
sed -i "s/^Version:.*/Version:        $VERSION/" packaging/linux/rpm/bookdl.spec

# Copy binary to SOURCES
cp "$BINARY_PATH" "$TOP_DIR/SOURCES/bookdl-linux-$ARCH"

# Copy spec file
cp packaging/linux/rpm/bookdl.spec "$TOP_DIR/SPECS/"

# Build RPM
rpmbuild --define "_topdir $TOP_DIR" \
         --target "$ARCH" \
         -bb "$TOP_DIR/SPECS/bookdl.spec"

# Copy RPM to current directory
cp "$TOP_DIR/RPMS/$ARCH/"*.rpm .

# Cleanup
rm -rf "$TOP_DIR"

echo "Package created: bookdl-${VERSION}-1.${ARCH}.rpm"
