#!/bin/bash

set -euo pipefail

# Wrapper script for MCP Inspector to run the TypeScript CLI in mock mode
cd "$(dirname "$0")/.."

# Set environment variables for clean MCP operation
export MCP_MODE=true
export MCP_QUIET=true

exec npx tsx src/cli/cli.ts --mock "$@"