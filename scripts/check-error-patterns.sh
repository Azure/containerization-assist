#!/bin/bash
set -e

MAX_FMT_ERRORF=${1:-100}
echo "Checking error patterns (max fmt.Errorf: $MAX_FMT_ERRORF)..."

# Build linter if needed
if [ ! -f tools/linters/richerror-boundary/richerror-boundary ]; then
    echo "Building error linter..."
    (cd tools/linters/richerror-boundary && go build -o richerror-boundary .)
fi

# Run error pattern check
./tools/linters/richerror-boundary/richerror-boundary pkg/mcp $MAX_FMT_ERRORF
