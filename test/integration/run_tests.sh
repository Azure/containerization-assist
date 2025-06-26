#!/bin/bash
# Integration test runner for Container Kit MCP Server

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Get the directory of this script
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$( cd "$SCRIPT_DIR/../.." && pwd )"

echo "🧪 Container Kit MCP Integration Tests"
echo "======================================"
echo ""

# Check prerequisites
echo "Checking prerequisites..."

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo -e "${RED}❌ Go is not installed${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Go is installed${NC}"

# Check if Docker is running
if ! docker info &> /dev/null; then
    echo -e "${YELLOW}⚠️  Docker is not running (some tests will be skipped)${NC}"
    DOCKER_AVAILABLE=false
else
    echo -e "${GREEN}✓ Docker is running${NC}"
    DOCKER_AVAILABLE=true
fi

# Check if Kind is installed
if ! command -v kind &> /dev/null; then
    echo -e "${YELLOW}⚠️  Kind is not installed (K8s tests will be skipped)${NC}"
    KIND_AVAILABLE=false
else
    echo -e "${GREEN}✓ Kind is installed${NC}"
    KIND_AVAILABLE=true
fi

echo ""

# Build the MCP server first
echo "Building MCP server..."
cd "$PROJECT_ROOT"
go build -o container-kit-mcp ./cmd/mcp-server
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ MCP server built successfully${NC}"
else
    echo -e "${RED}❌ Failed to build MCP server${NC}"
    exit 1
fi

echo ""

# Run unit tests for MCP package
echo "Running unit tests..."
go test -v ./pkg/mcp/... -short
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Unit tests passed${NC}"
else
    echo -e "${RED}❌ Unit tests failed${NC}"
    exit 1
fi

echo ""

# Run integration tests
echo "Running integration tests..."
cd "$PROJECT_ROOT"

# Set environment variables for tests
export CONTAINER_KIT_TEST_WORKSPACE="/tmp/container-kit-test"
export CONTAINER_KIT_LOG_LEVEL="debug"

# Create test workspace
mkdir -p "$CONTAINER_KIT_TEST_WORKSPACE"

# Run integration tests with appropriate tags
if [ "$DOCKER_AVAILABLE" = true ] && [ "$KIND_AVAILABLE" = true ]; then
    echo "Running full integration test suite..."
    go test -v ./test/integration/... -tags=integration,docker,kind
elif [ "$DOCKER_AVAILABLE" = true ]; then
    echo "Running integration tests without Kind..."
    go test -v ./test/integration/... -tags=integration,docker
else
    echo "Running basic integration tests..."
    go test -v ./test/integration/... -tags=integration
fi

TEST_RESULT=$?

# Cleanup
echo ""
echo "Cleaning up test workspace..."
rm -rf "$CONTAINER_KIT_TEST_WORKSPACE"

if [ $TEST_RESULT -eq 0 ]; then
    echo ""
    echo -e "${GREEN}✅ All tests passed!${NC}"
    echo ""
    echo "Next steps:"
    echo "1. Configure Claude Desktop using test/integration/mcp/claude_desktop_test.md"
    echo "2. Run manual integration tests with Claude Desktop"
    echo "3. Check test coverage: go test -coverprofile=coverage.out ./..."
else
    echo ""
    echo -e "${RED}❌ Some tests failed${NC}"
    exit 1
fi

# Generate test coverage report if requested
if [ "$1" = "--coverage" ]; then
    echo ""
    echo "Generating test coverage report..."
    go test -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html
    echo -e "${GREEN}✓ Coverage report generated: coverage.html${NC}"
fi
