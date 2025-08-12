#!/usr/bin/env bash
# Records successful tool calls for containerization workflow.
# Arguments: $1 = tool name, $2 = raw JSON output (optional)
set -euo pipefail
TOOL_NAME="$1"
OUTPUT_JSON="${2:-}"
LOG_FILE="/tmp/tool-success.log"

# Normalize tool names (expected 10 required tools)
# These must match the containerization tool names exposed by your MCP server
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

# Ensure log file exists
mkdir -p /tmp
: > /tmp/.tool-success.touch 2>/dev/null || true
[ -f "$LOG_FILE" ] || touch "$LOG_FILE"

# Only consider required tools
is_required=false
for t in "${REQUIRED_TOOLS[@]}"; do
  if [[ "$t" == "$TOOL_NAME" ]]; then
    is_required=true
    break
  fi
done

if [[ "$is_required" != true ]]; then
  exit 0
fi

# Determine success using .success flag if JSON is provided, else assume failure
SUCCESS_FLAG="false"
if [[ -n "$OUTPUT_JSON" ]]; then
  # Strip ANSI, control chars, then attempt to read .success
  CLEAN=$(echo "$OUTPUT_JSON" | sed 's/\x1b\[[0-9;]*m//g' | tr -d '\000-\010\013\014\016-\037' | tr -s ' ')
  SUCCESS_FLAG=$(echo "$CLEAN" | jq -r '.success // "false"' 2>/dev/null || echo "false")
fi

if [[ "$SUCCESS_FLAG" == "true" ]]; then
  # Record success once per tool
  if ! grep -q "^${TOOL_NAME}$" "$LOG_FILE"; then
    echo "$TOOL_NAME" >> "$LOG_FILE"
  fi
fi

exit 0
