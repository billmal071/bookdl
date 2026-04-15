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

# Work on a copy of the spec to avoid mutating the repo
cp packaging/linux/rpm/bookdl.spec "$TOP_DIR/SPECS/bookdl.spec"
SPEC="$TOP_DIR/SPECS/bookdl.spec"

# Update spec with version and source
sed -i "s/^Version:.*/Version:        $VERSION/" "$SPEC"
sed -i "s/^Source0:.*/Source0: bookdl-linux-$ARCH/" "$SPEC"

# Use noarch to avoid rpmbuild's platform validation failing on cross-arch
# builds (e.g. building aarch64 RPM on x86_64 Ubuntu runner).
# We rename the output RPM to the correct arch afterwards.
sed -i "s/^BuildArch:.*/BuildArch: noarch/" "$SPEC"

# Copy binary to SOURCES
cp "$BINARY_PATH" "$TOP_DIR/SOURCES/bookdl-linux-$ARCH"

# Build RPM
rpmbuild --define "_topdir $TOP_DIR" \
         -bb "$SPEC"

# Rename noarch RPM to target architecture
NOARCH_RPM=$(find "$TOP_DIR/RPMS" -name "*.noarch.rpm" | head -1)
FINAL_NAME="bookdl-${VERSION}-1.${ARCH}.rpm"
if [ -n "$NOARCH_RPM" ]; then
    cp "$NOARCH_RPM" "./$FINAL_NAME"
else
    find "$TOP_DIR/RPMS" -name "*.rpm" -exec cp {} . \;
fi

# Cleanup
rm -rf "$TOP_DIR"

echo "Package created: $FINAL_NAME"
