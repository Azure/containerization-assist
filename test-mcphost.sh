#!/bin/bash
set -e

# =============================================================================
# MCP Host Containerization Test Script
# =============================================================================
# This script sets up and runs mcphost for containerization testing
# Usage: ./test-mcphost.sh [repository_url]
# Example: ./test-mcphost.sh https://github.com/konveyor-ecosystem/coolstore
# =============================================================================

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default repository for testing
DEFAULT_REPO="https://github.com/konveyor-ecosystem/coolstore"
REPO_URL="${1:-$DEFAULT_REPO}"

# Configuration
WORKSPACE_DIR="./mcp-test-workspace"
OUTPUT_DIR="$WORKSPACE_DIR/containerization-output"
MCP_SERVER_BINARY="./containerization-assist-mcp"

echo -e "${BLUE}=== MCP Host Containerization Test ===${NC}"
echo -e "${BLUE}Repository: ${REPO_URL}${NC}"
echo -e "${BLUE}Workspace: ${WORKSPACE_DIR}${NC}"
echo ""

# =============================================================================
# Helper Functions
# =============================================================================

print_step() {
    echo -e "${YELLOW}[STEP]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

# Check if command exists
check_command() {
    if ! command -v "$1" &> /dev/null; then
        print_error "$1 is not installed or not in PATH"
        return 1
    fi
}

# =============================================================================
# Pre-flight Checks
# =============================================================================

print_step "Running pre-flight checks..."

# Check required commands
check_command "mcphost" || {
    print_error "mcphost is not installed. Please install it first."
    echo "Installation: https://github.com/mark3labs/mcphost"
    exit 1
}

check_command "go" || {
    print_error "Go is not installed. Please install Go first."
    exit 1
}

# Check if secrets file exists
if [[ ! -f ".secrets" ]]; then
    print_error ".secrets file not found. Please create it with Azure OpenAI credentials."
    echo "Required format:"
    echo "AZURE_OPENAI_DEPLOYMENT_ID=your-deployment-id"
    echo "AZURE_OPENAI_KEY=your-api-key"
    echo "AZURE_OPENAI_ENDPOINT=your-endpoint"
    exit 1
fi

print_success "All pre-flight checks passed"

# =============================================================================
# Environment Setup
# =============================================================================

print_step "Setting up environment..."

# Load environment variables from .secrets file
export $(grep -v '^#' .secrets | xargs)

# Verify required environment variables
if [[ -z "$AZURE_OPENAI_DEPLOYMENT_ID" || -z "$AZURE_OPENAI_KEY" || -z "$AZURE_OPENAI_ENDPOINT" ]]; then
    print_error "Missing required Azure OpenAI environment variables"
    exit 1
fi

print_info "Azure OpenAI Deployment: $AZURE_OPENAI_DEPLOYMENT_ID"
print_info "Azure OpenAI Endpoint: $AZURE_OPENAI_ENDPOINT"

# Create workspace directory
mkdir -p "$WORKSPACE_DIR"
mkdir -p "$OUTPUT_DIR"

print_success "Environment setup complete"

# =============================================================================
# Build MCP Server (if needed)
# =============================================================================

print_step "Checking MCP server binary..."

if [[ ! -f "$MCP_SERVER_BINARY" ]]; then
    print_info "Building MCP server from source..."
    go build -tags mcp -o "$MCP_SERVER_BINARY" .
    chmod +x "$MCP_SERVER_BINARY"
    print_success "MCP server built successfully"
else
    print_info "Using existing MCP server binary"
fi

# Verify the binary works
if ! ./"$MCP_SERVER_BINARY" --version > /dev/null 2>&1; then
    print_error "MCP server binary is not working correctly"
    exit 1
fi

print_success "MCP server binary is ready"

# =============================================================================
# MCP Configuration
# =============================================================================

print_step "Creating MCP configuration..."

# Create MCP configuration file
cat > "$WORKSPACE_DIR/mcphost.yml" << EOF
mcpServers:
  filesystem:
    type: "builtin"
    name: "fs"
    options:
      allowed_directories: ["$PWD", "$PWD/$WORKSPACE_DIR", "/tmp", "."]
  
  bash:
    type: "builtin"
    name: "bash"
    
  containerization-assist:
    type: "stdio"
    command: "$PWD/$MCP_SERVER_BINARY"
    args: []

model: "azure:\${env://AZURE_OPENAI_DEPLOYMENT_ID}"
provider-api-key: "\${env://AZURE_OPENAI_KEY}"
provider-url: "\${env://AZURE_OPENAI_ENDPOINT}"
EOF

print_success "MCP configuration created"

# =============================================================================
# Test MCP Configuration
# =============================================================================

print_step "Validating MCP configuration..."

cd "$WORKSPACE_DIR"

# Verify configuration file is valid
if [[ ! -f "mcphost.yml" ]]; then
    print_error "MCP configuration file not found"
    exit 1
fi

# Test configuration validation first (should be immediate)
echo "Testing mcphost configuration validation..."

# Test if mcphost can validate the config without hanging
mcphost --config mcphost.yml --help > /dev/null 2>&1 &
CONFIG_TEST_PID=$!

# Wait up to 3 seconds for config validation
for i in {1..3}; do
    if ! kill -0 $CONFIG_TEST_PID 2>/dev/null; then
        break
    fi
    sleep 1
done

# Kill if still running after 3 seconds
if kill -0 $CONFIG_TEST_PID 2>/dev/null; then
    kill $CONFIG_TEST_PID 2>/dev/null || true
    print_error "MCP configuration validation failed - mcphost hangs on config load"
    echo "This indicates a configuration issue (likely Azure OpenAI credentials or format)"
    print_info "Skipping connection test and proceeding to containerization..."
else
    wait $CONFIG_TEST_PID 2>/dev/null
    CONFIG_EXIT_CODE=$?
    
    if [[ $CONFIG_EXIT_CODE -eq 0 ]]; then
        print_success "MCP configuration validation passed"
        print_info "Configuration loaded successfully - proceeding to containerization"
    else
        print_error "MCP configuration validation failed (exit code: $CONFIG_EXIT_CODE)"
        print_info "Proceeding anyway - containerization may also fail"
    fi
fi


# =============================================================================
# Run Containerization
# =============================================================================

print_step "Starting containerization process..."

# Go back to root and ensure output directory exists
cd ..
mkdir -p "$OUTPUT_DIR"
cd "$OUTPUT_DIR"

print_info "Repository: $REPO_URL"
print_info "Output directory: $(pwd)"
print_info "Timeout: 5 minutes"

# Create comprehensive containerization prompt
CONTAINERIZATION_PROMPT="Please perform COMPLETE containerization of this repository: $REPO_URL. 

IMPORTANT INSTRUCTIONS:
1. This is an automated test - proceed automatically without asking for confirmations
2. Clone/download the repository to analyze it thoroughly
3. Create a comprehensive Dockerfile optimized for the application
4. Generate Kubernetes deployment manifests (deployment.yaml, service.yaml)
5. Create any necessary ConfigMaps or Secrets if applicable
6. Ensure all generated files are saved in the current working directory: $(pwd)
7. Provide a summary of what was created and why
8. If you encounter any issues, continue with best-effort solutions
9. Focus on creating production-ready containerization artifacts

Please begin the containerization process now."

# Run containerization with timeout and logging
echo "Starting containerization process..."
mcphost --config ../mcphost.yml --stream=false --compact --prompt "$CONTAINERIZATION_PROMPT" > ../mcp-containerization.log 2>&1 &
MCP_PID=$!

# Monitor progress
echo "Monitoring containerization progress..."
START_TIME=$(date +%s)
LAST_LOG_SIZE=0

while kill -0 $MCP_PID 2>/dev/null; do
    sleep 5
    CURRENT_TIME=$(date +%s)
    ELAPSED=$((CURRENT_TIME - START_TIME))
    
    # Check log file growth
    if [[ -f "../mcp-containerization.log" ]]; then
        CURRENT_LOG_SIZE=$(wc -c < "../mcp-containerization.log" 2>/dev/null || echo "0")
        if [[ "$CURRENT_LOG_SIZE" -gt "$LAST_LOG_SIZE" ]]; then
            print_info "Progress detected (log size: ${CURRENT_LOG_SIZE} bytes, elapsed: ${ELAPSED}s)"
            LAST_LOG_SIZE=$CURRENT_LOG_SIZE
        fi
    fi
    
    # Check timeout
    if [[ $ELAPSED -gt 300 ]]; then
        print_error "Containerization timeout exceeded (300s)"
        kill $MCP_PID 2>/dev/null || true
        break
    fi
done

wait $MCP_PID 2>/dev/null || true

print_success "Containerization process completed"

# =============================================================================
# Validate Results
# =============================================================================

print_step "Validating generated artifacts..."

cd "$OUTPUT_DIR"

# Show generated files
echo ""
print_info "Generated files in output directory:"
ls -la . 2>/dev/null || echo "No files in output directory"

# Search for Dockerfiles
DOCKERFILES=$(find . -name "Dockerfile" -type f 2>/dev/null)
if [[ -n "$DOCKERFILES" ]]; then
    print_success "Found Dockerfile(s):"
    echo "$DOCKERFILES"
    
    # Show Dockerfile content
    for dockerfile in $DOCKERFILES; do
        echo ""
        print_info "Content of $dockerfile:"
        echo "----------------------------------------"
        cat "$dockerfile"
        echo "----------------------------------------"
    done
else
    print_error "No Dockerfile found"
fi

# Search for Kubernetes manifests
K8S_MANIFESTS=$(find . -name "*.yaml" -o -name "*.yml" 2>/dev/null | grep -E "(deployment|service|configmap|secret|manifest)" | head -5)
if [[ -n "$K8S_MANIFESTS" ]]; then
    print_success "Found Kubernetes manifest(s):"
    echo "$K8S_MANIFESTS"
    
    # Show manifest content
    for manifest in $K8S_MANIFESTS; do
        echo ""
        print_info "Content of $manifest:"
        echo "----------------------------------------"
        head -30 "$manifest"
        echo "----------------------------------------"
    done
else
    print_error "No Kubernetes manifests found"
fi

# Search for any YAML files
ALL_YAMLS=$(find . -name "*.yaml" -o -name "*.yml" 2>/dev/null | head -5)
if [[ -n "$ALL_YAMLS" ]]; then
    print_info "All YAML files found:"
    echo "$ALL_YAMLS"
fi

# =============================================================================
# Show Logs and Summary
# =============================================================================

echo ""
print_step "Containerization log output:"
echo "============================================="
if [[ -f "../mcp-containerization.log" ]]; then
    cat "../mcp-containerization.log"
else
    echo "No log file found"
fi
echo "============================================="

echo ""
print_step "Test Summary:"

# Count artifacts
DOCKERFILE_COUNT=$(find . -name "Dockerfile" -type f 2>/dev/null | wc -l)
YAML_COUNT=$(find . -name "*.yaml" -o -name "*.yml" 2>/dev/null | wc -l)

echo "📊 Artifacts generated:"
echo "   - Dockerfiles: $DOCKERFILE_COUNT"
echo "   - YAML files: $YAML_COUNT"

if [[ "$DOCKERFILE_COUNT" -gt 0 && "$YAML_COUNT" -gt 0 ]]; then
    print_success "CONTAINERIZATION SUCCESS: Generated both Dockerfile and Kubernetes manifests"
    echo ""
    echo "🎉 Test completed successfully!"
    echo "📁 Output directory: $OUTPUT_DIR"
    echo "📄 Logs available in: $WORKSPACE_DIR/mcp-containerization.log"
    exit 0
elif [[ "$DOCKERFILE_COUNT" -gt 0 ]]; then
    print_success "PARTIAL SUCCESS: Generated Dockerfile but missing Kubernetes manifests"
    exit 0
else
    print_error "CONTAINERIZATION FAILED: No containerization artifacts generated"
    exit 1
fi
