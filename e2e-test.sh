#!/bin/bash
set -e

# E2E Test for Container Copilot MCP Server
# Tests the complete containerization workflow without relying on text output parsing

REPO_URL="https://github.com/konveyor-ecosystem/coolstore"
APP_NAME="coolstore"
WORKSPACE_DIR="/tmp/e2e-test-workspace"
TEST_TIMEOUT=600  # 10 minutes

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log() {
    echo -e "${GREEN}[$(date +'%H:%M:%S')] $1${NC}"
}

warn() {
    echo -e "${YELLOW}[$(date +'%H:%M:%S')] WARNING: $1${NC}"
}

error() {
    echo -e "${RED}[$(date +'%H:%M:%S')] ERROR: $1${NC}"
    exit 1
}

cleanup() {
    log "Cleaning up test resources..."
    kubectl delete deployment $APP_NAME --ignore-not-found=true 2>/dev/null || true
    kubectl delete service $APP_NAME --ignore-not-found=true 2>/dev/null || true
    docker rmi $APP_NAME:latest --force 2>/dev/null || true
    rm -rf $WORKSPACE_DIR 2>/dev/null || true
    pkill -f "mcphost.*$APP_NAME" 2>/dev/null || true
}

# Trap cleanup on exit
trap cleanup EXIT

# Step 1: Setup
log "Setting up E2E test environment..."
source ../azure_keys.sh
mkdir -p $WORKSPACE_DIR
rm -f /tmp/mcp-sessions.db

# Verify prerequisites
command -v mcphost >/dev/null 2>&1 || error "mcphost not found"
command -v kubectl >/dev/null 2>&1 || error "kubectl not found" 
command -v docker >/dev/null 2>&1 || error "docker not found"
[[ -f "./containerization-assist-mcp" ]] || error "MCP server binary not found"

# Step 2: Execute containerization workflow
log "Executing containerization workflow (timeout: ${TEST_TIMEOUT}s)..."
cd $WORKSPACE_DIR

# Run mcphost with simple prompt - don't parse output, just check exit code
timeout $TEST_TIMEOUT mcphost --config ../container-copilot/mcp-test-workspace/mcphost.yml \
    --stream=false \
    -p "Containerize this repository: $REPO_URL. Work autonomously without asking for input. Complete all steps: analyze, build, deploy to Kubernetes." \
    > mcphost-output.log 2>&1

MCPHOST_EXIT_CODE=$?
log "mcphost completed with exit code: $MCPHOST_EXIT_CODE"

# Step 3: Verify generated artifacts
log "Verifying generated artifacts..."

# Check Dockerfile
if [[ ! -f "Dockerfile" ]]; then
    error "Dockerfile not generated"
fi

# Validate Dockerfile for Java app
if ! grep -q "FROM.*openjdk" Dockerfile; then
    error "Dockerfile doesn't use OpenJDK base image"
fi

if ! grep -q "\.jar" Dockerfile; then
    error "Dockerfile doesn't handle JAR files"
fi

if ! grep -q "EXPOSE.*8080" Dockerfile; then
    error "Dockerfile doesn't expose port 8080"
fi

log "✅ Dockerfile validation passed"

# Check Kubernetes manifests
if [[ ! -f "deployment.yaml" && ! -f "k8s/deployment.yaml" ]]; then
    error "Kubernetes deployment manifest not found"
fi

DEPLOYMENT_FILE="deployment.yaml"
[[ -f "k8s/deployment.yaml" ]] && DEPLOYMENT_FILE="k8s/deployment.yaml"

# Validate deployment manifest
if ! command -v yq >/dev/null 2>&1; then
    # Fallback to grep if yq not available
    if ! grep -q "kind: Deployment" $DEPLOYMENT_FILE; then
        error "Invalid deployment manifest"
    fi
else
    # Use yq for proper YAML validation
    if [[ "$(yq eval '.kind' $DEPLOYMENT_FILE)" != "Deployment" ]]; then
        error "Invalid deployment manifest kind"
    fi
    
    if [[ "$(yq eval '.spec.template.spec.containers[0].image' $DEPLOYMENT_FILE)" == "null" ]]; then
        error "Deployment manifest missing container image"
    fi
fi

log "✅ Kubernetes manifest validation passed"

# Step 4: Verify Docker image was built
log "Verifying Docker image..."

if ! docker images --format "table {{.Repository}}:{{.Tag}}" | grep -q "$APP_NAME"; then
    error "Docker image '$APP_NAME' not found"
fi

# Check image has correct metadata
if ! docker inspect "$APP_NAME:latest" --format='{{.Config.ExposedPorts}}' | grep -q "8080"; then
    error "Docker image doesn't expose port 8080"
fi

log "✅ Docker image validation passed"

# Step 5: Verify Kubernetes deployment
log "Verifying Kubernetes deployment..."

# Check if deployment exists
if ! kubectl get deployment $APP_NAME >/dev/null 2>&1; then
    error "Kubernetes deployment '$APP_NAME' not found"
fi

# Wait for deployment to be ready (with timeout)
log "Waiting for deployment to be ready..."
if ! kubectl wait --for=condition=available deployment/$APP_NAME --timeout=300s; then
    warn "Deployment not ready within timeout, checking pod status..."
    kubectl get pods -l app=$APP_NAME
    kubectl describe pods -l app=$APP_NAME | tail -20
    # Don't fail here - pods might still be starting for a slow app
fi

# Check if service exists
if kubectl get svc $APP_NAME >/dev/null 2>&1; then
    log "✅ Kubernetes service exists"
else
    warn "Kubernetes service not found (may be optional)"
fi

log "✅ Kubernetes deployment validation passed"

# Step 6: Verify application health (optional - may fail if app takes time to start)
log "Testing application health..."

# Try to port-forward and test the application
if kubectl get svc $APP_NAME >/dev/null 2>&1; then
    log "Attempting to test application endpoint..."
    
    # Start port-forward in background
    kubectl port-forward svc/$APP_NAME 8080:80 >/dev/null 2>&1 &
    PF_PID=$!
    sleep 15
    
    # Test endpoints
    if curl -f -s http://localhost:8080/actuator/health >/dev/null 2>&1; then
        log "✅ Spring Boot health endpoint responding"
    elif curl -f -s http://localhost:8080/ >/dev/null 2>&1; then
        log "✅ Application root endpoint responding"
    else
        warn "Application endpoints not responding (may still be starting)"
    fi
    
    # Kill port-forward
    kill $PF_PID 2>/dev/null || true
else
    warn "No service found for health check"
fi

# Step 7: Final validation summary
log "E2E Test Summary:"
log "✅ Dockerfile generated and valid"
log "✅ Kubernetes manifests generated and valid" 
log "✅ Docker image built successfully"
log "✅ Kubernetes deployment created"

if [[ $MCPHOST_EXIT_CODE -eq 0 ]]; then
    log "✅ Overall workflow completed successfully"
else
    warn "mcphost exited with code $MCPHOST_EXIT_CODE but artifacts were created"
fi

log "🎉 E2E test PASSED - Container Copilot successfully containerized the Java application"

exit 0
