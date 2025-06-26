#!/bin/bash

# Enable strict error handling
set -e
set -o pipefail

echo "📦 Starting container-copilot demo with PostgreSQL and artifact generation"
echo "----------------------------------------------------------------------------"

# Configuration variables
TARGET_REPO=""  # Will be set from command-line argument
PROJECT_ROOT=".."  # Default project root (parent directory)

# PostgreSQL configuration
POSTGRES_CONTAINER_NAME="postgres"
POSTGRES_USER="todouser"
POSTGRES_PASSWORD="todopass"
POSTGRES_DB="todoapp"
POSTGRES_PORT="5432"

# Check if a target repository was provided
if [[ $# -lt 1 ]]; then
  echo "❌ Error: Target repository path is required"
  echo "Usage: $0 <path-to-target-repository> [project-root-path]"
  echo "  - path-to-target-repository: Path to the repository to analyze"
  echo "  - project-root-path: Optional path to the container-copilot project root (default: ..)"
  exit 1
fi

TARGET_REPO="$1"

# Convert relative paths to absolute paths based on current working directory
if [[ ! "$TARGET_REPO" = /* ]]; then
  TARGET_REPO="$(pwd)/$TARGET_REPO"
  echo "ℹ️ Converted target repository path to absolute: $TARGET_REPO"
fi

# Check if a custom project root was provided
if [[ $# -gt 1 ]]; then
  PROJECT_ROOT="$2"

  # Convert relative project root to absolute path
  if [[ ! "$PROJECT_ROOT" = /* ]]; then
    PROJECT_ROOT="$(pwd)/$PROJECT_ROOT"
    echo "ℹ️ Converted project root path to absolute: $PROJECT_ROOT"
  fi

  # Validate that the project root exists
  if [[ ! -d "$PROJECT_ROOT" ]]; then
    echo "❌ Error: Project root '$PROJECT_ROOT' does not exist or is not a directory"
    exit 1
  fi

  echo "ℹ️ Using custom project root: $PROJECT_ROOT"
else
  # Calculate project root based on script location if not provided
  SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
  PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
  echo "ℹ️ Using calculated project root: $PROJECT_ROOT"
fi

# Validate that the target repository exists
if [[ ! -d "$TARGET_REPO" ]]; then
  echo "❌ Error: Target repository '$TARGET_REPO' does not exist or is not a directory"
  exit 1
fi

# Check if Docker is installed and running
if ! docker info > /dev/null 2>&1; then
  echo "❌ Error: Docker is not running or not installed"
  exit 1
fi

# Ensure host.docker.internal resolves properly (Linux/WSL only)
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
  HOST_INTERNAL_IP="172.17.0.1"

  # Add the IP to loopback interface if not already present
  if ! ip addr show lo | grep -q "$HOST_INTERNAL_IP"; then
    echo "🔧 Adding $HOST_INTERNAL_IP to loopback interface for host.docker.internal support"
    sudo ip addr add $HOST_INTERNAL_IP dev lo
  fi

  # Add to /etc/hosts if not already there
  if ! grep -q "host.docker.internal" /etc/hosts; then
    echo "🔧 Mapping host.docker.internal to $HOST_INTERNAL_IP in /etc/hosts"
    echo "$HOST_INTERNAL_IP host.docker.internal" | sudo tee -a /etc/hosts > /dev/null
  fi
fi

# Check if PostgreSQL container already exists
if docker ps -a --format '{{.Names}}' | grep -q "^${POSTGRES_CONTAINER_NAME}$"; then
  echo "🧹 Cleaning up existing PostgreSQL container"
  docker stop ${POSTGRES_CONTAINER_NAME} > /dev/null 2>&1 || true
  docker rm ${POSTGRES_CONTAINER_NAME} > /dev/null 2>&1 || true
fi

# Launch PostgreSQL container
echo "🚀 Launching PostgreSQL container..."
docker run --name ${POSTGRES_CONTAINER_NAME} \
  -e POSTGRES_USER=${POSTGRES_USER} \
  -e POSTGRES_PASSWORD=${POSTGRES_PASSWORD} \
  -e POSTGRES_DB=${POSTGRES_DB} \
  -p ${POSTGRES_PORT}:5432 \
  -d postgres:14

# Wait for PostgreSQL to initialize
echo "⏳ Waiting for PostgreSQL to initialize..."
sleep 3
max_attempts=15
attempt=1

# More reliable check that includes connection with the specified user
wait_for_postgres() {
  docker exec ${POSTGRES_CONTAINER_NAME} pg_isready -U ${POSTGRES_USER} > /dev/null 2>&1 ||
  docker exec ${POSTGRES_CONTAINER_NAME} pg_isready -U postgres > /dev/null 2>&1
}

while ! wait_for_postgres; do
  if [ $attempt -eq $max_attempts ]; then
    echo "❌ PostgreSQL failed to start after ${max_attempts} attempts"
    echo "   Logs from PostgreSQL container:"
    docker logs ${POSTGRES_CONTAINER_NAME}
    exit 1
  fi
  echo "  Still waiting... (${attempt}/${max_attempts})"
  attempt=$((attempt + 1))
  sleep 2
done

echo "✅ PostgreSQL is up and running"
echo "  - Container: ${POSTGRES_CONTAINER_NAME}"
echo "  - Connection: localhost:${POSTGRES_PORT}"
echo "  - Database: ${POSTGRES_DB}"

echo "✅ Target repository: $TARGET_REPO"
echo ""

# Add trap to clean up resources even if the script exits unexpectedly
cleanup() {
  echo -e "\n🧹 Cleaning up resources..."
  docker stop ${POSTGRES_CONTAINER_NAME} > /dev/null 2>&1 || true
  docker rm ${POSTGRES_CONTAINER_NAME} > /dev/null 2>&1 || true
  echo "✅ Cleanup complete"
}
trap cleanup EXIT

# Run container-copilot on the target repository
echo -e "\n→ Running container‑copilot on '${TARGET_REPO}'..."
echo "----------------------------------------------------------------"
(
  cd "$PROJECT_ROOT"
  go run . generate "$TARGET_REPO" --context "Use 'host.docker.internal' for postgresql host in the manifests."
)

echo "----------------------------------------------------------------"
echo "✅ Demo complete. Generated artifacts for $TARGET_REPO"
