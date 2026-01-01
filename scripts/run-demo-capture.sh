#!/bin/bash
# run-demo-capture.sh - Stage data and capture demo screenshots
#
# This script:
# 1. Stages demo data
# 2. Temporarily swaps codebak config
# 3. Runs VHS to capture screenshots
# 4. Restores original config

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
CONFIG_DIR="$HOME/.codebak"
DEMO_DIR="/tmp/codebak-demo"

echo "==> Staging demo data..."
"$SCRIPT_DIR/stage-demo.sh" "$DEMO_DIR"

echo ""
echo "==> Backing up real config..."
mkdir -p "$CONFIG_DIR"
if [ -f "$CONFIG_DIR/config.yaml" ]; then
    cp "$CONFIG_DIR/config.yaml" "$CONFIG_DIR/config.yaml.bak"
    echo "    Backed up to $CONFIG_DIR/config.yaml.bak"
else
    echo "    No existing config found"
fi

echo ""
echo "==> Installing demo config..."
cp "$DEMO_DIR/.codebak/config.yaml" "$CONFIG_DIR/config.yaml"

cleanup() {
    echo ""
    echo "==> Restoring original config..."
    if [ -f "$CONFIG_DIR/config.yaml.bak" ]; then
        mv "$CONFIG_DIR/config.yaml.bak" "$CONFIG_DIR/config.yaml"
        echo "    Restored from backup"
    else
        rm -f "$CONFIG_DIR/config.yaml"
        echo "    Removed demo config"
    fi
}

# Ensure cleanup runs on exit
trap cleanup EXIT

echo ""
echo "==> Running VHS capture..."
cd "$PROJECT_DIR"
~/go/bin/vhs docs/demo-staged.tape

echo ""
echo "==> Demo capture complete!"
echo "    Screenshots: $PROJECT_DIR/docs/screenshots/"
echo "    GIF: $PROJECT_DIR/docs/codebak-demo.gif"
