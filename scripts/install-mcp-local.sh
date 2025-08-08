#!/bin/bash
# Offline MCP installer for macOS/Linux
# - Accepts a local MCP binary path (no network required)
# - Installs it as `container-kit-mcp` into ~/.container-kit by default (no PATH needed)
# - Creates global user MCP config by default (VS Code user mcp.json), or
#   .vscode/mcp.json if --workspace is provided. It will only create the file
#   if it does not already exist (non-destructive).
#
# Usage:
#   ./scripts/install-mcp-local.sh --binary /path/to/container-kit-mcp [--workspace /path/to/repo] [--install-dir "$HOME/.container-kit"] [--force]
#   ENV: CONFIG_VSCODE=true|false (default true)

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

REPO_OWNER="Azure"
REPO_NAME="container-kit"
TARGET_NAME="container-kit-mcp"
DEFAULT_INSTALL_DIR="$HOME/.container-kit"
CONFIG_VSCODE=${CONFIG_VSCODE:-true}

info() { echo -e "${YELLOW}$*${NC}"; }
success() { echo -e "${GREEN}$*${NC}"; }
error() { echo -e "${RED}Error: $*${NC}" >&2; }

command_exists() { command -v "$1" >/dev/null 2>&1; }

# Parse args
BINARY_SRC=""
WORKSPACE_DIR=""
INSTALL_DIR="$DEFAULT_INSTALL_DIR"
FORCE=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --binary)
      BINARY_SRC=${2:-}; shift 2 ;;
    --workspace)
      WORKSPACE_DIR=${2:-}; shift 2 ;;
    --install-dir)
      INSTALL_DIR=${2:-}; shift 2 ;;
    --force)
      FORCE=true; shift ;;
    -h|--help)
      cat <<EOF
Offline MCP installer

Options:
  --binary PATH         Path to the container-kit-mcp binary to install (required)
  --workspace PATH      VS Code workspace folder to configure (.vscode/mcp.json)
  --install-dir PATH    Installation directory (default: $DEFAULT_INSTALL_DIR)
  --force               Overwrite existing installed binary if present
  --help                Show this help

Env:
  CONFIG_VSCODE=true|false  Whether to write MCP config (default: true)
  MCP_GLOBAL_CONFIG=PATH    Override path to global MCP config file
EOF
      exit 0 ;;
    *)
      error "Unknown argument: $1"; exit 1 ;;
  esac
done

# Basic validation
if [[ -z "$BINARY_SRC" ]]; then
  error "--binary PATH is required"; exit 1
fi
if [[ ! -f "$BINARY_SRC" ]]; then
  error "Binary not found: $BINARY_SRC"; exit 1
fi

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  linux|darwin)
    ;;
  mingw*|msys*|cygwin*|windows*)
    info "Windows detected. This installer is for macOS/Linux only."
    info "Use the PowerShell setup instead:"
    info "  scripts/setup-user.ps1 -Force"
    info "Or download and run:"
    info "  iwr https://raw.githubusercontent.com/Azure/container-kit/main/scripts/setup-user.ps1 -OutFile setup-user.ps1; ./setup-user.ps1 -Force"
    exit 0
    ;;
  *)
    error "Unsupported OS: $OS"; exit 1
    ;;
esac

# Install
info "Installing $TARGET_NAME..."
mkdir -p "$INSTALL_DIR"
install_path="$INSTALL_DIR/$TARGET_NAME"

if [[ -f "$install_path" && $FORCE != true ]]; then
  info "$install_path already exists (use --force to overwrite)"
fi

# Try copy to chosen directory; if not writable, attempt sudo
cp "$BINARY_SRC" "$install_path" 2>/dev/null || { info "Installing to $INSTALL_DIR (sudo may be required)"; sudo cp "$BINARY_SRC" "$install_path"; }
chmod +x "$install_path" || sudo chmod +x "$install_path" || true
success "Installed to: $install_path"

# VS Code configuration
if [[ "$CONFIG_VSCODE" == "true" ]]; then
  # If a workspace was provided, prefer workspace-level config. Otherwise default to
  # global (user-level) MCP config consumed by GitHub Copilot Chat.

  # Resolve global MCP config file path candidates if override not provided
  detect_global_mcp_config() {
    if [[ -n "${MCP_GLOBAL_CONFIG:-}" ]]; then
      echo "$MCP_GLOBAL_CONFIG"
      return
    fi
    local path=""
    if [[ "$OS" == "darwin" ]]; then
      local base="$HOME/Library/Application Support"
      local candidates=(
        "$base/Code/User/globalStorage/github.copilot-chat/mcp.json"
        "$base/Code - Insiders/User/globalStorage/github.copilot-chat/mcp.json"
        "$base/VSCodium/User/globalStorage/github.copilot-chat/mcp.json"
      )
      for c in "${candidates[@]}"; do path="$c"; break; done
    else
      local cfgbase="${XDG_CONFIG_HOME:-$HOME/.config}"
      local candidates=(
        "$cfgbase/Code/User/globalStorage/github.copilot-chat/mcp.json"
        "$cfgbase/Code - Insiders/User/globalStorage/github.copilot-chat/mcp.json"
        "$cfgbase/VSCodium/User/globalStorage/github.copilot-chat/mcp.json"
      )
      for c in "${candidates[@]}"; do path="$c"; break; done
    fi
    echo "$path"
  }

  if [[ -n "${WORKSPACE_DIR}" ]]; then
    mkdir -p "$WORKSPACE_DIR/.vscode"
    MCP_CONFIG_FILE="$WORKSPACE_DIR/.vscode/mcp.json"
  else
    MCP_CONFIG_FILE="$(detect_global_mcp_config)"
    if [[ -z "$MCP_CONFIG_FILE" ]]; then
      info "Could not resolve a global MCP config path; skipping config. Set MCP_GLOBAL_CONFIG to override."
      MCP_CONFIG_FILE=""
    else
      mkdir -p "$(dirname "$MCP_CONFIG_FILE")"
    fi
  fi

  if [[ -n "$MCP_CONFIG_FILE" ]]; then
    if [[ -f "$MCP_CONFIG_FILE" ]]; then
  info "$MCP_CONFIG_FILE already exists; leaving it unchanged"
  echo
  info "Add the following entry under the 'servers' object to enable Container Kit:"
  cat <<EOF
"containerKit": {
  "type": "stdio",
  "command": "$install_path",
  "args": []
}
EOF
  echo
  info "Note: Ensure it's inserted within the top-level 'servers' map, with proper commas."
    else
      info "Creating $MCP_CONFIG_FILE"
      cat > "$MCP_CONFIG_FILE" <<EOF
{
  "servers": {
    "containerKit": {
      "type": "stdio",
      "command": "$install_path",
      "args": []
    }
  }
}
EOF
      success "MCP configured at: $MCP_CONFIG_FILE"
    fi
  fi
fi

# Verify
ver=$("$install_path" --version 2>/dev/null || echo "unknown")
success "✅ $TARGET_NAME installed. Version: $ver"

info "Done. For Windows users, use scripts/setup-user.ps1 instead."
