#!/bin/sh
# Tomato Installation Script
# Usage: curl -fsSL https://raw.githubusercontent.com/tomatool/tomato/main/install.sh | sh
# Usage: curl -fsSL https://raw.githubusercontent.com/tomatool/tomato/main/install.sh | sh -s -- --rc
# Usage: curl -fsSL https://raw.githubusercontent.com/tomatool/tomato/main/install.sh | USE_RC=true sh

set -e

REPO="tomatool/tomato"
BINARY_NAME="tomato"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
USE_RC="${USE_RC:-false}"

# Parse arguments
while [ $# -gt 0 ]; do
    case "$1" in
        --rc)
            USE_RC=true
            shift
            ;;
        *)
            shift
            ;;
    esac
done

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() {
    printf "${GREEN}[INFO]${NC} %s\n" "$1"
}

warn() {
    printf "${YELLOW}[WARN]${NC} %s\n" "$1"
}

error() {
    printf "${RED}[ERROR]${NC} %s\n" "$1"
    exit 1
}

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$OS" in
        darwin) OS="darwin" ;;
        linux) OS="linux" ;;
        *) error "Unsupported operating system: $OS (supported: linux, darwin)" ;;
    esac

    case "$ARCH" in
        x86_64|amd64) ARCH="amd64" ;;
        arm64|aarch64) ARCH="arm64" ;;
        *) error "Unsupported architecture: $ARCH (supported: amd64, arm64)" ;;
    esac

    PLATFORM="${OS}_${ARCH}"
    info "Detected platform: $PLATFORM"
}

# Get latest release version
get_latest_version() {
    if [ "$USE_RC" = true ]; then
        # Get highest RC version by semver
        VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases" | \
            grep '"tag_name"' | \
            sed -E 's/.*"([^"]+)".*/\1/' | \
            grep -E '\-rc\.[0-9]+$' | \
            sort -t. -k1,1V -k2,2V -k3,3V -k4,4V | \
            tail -n1)
        if [ -z "$VERSION" ]; then
            error "No release candidate found. Use without --rc for stable release."
        fi
        info "Latest RC version: $VERSION"
    else
        # Get latest stable release
        VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
        if [ -z "$VERSION" ]; then
            error "Failed to get latest version"
        fi
        info "Latest version: $VERSION"
    fi
}

# Download and install
install() {
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY_NAME}_${VERSION#v}_${PLATFORM}.tar.gz"

    info "Downloading from: $DOWNLOAD_URL"

    TMP_DIR=$(mktemp -d)
    trap "rm -rf $TMP_DIR" EXIT

    if ! curl -fsSL "$DOWNLOAD_URL" -o "$TMP_DIR/tomato.tar.gz"; then
        error "Failed to download release. Please check if the release exists for your platform."
    fi

    info "Extracting..."
    tar -xzf "$TMP_DIR/tomato.tar.gz" -C "$TMP_DIR"

    # Check if we need sudo
    if [ -w "$INSTALL_DIR" ]; then
        mv "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR/"
        chmod +x "$INSTALL_DIR/$BINARY_NAME"
    else
        warn "Need sudo to install to $INSTALL_DIR"
        sudo mv "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR/"
        sudo chmod +x "$INSTALL_DIR/$BINARY_NAME"
    fi

    info "Installed to: $INSTALL_DIR/$BINARY_NAME"
}

# Verify installation
verify() {
    if command -v tomato >/dev/null 2>&1; then
        INSTALLED_VERSION=$(tomato --version 2>&1 | head -n1)
        info "Successfully installed: $INSTALLED_VERSION"
    else
        warn "Installation complete, but 'tomato' not found in PATH"
        warn "Add $INSTALL_DIR to your PATH or run: export PATH=\"\$PATH:$INSTALL_DIR\""
    fi
}

# Main
main() {
    echo ""
    echo "  ___________  __  ______  __________"
    echo "    / __/ __ \\/  |/  / _ |/_  __/ __ \\"
    echo "   / _// /_/ / /|_/ / __ | / / / /_/ /"
    echo "  /___/\\____/_/  /_/_/ |_|/_/  \\____/"
    echo ""
    echo "  Behavioral testing toolkit"
    echo ""

    detect_platform
    get_latest_version
    install
    verify

    echo ""
    info "Get started with: tomato init"
    echo ""
}

main "$@"
