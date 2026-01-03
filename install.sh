#!/bin/bash
# codebak installer
# Usage: curl -sSL https://raw.githubusercontent.com/jmcdonald/codebak/main/install.sh | bash

set -e

REPO="jmcdonald/codebak"
INSTALL_DIR="${INSTALL_DIR:-$HOME/bin}"
BINARY="codebak"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() { echo -e "${GREEN}==>${NC} $1"; }
warn() { echo -e "${YELLOW}==>${NC} $1"; }
error() { echo -e "${RED}ERROR:${NC} $1" >&2; exit 1; }

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$ARCH" in
        x86_64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *) error "Unsupported architecture: $ARCH" ;;
    esac

    case "$OS" in
        darwin|linux) ;;
        *) error "Unsupported OS: $OS" ;;
    esac

    PLATFORM="${OS}_${ARCH}"
    info "Detected platform: $PLATFORM"
}

# Get latest release version
get_latest_version() {
    VERSION=$(curl -sSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/')
    if [ -z "$VERSION" ]; then
        error "Could not determine latest version"
    fi
    info "Latest version: v$VERSION"
}

# Download and install
install() {
    TARBALL="${BINARY}_${VERSION}_${PLATFORM}.tar.gz"
    DOWNLOAD_URL="https://github.com/$REPO/releases/download/v$VERSION/$TARBALL"
    CHECKSUM_URL="https://github.com/$REPO/releases/download/v$VERSION/checksums.txt"

    info "Downloading $TARBALL..."
    TMP_DIR=$(mktemp -d)
    trap "rm -rf $TMP_DIR" EXIT

    curl -sSL "$DOWNLOAD_URL" -o "$TMP_DIR/$TARBALL" || error "Download failed"
    curl -sSL "$CHECKSUM_URL" -o "$TMP_DIR/checksums.txt" || warn "Could not download checksums"

    # Verify checksum if available
    if [ -f "$TMP_DIR/checksums.txt" ]; then
        info "Verifying checksum..."
        cd "$TMP_DIR"
        EXPECTED=$(grep "$TARBALL" checksums.txt | awk '{print $1}')
        if [ -n "$EXPECTED" ]; then
            if command -v sha256sum &> /dev/null; then
                ACTUAL=$(sha256sum "$TARBALL" | awk '{print $1}')
            elif command -v shasum &> /dev/null; then
                ACTUAL=$(shasum -a 256 "$TARBALL" | awk '{print $1}')
            else
                warn "No sha256 tool found, skipping verification"
                ACTUAL="$EXPECTED"
            fi
            if [ "$EXPECTED" != "$ACTUAL" ]; then
                error "Checksum mismatch! Expected: $EXPECTED, Got: $ACTUAL"
            fi
            info "Checksum verified"
        fi
        cd - > /dev/null
    fi

    # Extract and install
    info "Installing to $INSTALL_DIR..."
    mkdir -p "$INSTALL_DIR"
    tar -xzf "$TMP_DIR/$TARBALL" -C "$TMP_DIR"
    mv "$TMP_DIR/$BINARY" "$INSTALL_DIR/$BINARY"
    chmod +x "$INSTALL_DIR/$BINARY"

    # Verify installation
    if [ -x "$INSTALL_DIR/$BINARY" ]; then
        info "Successfully installed $BINARY to $INSTALL_DIR"
        echo ""
        "$INSTALL_DIR/$BINARY" version
        echo ""

        # Add to PATH if not already there
        if ! echo "$PATH" | grep -q "$INSTALL_DIR"; then
            add_to_path
        fi
    else
        error "Installation failed"
    fi
}

# Add install dir to shell PATH
add_to_path() {
    SHELL_NAME=$(basename "$SHELL")
    PATH_EXPORT="export PATH=\"\$PATH:$INSTALL_DIR\""

    case "$SHELL_NAME" in
        zsh)
            RC_FILE="$HOME/.zshrc"
            ;;
        bash)
            # Prefer .bashrc, fall back to .bash_profile
            if [ -f "$HOME/.bashrc" ]; then
                RC_FILE="$HOME/.bashrc"
            else
                RC_FILE="$HOME/.bash_profile"
            fi
            ;;
        fish)
            RC_FILE="$HOME/.config/fish/config.fish"
            PATH_EXPORT="set -gx PATH \$PATH $INSTALL_DIR"
            ;;
        *)
            warn "Unknown shell: $SHELL_NAME"
            warn "Add to your PATH manually:"
            echo "  export PATH=\"\$PATH:$INSTALL_DIR\""
            return
            ;;
    esac

    # Check if already in rc file
    if [ -f "$RC_FILE" ] && grep -q "$INSTALL_DIR" "$RC_FILE" 2>/dev/null; then
        info "PATH already configured in $RC_FILE"
        return
    fi

    # Add to rc file
    echo "" >> "$RC_FILE"
    echo "# Added by codebak installer" >> "$RC_FILE"
    echo "$PATH_EXPORT" >> "$RC_FILE"

    info "Added $INSTALL_DIR to PATH in $RC_FILE"
    warn "Run 'source $RC_FILE' or open a new terminal to use codebak"
}

main() {
    info "Installing codebak..."
    detect_platform
    get_latest_version
    install
    info "Done! Run 'codebak help' to get started."
}

main
