#!/bin/bash

# Container Kit MCP Server Wrapper
# This script ensures proper initialization for the MCP server

# Set up paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DATA_DIR="${HOME}/.container-kit"
WORKSPACE_DIR="${DATA_DIR}/workspaces"
STORE_PATH="${DATA_DIR}/sessions.db"

# Create necessary directories
mkdir -p "${WORKSPACE_DIR}"
mkdir -p "${DATA_DIR}"

# Create logs directory
mkdir -p "${DATA_DIR}/logs"

# Kill any existing container-kit-mcp processes to avoid conflicts
# This prevents database lock issues and hanging connections
if pgrep -f "container-kit-mcp.*--transport=stdio" > /dev/null 2>&1; then
    if [ -n "$DEBUG" ]; then
        echo "Found existing container-kit-mcp process(es), terminating..." >&2
    fi
    pkill -f "container-kit-mcp.*--transport=stdio"
    # Give processes time to clean up
    sleep 0.5

    # If FORCE_CLEANUP is set, use SIGKILL after waiting
    if [ -n "$FORCE_CLEANUP" ] && pgrep -f "container-kit-mcp.*--transport=stdio" > /dev/null 2>&1; then
        if [ -n "$DEBUG" ]; then
            echo "Process still running, using SIGKILL..." >&2
        fi
        pkill -9 -f "container-kit-mcp.*--transport=stdio"
        sleep 0.5
    fi
fi

# Clean up any stale preferences database
PREFS_PATH="${WORKSPACE_DIR}/preferences.db"
if [ -f "${PREFS_PATH}" ] && ! pgrep -f "container-kit-mcp" > /dev/null 2>&1; then
    # Check if the preferences database file is actually locked
    if ! fuser "${PREFS_PATH}" > /dev/null 2>&1; then
        # No process is using the file, but it might have a stale lock
        if [ -n "$DEBUG" ]; then
            echo "Preferences database exists but no process is using it, checking for stale locks..." >&2
        fi

        # Try to remove any .lock file that BoltDB might have created
        rm -f "${PREFS_PATH}.lock" 2>/dev/null

        # Also check if we can open the file for writing (this will fail if locked)
        if ! (echo -n "" >> "${PREFS_PATH}" 2>/dev/null); then
            if [ -n "$DEBUG" ]; then
                echo "Preferences database appears to be locked, moving it aside..." >&2
            fi
            # Move the locked database aside with timestamp
            mv "${PREFS_PATH}" "${PREFS_PATH}.locked.$(date +%Y%m%d_%H%M%S)"
        fi
    else
        # Another process is using the preferences database
        if [ -n "$DEBUG" ]; then
            USING_PID=$(fuser "${PREFS_PATH}" 2>/dev/null | tr -d ' ')
            echo "Preferences database is being used by process ${USING_PID}" >&2
        fi
    fi
fi

# Force cleanup of BoltDB locks if no MCP process is running
if [ -f "${STORE_PATH}" ] && ! pgrep -f "container-kit-mcp" > /dev/null 2>&1; then
    # Check if the database file is actually locked
    if ! fuser "${STORE_PATH}" > /dev/null 2>&1; then
        # No process is using the file, but BoltDB might have a stale lock
        if [ -n "$DEBUG" ]; then
            echo "Database file exists but no process is using it, checking for stale locks..." >&2
        fi

        # Try to remove any .lock file that BoltDB might have created
        rm -f "${STORE_PATH}.lock" 2>/dev/null

        # Also check if we can open the file for writing (this will fail if locked)
        if ! (echo -n "" >> "${STORE_PATH}" 2>/dev/null); then
            if [ -n "$DEBUG" ]; then
                echo "Database appears to be locked, moving it aside..." >&2
            fi
            # Move the locked database aside with timestamp
            mv "${STORE_PATH}" "${STORE_PATH}.locked.$(date +%Y%m%d_%H%M%S)"
        fi
    else
        # Another process is using the database
        if [ -n "$DEBUG" ]; then
            USING_PID=$(fuser "${STORE_PATH}" 2>/dev/null | tr -d ' ')
            echo "Database is being used by process ${USING_PID}" >&2
        fi
        # Use a temporary database in this case
        TEMP_DIR=$(mktemp -d -t container-kit-mcp-XXXXXX)
        STORE_PATH="${TEMP_DIR}/sessions.db"
        export MCP_TEMP_DB="true"
        # Clean up on exit
        trap "rm -rf ${TEMP_DIR}" EXIT
    fi
fi

# Check if we're in test mode and should use temporary database
if [ -n "$MCP_TEST_MODE" ]; then
    TEMP_DIR=$(mktemp -d -t container-kit-mcp-XXXXXX)
    STORE_PATH="${TEMP_DIR}/sessions.db"
    export MCP_TEMP_DB="true"
    # Clean up on exit
    trap "rm -rf ${TEMP_DIR}" EXIT
fi

# Find the container-kit-mcp binary
# First check if it's in the parent directory
if [ -f "${SCRIPT_DIR}/../container-kit-mcp" ]; then
    MCP_BINARY="${SCRIPT_DIR}/../container-kit-mcp"
elif [ -f "${SCRIPT_DIR}/container-kit-mcp" ]; then
    MCP_BINARY="${SCRIPT_DIR}/container-kit-mcp"
else
    echo "Error: container-kit-mcp binary not found" >&2
    echo "Please build it with: go build -tags mcp -o container-kit-mcp ./cmd/mcp-server" >&2
    exit 1
fi

# Execute the MCP server with proper configuration
exec "${MCP_BINARY}" \
    --workspace-dir="${WORKSPACE_DIR}" \
    --store-path="${STORE_PATH}" \
    --transport=stdio \
    --log-level=info \
    --conversation \
    "$@"
