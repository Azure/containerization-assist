#!/usr/bin/env bash
# Records successful tool calls for containerization workflow.
# Called by mcphost PostToolUse hook; reads JSON payload from stdin.
set -euo pipefail

DEBUG_LOG="/tmp/hook-debug.log"
LOG_FILE="/tmp/tool-success.log"

mkdir -p /tmp
touch "$DEBUG_LOG" "$LOG_FILE"

echo "$(date): PostToolUse hook invoked" >> "$DEBUG_LOG"

# Read JSON from stdin as per mcphost examples (contains tool_name, tool_input)
PAYLOAD=$(cat || true)
if [ -z "${PAYLOAD}" ]; then
  echo "$(date): No stdin payload provided to hook" >> "$DEBUG_LOG"
  exit 0
fi

echo "$(date): Raw payload: ${PAYLOAD}" >> "$DEBUG_LOG"

# Extract tool_name field
TOOL_NAME=$(echo "$PAYLOAD" | jq -r '.tool_name // empty' 2>/dev/null || echo "")
if [ -z "$TOOL_NAME" ]; then
  echo "$(date): tool_name missing in payload" >> "$DEBUG_LOG"
  exit 0
fi

echo "$(date): Extracted tool_name: '$TOOL_NAME'" >> "$DEBUG_LOG"

# Accept base names or namespaced ones like mcp_containerkit_*; normalize by stripping common prefixes
NORM_NAME="$TOOL_NAME"
if [[ "$NORM_NAME" == mcp_containerkit_* ]]; then
  NORM_NAME="${NORM_NAME#mcp_containerkit_}"
elif [[ "$NORM_NAME" == mcp__*__* ]]; then
  # pattern like mcp__server__tool
  NORM_NAME="${NORM_NAME##*__}"
fi

REQUIRED_TOOLS=(
  analyze_repository
  generate_dockerfile
  build_image
  scan_image
  tag_image
  push_image
  generate_k8s_manifests
  prepare_cluster
  deploy_application
  verify_deployment
)

for t in "${REQUIRED_TOOLS[@]}"; do
  if [[ "$NORM_NAME" == "$t" ]]; then
    if ! grep -q "^${t}$" "$LOG_FILE"; then
      echo "$t" >> "$LOG_FILE"
      echo "$(date): Logged required tool: $t (from '$TOOL_NAME')" >> "$DEBUG_LOG"
    else
      echo "$(date): Tool already logged: $t" >> "$DEBUG_LOG"
    fi
    exit 0
  fi
done

echo "$(date): Non-required tool encountered: '$TOOL_NAME' (normalized: '$NORM_NAME')" >> "$DEBUG_LOG"
exit 0
