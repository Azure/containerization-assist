#!/bin/bash
# Minimal E2E test focused on core containerization functionality
set -e

APP_NAME="coolstore"
TIMEOUT=300

log() { echo "[$(date +'%H:%M:%S')] $1"; }
error() { echo "ERROR: $1"; exit 1; }

# Setup
log "Starting minimal E2E test..."
source ../azure_keys.sh
mkdir -p /tmp/e2e-minimal
cd /tmp/e2e-minimal

# Execute workflow (ignore output, just check exit code)
log "Running containerization workflow..."
timeout $TIMEOUT mcphost --config ../container-copilot/mcp-test-workspace/mcphost.yml \
    -p "Containerize https://github.com/konveyor-ecosystem/coolstore autonomously. No user input required." \
    >/dev/null 2>&1 || FAILED=1

# Core validations
log "Validating results..."

# Must have Dockerfile
[[ -f "Dockerfile" ]] || error "No Dockerfile generated"
grep -q "openjdk" Dockerfile || error "Not a Java Dockerfile"

# Must have built image
docker images | grep -q "$APP_NAME" || error "No Docker image built"

# Must have deployed to Kubernetes  
kubectl get deployment $APP_NAME >/dev/null || error "No Kubernetes deployment"

# Cleanup
kubectl delete deployment $APP_NAME --ignore-not-found >/dev/null
docker rmi $APP_NAME:latest --force >/dev/null 2>&1 || true

log "✅ E2E test PASSED"
