#!/bin/bash
# stage-demo.sh - Create staged demo data for compelling screenshots
#
# Usage: ./scripts/stage-demo.sh [demo_dir]
# Default: /tmp/codebak-demo

set -e

DEMO_DIR="${1:-/tmp/codebak-demo}"
DEMO_SOURCE="$DEMO_DIR/code"
DEMO_BACKUP="$DEMO_DIR/backups"
DEMO_CONFIG="$DEMO_DIR/.codebak"

echo "==> Staging codebak demo data in $DEMO_DIR"

# Clean and create directories
rm -rf "$DEMO_DIR"
mkdir -p "$DEMO_SOURCE" "$DEMO_BACKUP" "$DEMO_CONFIG"

# Create config file
cat > "$DEMO_CONFIG/config.yaml" << EOF
source_dir: $DEMO_SOURCE
backup_dir: $DEMO_BACKUP
exclude:
  - node_modules
  - .venv
  - __pycache__
  - .git
retention:
  keep_last: 30
EOF

# ============================================
# CREATE DEMO PROJECTS (varied states)
# ============================================

echo "==> Creating demo source projects..."

# Project 1: Active Go project (will have backups)
mkdir -p "$DEMO_SOURCE/website-redesign/cmd" "$DEMO_SOURCE/website-redesign/pkg/api"
cat > "$DEMO_SOURCE/website-redesign/main.go" << 'EOF'
package main

import "fmt"

func main() {
    fmt.Println("Website Redesign v2.0")
    startServer()
}

func startServer() {
    // Server implementation
}
EOF
cat > "$DEMO_SOURCE/website-redesign/pkg/api/handler.go" << 'EOF'
package api

type Handler struct {
    db Database
}

func (h *Handler) GetUsers() []User {
    return h.db.QueryUsers()
}
EOF
echo "# Website Redesign\nRedesigning the company website." > "$DEMO_SOURCE/website-redesign/README.md"

# Project 2: Python ML project
mkdir -p "$DEMO_SOURCE/ml-pipeline/src" "$DEMO_SOURCE/ml-pipeline/models"
cat > "$DEMO_SOURCE/ml-pipeline/src/train.py" << 'EOF'
import torch
from models import TransformerModel

def train(data_path: str):
    model = TransformerModel()
    # Training logic
    return model
EOF
echo "# ML Pipeline\nMachine learning training pipeline." > "$DEMO_SOURCE/ml-pipeline/README.md"

# Project 3: TypeScript frontend
mkdir -p "$DEMO_SOURCE/mobile-app/src/components" "$DEMO_SOURCE/mobile-app/src/hooks"
cat > "$DEMO_SOURCE/mobile-app/src/App.tsx" << 'EOF'
import React from 'react';
import { Dashboard } from './components/Dashboard';

export const App: React.FC = () => {
  return <Dashboard />;
};
EOF
echo "# Mobile App\nReact Native mobile application." > "$DEMO_SOURCE/mobile-app/README.md"

# Project 4: Rust CLI tool
mkdir -p "$DEMO_SOURCE/cli-toolkit/src"
cat > "$DEMO_SOURCE/cli-toolkit/src/main.rs" << 'EOF'
use clap::Parser;

#[derive(Parser)]
struct Args {
    command: String,
}

fn main() {
    let args = Args::parse();
    println!("Running: {}", args.command);
}
EOF
echo "# CLI Toolkit\nCommand-line utilities." > "$DEMO_SOURCE/cli-toolkit/README.md"

# Project 5: New project (no backups yet)
mkdir -p "$DEMO_SOURCE/new-project"
echo "# New Project\nJust started!" > "$DEMO_SOURCE/new-project/README.md"

# ============================================
# CREATE BACKUP VERSIONS (for history view)
# ============================================

echo "==> Creating backup versions..."

