#!/bin/bash
# Enhanced E2E test with runtime validation
set -e

APP_NAME="coolstore"
WORKSPACE_DIR="/tmp/e2e-runtime-test"

log() { echo "[$(date +'%H:%M:%S')] $1"; }
error() { echo "ERROR: $1"; exit 1; }

# Setup
log "Starting enhanced E2E test with runtime validation..."
source ../azure_keys.sh
mkdir -p $WORKSPACE_DIR
cd $WORKSPACE_DIR

# Execute workflow
log "Running containerization workflow..."
timeout 600 mcphost --config ../container-copilot/mcp-test-workspace/mcphost.yml \
    -p "Containerize https://github.com/konveyor-ecosystem/coolstore autonomously." \
    >/dev/null 2>&1 || true

# Basic artifact validation
log "Validating basic artifacts..."
[[ -f "Dockerfile" ]] || error "No Dockerfile generated"
grep -q "openjdk" Dockerfile || error "Not a Java Dockerfile"

# Enhanced: Docker image runtime validation
log "Performing runtime validation of Docker image..."
if docker images | grep -q "$APP_NAME"; then
    log "Testing if built image can actually run..."
    
    # Try to start the container and check if it fails immediately
    CONTAINER_ID=$(docker run -d $APP_NAME:latest 2>/dev/null || echo "FAILED")
    
    if [[ "$CONTAINER_ID" == "FAILED" ]]; then
        error "Container failed to start at all"
    fi
    
    # Wait a few seconds and check container status
    sleep 10
    CONTAINER_STATUS=$(docker ps -a --filter "id=$CONTAINER_ID" --format "{{.Status}}")
    
    if [[ "$CONTAINER_STATUS" == *"Exited"* ]]; then
        log "Container exited - checking logs for failure reasons..."
        CONTAINER_LOGS=$(docker logs $CONTAINER_ID 2>&1)
        
        # Check for specific Java application failures
        if echo "$CONTAINER_LOGS" | grep -q "Unable to access jarfile"; then
            error "RUNTIME VALIDATION FAILED: JAR file not found - this is exactly the coolstore issue!"
        elif echo "$CONTAINER_LOGS" | grep -q "ClassNotFoundException"; then
            error "RUNTIME VALIDATION FAILED: Missing Java classes"
        elif echo "$CONTAINER_LOGS" | grep -q "Exception in thread"; then
            error "RUNTIME VALIDATION FAILED: Application startup exception"
        else
            warn "Container exited but no obvious failure pattern detected"
        fi
    elif [[ "$CONTAINER_STATUS" == *"Up"* ]]; then
        log "✅ Container is running - runtime validation passed!"
    fi
    
    # Cleanup
    docker stop $CONTAINER_ID >/dev/null 2>&1 || true
    docker rm $CONTAINER_ID >/dev/null 2>&1 || true
else
    error "No Docker image built"
fi

# Continue with Kubernetes validation...
log "Validating Kubernetes deployment..."
if kubectl get deployment $APP_NAME >/dev/null 2>&1; then
    # Check if pods are actually running (not just deployed)
    READY_PODS=$(kubectl get deployment $APP_NAME -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
    DESIRED_PODS=$(kubectl get deployment $APP_NAME -o jsonpath='{.spec.replicas}' 2>/dev/null || echo "1")
    
    if [[ "$READY_PODS" == "null" || "$READY_PODS" == "" ]]; then
        READY_PODS=0
    fi
    
    log "Pods ready: $READY_PODS/$DESIRED_PODS"
    
    if [[ "$READY_PODS" -lt "$DESIRED_PODS" ]]; then
        warn "Not all pods are ready - checking pod logs..."
        POD_LOGS=$(kubectl logs deployment/$APP_NAME --tail=20 2>/dev/null || echo "No logs")
        
        if echo "$POD_LOGS" | grep -q "Unable to access jarfile"; then
            error "KUBERNETES VALIDATION FAILED: Same JAR file issue detected in Kubernetes pods"
        fi
    else
        log "✅ All pods are ready"
    fi
else
    error "No Kubernetes deployment found"
fi

# Cleanup
kubectl delete deployment $APP_NAME --ignore-not-found >/dev/null 2>&1 || true
kubectl delete service $APP_NAME-service --ignore-not-found >/dev/null 2>&1 || true
docker rmi $APP_NAME:latest --force >/dev/null 2>&1 || true

log "🎉 Enhanced E2E test completed - runtime validation would have caught the coolstore issue!"
