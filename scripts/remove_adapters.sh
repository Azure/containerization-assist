#!/bin/bash
# Script to safely remove adapter pattern files

set -e

echo "🔄 Removing Adapter Pattern Files"
echo "================================="

# Check if adapters are still being used
echo "1. Checking for adapter usage..."

DOCKER_ADAPTER_USAGE=$(rg "adapters\.NewDockerClient|NewDockerClient" --type go || echo "")
RETRY_ADAPTER_USAGE=$(rg "adapters\.NewRetryCoordinator|NewRetryCoordinator" --type go || echo "")

if [[ -n "$DOCKER_ADAPTER_USAGE" ]]; then
    echo "❌ Docker adapter still in use:"
    echo "$DOCKER_ADAPTER_USAGE"
    echo "Please migrate to infra.DockerService first"
    exit 1
fi

if [[ -n "$RETRY_ADAPTER_USAGE" ]]; then
    echo "❌ Retry adapter still in use:"
    echo "$RETRY_ADAPTER_USAGE"
    echo "Please migrate to infra.RetryService first"
    exit 1
fi

echo "✅ No adapter usage found"

# Check if adapters directory exists
if [[ ! -d "pkg/mcp/infra/adapters" ]]; then
    echo "✅ Adapters directory doesn't exist - already clean"
    exit 0
fi

# Remove adapter files
echo "2. Removing adapter files..."

if [[ -f "pkg/mcp/infra/adapters/docker_adapter.go" ]]; then
    rm "pkg/mcp/infra/adapters/docker_adapter.go"
    echo "✅ Removed docker_adapter.go"
fi

if [[ -f "pkg/mcp/infra/adapters/retry_adapter.go" ]]; then
    rm "pkg/mcp/infra/adapters/retry_adapter.go"
    echo "✅ Removed retry_adapter.go"
fi

# Remove adapters directory if empty
if [[ -d "pkg/mcp/infra/adapters" ]]; then
    if [[ -z "$(ls -A pkg/mcp/infra/adapters)" ]]; then
        rmdir "pkg/mcp/infra/adapters"
        echo "✅ Removed empty adapters directory"
    else
        echo "⚠️  Adapters directory not empty - keeping it"
        ls -la pkg/mcp/infra/adapters/
    fi
fi

# Test compilation
echo "3. Testing compilation..."
if go build -v . > /dev/null 2>&1; then
    echo "✅ Compilation successful"
else
    echo "❌ Compilation failed - reverting would be needed"
    exit 1
fi

# Run architecture validation
echo "4. Running architecture validation..."
if [[ -f "scripts/validate-architecture.sh" ]]; then
    if ./scripts/validate-architecture.sh 2>&1 | grep -q "No adapter/wrapper pattern files found"; then
        echo "✅ Architecture validation passed - no adapters detected"
    else
        echo "⚠️  Architecture validation still shows adapters"
        ./scripts/validate-architecture.sh 2>&1 | grep -A 3 -B 3 "adapter"
    fi
else
    echo "⚠️  Architecture validation script not found"
fi

echo ""
echo "🎉 Adapter removal complete!"
echo "Benefits achieved:"
echo "  • Eliminated adapter pattern violations"
echo "  • Reduced code complexity"
echo "  • Improved performance (direct calls)"
echo "  • Better testability"
echo "  • Cleaner three-layer architecture"