create_backup() {
    local project="$1"
    local version="$2"
    local file_count="$3"
    local size_kb="$4"
    local git_head="$5"
    local days_ago="$6"

    local backup_dir="$DEMO_BACKUP/$project"
    local zip_file="$backup_dir/$version.zip"
    local manifest="$backup_dir/manifest.json"

    mkdir -p "$backup_dir"

    # Create a zip with actual content
    cd "$DEMO_SOURCE/$project"
    zip -r "$zip_file" . -x "*.git*" > /dev/null 2>&1

    # Calculate actual size and checksum
    local actual_size=$(stat -f%z "$zip_file" 2>/dev/null || stat -c%s "$zip_file")
    local checksum=$(shasum -a 256 "$zip_file" | cut -d' ' -f1)

    # Calculate timestamp
    local timestamp=$(date -v-${days_ago}d +"%Y-%m-%dT03:00:00Z" 2>/dev/null || date -d "$days_ago days ago" +"%Y-%m-%dT03:00:00Z")

    # Create or append to manifest
    if [ ! -f "$manifest" ]; then
        cat > "$manifest" << MANIFEST
{
  "project": "$project",
  "source": "$DEMO_SOURCE/$project",
  "backups": []
}
MANIFEST
    fi

    # Add backup entry using jq or sed
    local backup_entry="{\"file\":\"$version.zip\",\"sha256\":\"$checksum\",\"size_bytes\":$actual_size,\"created_at\":\"$timestamp\",\"git_head\":\"$git_head\",\"file_count\":$file_count,\"excluded\":[\"node_modules\",\".venv\"]}"

    # Simple append to backups array (using temp file)
    python3 -c "
import json
with open('$manifest', 'r') as f:
    data = json.load(f)
data['backups'].append($backup_entry)
with open('$manifest', 'w') as f:
    json.dump(data, f, indent=2)
" 2>/dev/null || echo "Warning: Could not update manifest for $project"

    cd - > /dev/null
}

# Website Redesign: 4 versions (active project)
create_backup "website-redesign" "20260101-030000" 45 125 "abc123d" 0
create_backup "website-redesign" "20251231-030000" 42 120 "def456a" 1
create_backup "website-redesign" "20251230-030000" 40 118 "789bcd0" 2
create_backup "website-redesign" "20251229-030000" 38 115 "fed321c" 3

# ML Pipeline: 2 versions
create_backup "ml-pipeline" "20260101-030000" 28 89 "aaa111b" 0
create_backup "ml-pipeline" "20251228-030000" 25 82 "bbb222c" 3

# Mobile App: 3 versions
create_backup "mobile-app" "20251231-030000" 156 342 "ccc333d" 1
create_backup "mobile-app" "20251225-030000" 148 325 "ddd444e" 6
create_backup "mobile-app" "20251220-030000" 140 310 "eee555f" 11

# CLI Toolkit: 1 version
create_backup "cli-toolkit" "20251230-030000" 12 45 "fff666g" 2

# ============================================
# CREATE DIFF DATA (for diff view feature)
# ============================================

echo "==> Creating diff data for version comparison..."

# Modify website-redesign to create actual differences between versions
# Add new file for version 2
cat > "$DEMO_SOURCE/website-redesign/pkg/api/auth.go" << 'EOF'
package api

// AuthMiddleware handles authentication
type AuthMiddleware struct {
    secret string
}

func (a *AuthMiddleware) Validate(token string) bool {
    return len(token) > 0
}
EOF

# Modify existing file
cat > "$DEMO_SOURCE/website-redesign/main.go" << 'EOF'
package main

import (
    "fmt"
    "log"
)

func main() {
    fmt.Println("Website Redesign v2.1")
    if err := startServer(); err != nil {
        log.Fatal(err)
    }
}

func startServer() error {
    // Enhanced server implementation with error handling
    return nil
}

func healthCheck() string {
    return "OK"
}
EOF

# Create a new backup with the modified files
create_backup "website-redesign" "20260101-140000" 48 130 "new123x" 0

echo ""
echo "==> Demo data staged successfully!"
echo ""
echo "Projects created:"
echo "  - website-redesign (4 versions, recent activity)"
echo "  - ml-pipeline (2 versions)"
echo "  - mobile-app (3 versions)"
echo "  - cli-toolkit (1 version)"
echo "  - new-project (no backups)"
echo ""
echo "To use with codebak:"
echo "  export CODEBAK_CONFIG=$DEMO_CONFIG/config.yaml"
echo "  codebak"
echo ""
echo "Or modify ~/.codebak/config.yaml to point to:"
echo "  source_dir: $DEMO_SOURCE"
echo "  backup_dir: $DEMO_BACKUP"
