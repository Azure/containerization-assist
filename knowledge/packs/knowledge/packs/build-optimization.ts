export default [
  {
    "id": "buildkit-enable-optimizations",
    "category": "build",
    "pattern": "docker build",
    "recommendation": "Enable BuildKit for faster builds with better caching and parallel execution",
    "example": "# Enable BuildKit\nexport DOCKER_BUILDKIT=1\n\n# Or use docker buildx\ndocker buildx build --load -t myapp:latest .\n\n# Dockerfile with BuildKit syntax\n# syntax=docker/dockerfile:1.4\nFROM node:20-alpine\n\n# BuildKit features available\nRUN --mount=type=cache,target=/root/.npm \\\n    npm ci --only=production\n\nCOPY . .\nCMD [\"node\", \"server.js\"]",
    "severity": "high",
    "tags": [
      "build-tool",
      "buildkit",
      "caching",
      "generate-dockerfile",
      "npm-ci",
      "optimization",
      "performance"
    ],
    "description": "BuildKit provides 2-10x faster builds with improved caching and parallelization"
  },
  {
    "id": "buildkit-cache-mount-npm",
    "category": "build",
    "pattern": "npm install|npm ci",
    "recommendation": "Use BuildKit cache mounts for npm to persist node_modules cache across builds",
    "example": "# syntax=docker/dockerfile:1.4\nFROM node:20-alpine\n\nWORKDIR /app\nCOPY package*.json ./\n\n# Cache npm packages across builds\nRUN --mount=type=cache,target=/root/.npm \\\n    --mount=type=cache,target=/app/node_modules \\\n    npm ci --only=production\n\nCOPY . .\nUSER node\nCMD [\"node\", \"server.js\"]",
    "severity": "high",
    "tags": [
      "build-tool",
      "buildkit",
      "cache",
      "generate-dockerfile",
      "nodejs",
      "npm",
      "npm-ci",
      "optimization"
    ],
    "description": "Cache mounts can reduce npm install time from minutes to seconds on subsequent builds"
  },
  {
    "id": "buildkit-cache-mount-pip",
    "category": "build",
    "pattern": "pip install",
    "recommendation": "Use BuildKit cache mounts for pip to cache Python packages",
    "example": "# syntax=docker/dockerfile:1.4\nFROM python:3.11-slim\n\nWORKDIR /app\nCOPY requirements.txt .\n\n# Cache pip packages\nRUN --mount=type=cache,target=/root/.cache/pip \\\n    pip install --no-cache-dir -r requirements.txt\n\nCOPY . .\nUSER 1001\nCMD [\"python\", \"app.py\"]",
    "severity": "high",
    "tags": [
      "build-tool",
      "buildkit",
      "cache",
      "generate-dockerfile",
      "optimization",
      "pip",
      "pip-install",
      "python"
    ],
    "description": "Pip cache mounts dramatically speed up Python dependency installation"
  },
  {
    "id": "buildkit-cache-mount-go",
    "category": "build",
    "pattern": "go build|go mod download",
    "recommendation": "Use BuildKit cache mounts for Go module and build caches",
    "example": "# syntax=docker/dockerfile:1.4\nFROM golang:1.21-alpine AS builder\n\nWORKDIR /app\nCOPY go.mod go.sum ./\n\n# Cache Go modules\nRUN --mount=type=cache,target=/go/pkg/mod \\\n    go mod download\n\nCOPY . .\n\n# Cache build artifacts\nRUN --mount=type=cache,target=/go/pkg/mod \\\n    --mount=type=cache,target=/root/.cache/go-build \\\n    CGO_ENABLED=0 go build -o main .\n\nFROM scratch\nCOPY --from=builder /app/main /main\nENTRYPOINT [\"/main\"]",
    "severity": "high",
    "tags": [
      "build-tool",
      "buildkit",
      "cache",
      "generate-dockerfile",
      "go-mod",
      "go-modules",
      "golang",
      "optimization"
    ],
    "description": "Go cache mounts can reduce build time by 80% for large projects"
  },
  {
    "id": "buildkit-cache-mount-maven",
    "category": "build",
    "pattern": "mvn",
    "recommendation": "Use BuildKit cache mounts for Maven local repository",
    "example": "# syntax=docker/dockerfile:1.4\nFROM maven:3.9-eclipse-temurin-17 AS builder\n\nWORKDIR /app\nCOPY pom.xml .\n\n# Cache Maven dependencies\nRUN --mount=type=cache,target=/root/.m2 \\\n    mvn dependency:go-offline\n\nCOPY src ./src\n\n# Build with cached dependencies\nRUN --mount=type=cache,target=/root/.m2 \\\n    mvn package -DskipTests\n\nFROM eclipse-temurin:17-jre-alpine\nCOPY --from=builder /app/target/app.jar /app.jar\nUSER 1001\nENTRYPOINT [\"java\", \"-jar\", \"/app.jar\"]",
    "severity": "high",
    "tags": [
      "build-tool",
      "buildkit",
      "cache",
      "generate-dockerfile",
      "java",
      "maven",
      "optimization"
    ],
    "description": "Maven cache mounts avoid re-downloading dependencies on every build"
  },
  {
    "id": "buildkit-parallel-stages",
    "category": "build",
    "pattern": "FROM.*AS",
    "recommendation": "Structure multi-stage builds to enable parallel stage execution",
    "example": "# syntax=docker/dockerfile:1.4\n\n# Base stage\nFROM node:20-alpine AS base\nWORKDIR /app\nCOPY package*.json ./\n\n# Build frontend (parallel stage 1)\nFROM base AS build-frontend\nCOPY frontend/ ./frontend/\nRUN --mount=type=cache,target=/root/.npm \\\n    cd frontend && npm ci && npm run build\n\n# Build backend (parallel stage 2)\nFROM base AS build-backend\nCOPY backend/ ./backend/\nRUN --mount=type=cache,target=/root/.npm \\\n    cd backend && npm ci && npm run build\n\n# Final stage combines both\nFROM node:20-alpine\nWORKDIR /app\nCOPY --from=build-frontend /app/frontend/dist ./public\nCOPY --from=build-backend /app/backend/dist ./dist\nUSER node\nCMD [\"node\", \"dist/server.js\"]",
    "severity": "medium",
    "tags": [
      "build-stage",
      "build-tool",
      "buildkit",
      "generate-dockerfile",
      "multistage",
      "npm-ci",
      "optimization",
      "parallel",
      "runtime-stage"
    ],
    "description": "Parallel stages can reduce total build time by 30-50% for complex builds"
  },
  {
    "id": "buildkit-secret-mount-credentials",
    "category": "build",
    "pattern": "RUN.*--mount=type=secret",
    "recommendation": "Use BuildKit secret mounts to access credentials during build without exposing them",
    "example": "# syntax=docker/dockerfile:1.4\nFROM node:20-alpine\n\nWORKDIR /app\nCOPY package*.json ./\n\n# Use secret for private npm registry\nRUN --mount=type=secret,id=npmrc,target=/root/.npmrc \\\n    npm ci --only=production\n\nCOPY . .\nUSER node\nCMD [\"node\", \"server.js\"]\n\n# Build with secret:\n# docker build --secret id=npmrc,src=$HOME/.npmrc -t myapp .\n# Or from environment:\n# echo \"$NPM_TOKEN\" | docker build --secret id=npmrc,src=/dev/stdin -t myapp .",
    "severity": "high",
    "tags": [
      "aws",
      "build-tool",
      "buildkit",
      "credentials",
      "generate-dockerfile",
      "npm-ci",
      "secrets",
      "security"
    ],
    "description": "Secret mounts prevent credentials from being stored in image layers"
  },
  {
    "id": "buildkit-ssh-mount-git",
    "category": "build",
    "pattern": "RUN.*--mount=type=ssh",
    "recommendation": "Use BuildKit SSH mounts to access private Git repositories during build",
    "example": "# syntax=docker/dockerfile:1.4\nFROM node:20-alpine\n\nRUN apk add --no-cache git openssh-client\n\nWORKDIR /app\nCOPY package*.json ./\n\n# Use SSH for private Git dependencies\nRUN --mount=type=ssh \\\n    --mount=type=cache,target=/root/.npm \\\n    mkdir -p -m 0600 ~/.ssh && \\\n    ssh-keyscan github.com >> ~/.ssh/known_hosts && \\\n    npm ci --only=production\n\nCOPY . .\nUSER node\nCMD [\"node\", \"server.js\"]\n\n# Build with SSH agent:\n# docker build --ssh default -t myapp .",
    "severity": "medium",
    "tags": [
      "build-tool",
      "buildkit",
      "generate-dockerfile",
      "git",
      "npm-ci",
      "optimization",
      "private-repos",
      "ssh"
    ],
    "description": "SSH mounts enable access to private repos without embedding keys in image"
  },
  {
    "id": "build-arg-cache-optimization",
    "category": "build",
    "pattern": "ARG",
    "recommendation": "Place ARG instructions after COPY to avoid cache invalidation on version changes",
    "example": "FROM node:20-alpine\n\n# Place stable instructions first\nWORKDIR /app\nCOPY package*.json ./\nRUN npm ci --only=production\nCOPY . .\n\n# ARG late in Dockerfile to preserve cache\nARG BUILD_VERSION=dev\nARG BUILD_DATE\nARG VCS_REF\n\n# Use ARGs in labels (doesn't affect cache much)\nLABEL version=\"${BUILD_VERSION}\" \\\n      build-date=\"${BUILD_DATE}\" \\\n      vcs-ref=\"${VCS_REF}\"\n\nUSER node\nCMD [\"node\", \"server.js\"]",
    "severity": "medium",
    "tags": [
      "build-args",
      "build-tool",
      "caching",
      "generate-dockerfile",
      "layer-ordering",
      "npm-ci",
      "optimization"
    ],
    "description": "Late ARG placement prevents cache invalidation on metadata-only changes"
  },
  {
    "id": "copy-optimization-specific-files",
    "category": "build",
    "pattern": "COPY",
    "recommendation": "Copy only necessary files and use specific COPY commands instead of COPY . .",
    "example": "FROM node:20-alpine\n\nWORKDIR /app\n\n# Copy dependency files first\nCOPY package.json package-lock.json ./\nRUN npm ci --only=production\n\n# Copy only source code (not node_modules, tests, docs)\nCOPY src/ ./src/\nCOPY public/ ./public/\nCOPY config/ ./config/\n\n# Avoid: COPY . .\n# Use .dockerignore instead\n\nUSER node\nCMD [\"node\", \"src/server.js\"]",
    "severity": "medium",
    "tags": [
      "build-context",
      "build-tool",
      "copy",
      "generate-dockerfile",
      "npm-ci",
      "optimization",
      "selective"
    ],
    "description": "Specific COPY commands create better cache boundaries and smaller build context"
  },
  {
    "id": "layer-squashing-single-run",
    "category": "build",
    "pattern": "RUN",
    "recommendation": "Combine related RUN commands with && to reduce layer count and image size",
    "example": "FROM ubuntu:22.04\n\n# Bad: Multiple layers\n# RUN apt-get update\n# RUN apt-get install -y curl\n# RUN apt-get install -y vim\n# RUN rm -rf /var/lib/apt/lists/*\n\n# Good: Single layer with proper cleanup\nRUN apt-get update && \\\n    apt-get install -y --no-install-recommends \\\n        curl \\\n        vim \\\n        ca-certificates \\\n    && rm -rf /var/lib/apt/lists/* \\\n    && apt-get clean\n\nWORKDIR /app\nCOPY . .\nCMD [\"./app\"]",
    "severity": "high",
    "tags": [
      "build-tool",
      "canonical",
      "generate-dockerfile",
      "layers",
      "optimization",
      "run-commands",
      "size-reduction"
    ],
    "description": "Combined RUN commands reduce image size by 20-40% and layer count"
  },
  {
    "id": "buildkit-inline-cache",
    "category": "build",
    "pattern": "docker build",
    "recommendation": "Use inline cache to enable cache reuse across different build environments",
    "example": "# Build with inline cache export\ndocker buildx build \\\n  --cache-to type=inline \\\n  --cache-from type=registry,ref=myregistry/myapp:latest \\\n  -t myregistry/myapp:latest \\\n  --push \\\n  .\n\n# Subsequent builds use cache from registry\ndocker buildx build \\\n  --cache-from type=registry,ref=myregistry/myapp:latest \\\n  -t myregistry/myapp:v2 \\\n  --push \\\n  .\n\n# Or use local cache\ndocker buildx build \\\n  --cache-to type=local,dest=/tmp/cache \\\n  --cache-from type=local,src=/tmp/cache \\\n  -t myapp:latest \\\n  .",
    "severity": "high",
    "tags": [
      "build-tool",
      "buildkit",
      "cache",
      "ci-cd",
      "generate-dockerfile",
      "inline-cache",
      "optimization"
    ],
    "description": "Inline cache enables cache sharing in CI/CD where layer cache isn't persisted"
  },
  {
    "id": "buildkit-registry-cache",
    "category": "build",
    "pattern": "docker buildx",
    "recommendation": "Use registry cache for distributed builds across multiple machines",
    "example": "# Build with registry cache\ndocker buildx build \\\n  --cache-from type=registry,ref=myregistry/myapp:buildcache \\\n  --cache-to type=registry,ref=myregistry/myapp:buildcache,mode=max \\\n  -t myregistry/myapp:latest \\\n  --push \\\n  .\n\n# mode=max exports all layers (not just final image)\n# mode=min exports only final image layers\n\n# Subsequent builds reuse cache\ndocker buildx build \\\n  --cache-from type=registry,ref=myregistry/myapp:buildcache \\\n  -t myregistry/myapp:v2 \\\n  --push \\\n  .",
    "severity": "high",
    "tags": [
      "build-tool",
      "buildkit",
      "ci-cd",
      "distributed-builds",
      "generate-dockerfile",
      "registry-cache"
    ],
    "description": "Registry cache enables cache sharing across CI runners and developers"
  },
  {
    "id": "multi-platform-builds",
    "category": "build",
    "pattern": "docker buildx",
    "recommendation": "Build multi-platform images (amd64, arm64) in single command using buildx",
    "example": "# Create builder if needed\ndocker buildx create --name multiplatform --use\n\n# Build for multiple platforms\ndocker buildx build \\\n  --platform linux/amd64,linux/arm64,linux/arm/v7 \\\n  -t myregistry/myapp:latest \\\n  --push \\\n  .\n\n# With cache\ndocker buildx build \\\n  --platform linux/amd64,linux/arm64 \\\n  --cache-from type=registry,ref=myregistry/myapp:buildcache \\\n  --cache-to type=registry,ref=myregistry/myapp:buildcache,mode=max \\\n  -t myregistry/myapp:latest \\\n  --push \\\n  .",
    "severity": "medium",
    "tags": [
      "amd64",
      "arm64",
      "build-tool",
      "buildx",
      "cross-platform",
      "generate-dockerfile",
      "multi-platform"
    ],
    "description": "Multi-platform builds support ARM-based servers and Apple Silicon development"
  },
  {
    "id": "buildx-bake-orchestration",
    "category": "build",
    "pattern": "docker-bake.hcl",
    "recommendation": "Use docker buildx bake for complex build orchestration with multiple targets",
    "example": "# docker-bake.hcl\ngroup \"default\" {\n  targets = [\"frontend\", \"backend\"]\n}\n\ntarget \"frontend\" {\n  context = \"./frontend\"\n  dockerfile = \"Dockerfile\"\n  tags = [\"myapp/frontend:latest\"]\n  cache-from = [\"type=registry,ref=myapp/frontend:buildcache\"]\n  cache-to = [\"type=registry,ref=myapp/frontend:buildcache,mode=max\"]\n  platforms = [\"linux/amd64\", \"linux/arm64\"]\n}\n\ntarget \"backend\" {\n  context = \"./backend\"\n  dockerfile = \"Dockerfile\"\n  tags = [\"myapp/backend:latest\"]\n  cache-from = [\"type=registry,ref=myapp/backend:buildcache\"]\n  cache-to = [\"type=registry,ref=myapp/backend:buildcache,mode=max\"]\n  platforms = [\"linux/amd64\", \"linux/arm64\"]\n}\n\n# Build all targets\n# docker buildx bake --push",
    "severity": "medium",
    "tags": [
      "automation",
      "bake",
      "build-tool",
      "buildx",
      "generate-dockerfile",
      "multi-target",
      "orchestration"
    ],
    "description": "Buildx bake simplifies building multiple related images with shared configuration"
  },
  {
    "id": "build-context-optimization",
    "category": "build",
    "pattern": "docker build",
    "recommendation": "Reduce build context size by using .dockerignore to exclude unnecessary files",
    "example": "# .dockerignore\n# Version control\n.git\n.gitignore\n.gitattributes\n\n# Dependencies (already in container)\nnode_modules\nvendor\ntarget\n__pycache__\n*.pyc\n\n# Build artifacts\ndist\nbuild\n*.o\n*.so\n\n# IDE and editor files\n.vscode\n.idea\n.DS_Store\n*.swp\n*.swo\n\n# Documentation\nREADME.md\nDOCS.md\ndocs/\n\n# Tests\ntests/\ntest/\n*.test.js\n*.spec.js\n\n# CI/CD\n.github\n.gitlab-ci.yml\nJenkinsfile\n.circleci\n\n# Logs\n*.log\nlogs/\n\n# Docker files\nDockerfile*\ndocker-compose*.yml\n.dockerignore",
    "severity": "high",
    "tags": [
      "build-context",
      "build-tool",
      "dockerignore",
      "generate-dockerfile",
      "optimization",
      "performance"
    ],
    "description": "Smaller build context can reduce build time by 50-90% for large projects"
  },
  {
    "id": "npm-ci-vs-install",
    "category": "build",
    "pattern": "npm install",
    "recommendation": "Use npm ci instead of npm install for faster, more reliable builds",
    "example": "FROM node:20-alpine\n\nWORKDIR /app\n\n# Copy lock file for deterministic builds\nCOPY package.json package-lock.json ./\n\n# Use npm ci (clean install)\n# - Faster than npm install\n# - Requires package-lock.json\n# - Removes node_modules first\n# - Never modifies package-lock.json\nRUN npm ci --only=production && \\\n    npm cache clean --force\n\n# Don't use: npm install\n# npm install can modify package-lock.json and is slower\n\nCOPY . .\nUSER node\nCMD [\"node\", \"server.js\"]",
    "severity": "high",
    "tags": [
      "build-tool",
      "deterministic",
      "generate-dockerfile",
      "nodejs",
      "npm",
      "npm-ci",
      "optimization"
    ],
    "description": "npm ci is 2-10x faster than npm install and ensures reproducible builds"
  },
  {
    "id": "go-mod-download-cache",
    "category": "build",
    "pattern": "go mod download",
    "recommendation": "Download Go modules separately from build to leverage layer caching",
    "example": "FROM golang:1.21-alpine AS builder\n\nWORKDIR /app\n\n# Copy go.mod and go.sum first\nCOPY go.mod go.sum ./\n\n# Download dependencies (cached layer)\nRUN go mod download && \\\n    go mod verify\n\n# Copy source code\nCOPY . .\n\n# Build (only rebuilds if source changes)\nRUN CGO_ENABLED=0 GOOS=linux go build \\\n    -ldflags='-w -s' \\\n    -o /app/main .\n\nFROM scratch\nCOPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/\nCOPY --from=builder /app/main /main\nUSER 65534\nENTRYPOINT [\"/main\"]",
    "severity": "high",
    "tags": [
      "build-tool",
      "caching",
      "generate-dockerfile",
      "go-mod",
      "golang",
      "optimization"
    ],
    "description": "Separate go mod download creates cacheable layer, avoiding re-download on code changes"
  },
  {
    "id": "pip-wheel-build-cache",
    "category": "build",
    "pattern": "pip install",
    "recommendation": "Build Python wheels first to cache compiled dependencies separately",
    "example": "FROM python:3.11-slim AS builder\n\nWORKDIR /app\n\n# Install build dependencies\nRUN apt-get update && \\\n    apt-get install -y --no-install-recommends gcc && \\\n    rm -rf /var/lib/apt/lists/*\n\n# Copy requirements and build wheels\nCOPY requirements.txt .\nRUN pip wheel --no-cache-dir --no-deps --wheel-dir /wheels -r requirements.txt\n\n# Runtime stage\nFROM python:3.11-slim\n\nWORKDIR /app\n\n# Install from pre-built wheels (no compilation)\nCOPY --from=builder /wheels /wheels\nCOPY requirements.txt .\nRUN pip install --no-cache-dir --no-index --find-links=/wheels -r requirements.txt && \\\n    rm -rf /wheels\n\nCOPY . .\nUSER 1001\nCMD [\"python\", \"app.py\"]",
    "severity": "medium",
    "tags": [
      "build-tool",
      "caching",
      "generate-dockerfile",
      "optimization",
      "pip",
      "pip-install",
      "python",
      "wheels"
    ],
    "description": "Pre-built wheels eliminate compilation in final stage, reducing build time and image size"
  },
  {
    "id": "maven-dependency-caching",
    "category": "build",
    "pattern": "mvn",
    "recommendation": "Copy pom.xml first and run dependency resolution separately for better caching",
    "example": "FROM maven:3.9-eclipse-temurin-17 AS builder\n\nWORKDIR /app\n\n# Copy pom.xml first (changes infrequently)\nCOPY pom.xml .\n\n# Download dependencies (cached layer)\nRUN mvn dependency:go-offline -B\n\n# Copy source code (changes frequently)\nCOPY src ./src\n\n# Build (only rebuilds if source changes)\nRUN mvn package -DskipTests -B\n\nFROM eclipse-temurin:17-jre-alpine\nWORKDIR /app\nCOPY --from=builder /app/target/*.jar app.jar\nUSER 1001\nENTRYPOINT [\"java\", \"-jar\", \"app.jar\"]",
    "severity": "high",
    "tags": [
      "build-tool",
      "caching",
      "dependencies",
      "generate-dockerfile",
      "java",
      "maven",
      "optimization"
    ],
    "description": "Separate dependency download creates cacheable layer, avoiding re-download on code changes"
  }
] as const;
