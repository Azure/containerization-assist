#!/usr/bin/env bash
# Records successful tool calls for containerization workflow.
# Supports two modes:
# 1) Positional args: $1 = tool name, $2 = raw JSON output (optional)
# 2) Structured JSON via stdin with fields: {"tool":"<name>", "success": true/false}
set -euo pipefail

LOG_FILE="/tmp/tool-success.log"
TOOL_NAME="${1:-}"
OUTPUT_JSON="${2:-}"

# If no positional tool name, try to parse from stdin JSON
if [[ -z "$TOOL_NAME" ]]; then
  if IFS= read -r -t 0.1 STDIN_DATA; then
    # Read full stdin
    STDIN_DATA+=$(cat)
    # Attempt to parse tool and success
    PARSED_TOOL=$(echo "$STDIN_DATA" | jq -r '.tool // empty' 2>/dev/null || echo "")
    PARSED_SUCCESS=$(echo "$STDIN_DATA" | jq -r '.success // empty' 2>/dev/null || echo "")
    if [[ -n "$PARSED_TOOL" ]]; then
      TOOL_NAME="$PARSED_TOOL"
      if [[ -z "$OUTPUT_JSON" ]]; then
        OUTPUT_JSON="$STDIN_DATA"
      fi
      # If success provided explicitly, forward it later
      EXPLICIT_SUCCESS="$PARSED_SUCCESS"
    fi
  fi
fi

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

SUCCESS_FLAG="false"
# Prefer explicit success from stdin JSON if available
if [[ -n "${EXPLICIT_SUCCESS:-}" ]]; then
  SUCCESS_FLAG="$EXPLICIT_SUCCESS"
elif [[ -n "$OUTPUT_JSON" ]]; then
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
