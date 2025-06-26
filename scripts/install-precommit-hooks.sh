#!/bin/bash

# Install pre-commit framework hooks

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "Installing pre-commit framework hooks..."
echo "======================================="

# Check if pre-commit is installed
if ! command -v pre-commit &> /dev/null; then
    echo "Installing pre-commit..."
    if command -v pip &> /dev/null; then
        pip install pre-commit
    elif command -v brew &> /dev/null; then
        brew install pre-commit
    else
        echo "Error: Neither pip nor brew is available. Please install pre-commit manually."
        echo "Visit: https://pre-commit.com/#install"
        exit 1
    fi
fi

# Change to root directory
cd "$ROOT_DIR"

# Install the git hooks
echo "Installing git hooks..."
pre-commit install
pre-commit install --hook-type commit-msg

# Run on all files to establish baseline
echo "Running pre-commit on all files to establish baseline..."
pre-commit run --all-files || true

echo ""
echo "✅ Pre-commit hooks installed successfully!"
echo ""
echo "The following checks will run automatically:"
echo "  - Trailing whitespace removal"
echo "  - End of file fixing"
echo "  - YAML validation"
echo "  - Large file prevention (>1MB)"
echo "  - Go formatting (gofmt -s)"
echo "  - Go imports organization (goimports)"
echo "  - Go module tidying"
echo "  - Linting with golangci-lint"
echo ""
echo "To manually run hooks:"
echo "  pre-commit run --all-files"
echo ""
echo "To bypass hooks (emergency only):"
echo "  git commit --no-verify"
