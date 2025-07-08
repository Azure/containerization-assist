#!/bin/bash
set -e

echo "=== CAREFUL PACKAGE REORGANIZATION ==="

# First, remove the hastily copied files to start fresh
echo "Cleaning up initial migration..."
rm -rf pkg/mcp/api/* pkg/mcp/core/* pkg/mcp/tools/* pkg/mcp/session/*
rm -rf pkg/mcp/workflow/* pkg/mcp/transport/* pkg/mcp/storage/*
rm -rf pkg/mcp/security/* pkg/mcp/templates/* pkg/mcp/internal/*

# Create proper subdirectory structure
echo "Creating proper subdirectory structure..."

# API - Just interfaces
mkdir -p pkg/mcp/api
cp pkg/mcp/application/api/*.go pkg/mcp/api/ 2>/dev/null || true

# Core - Server and registry
mkdir -p pkg/mcp/core
cp pkg/mcp/application/core/*.go pkg/mcp/core/ 2>/dev/null || true
# Move state and types to subdirs
mkdir -p pkg/mcp/core/state
cp -r pkg/mcp/application/core/state/* pkg/mcp/core/state/ 2>/dev/null || true
mkdir -p pkg/mcp/core/types
cp -r pkg/mcp/application/core/types/* pkg/mcp/core/types/ 2>/dev/null || true

# Core registry consolidation
mkdir -p pkg/mcp/core/registry
cp pkg/mcp/app/registry/*.go pkg/mcp/core/registry/ 2>/dev/null || true
cp pkg/mcp/application/orchestration/registry/*.go pkg/mcp/core/registry/ 2>/dev/null || true
cp pkg/mcp/services/registry/*.go pkg/mcp/core/registry/ 2>/dev/null || true

# Tools - All containerization
mkdir -p pkg/mcp/tools/analyze
cp pkg/mcp/domain/containerization/analyze/*.go pkg/mcp/tools/analyze/ 2>/dev/null || true
mkdir -p pkg/mcp/tools/build
cp pkg/mcp/domain/containerization/build/*.go pkg/mcp/tools/build/ 2>/dev/null || true
mkdir -p pkg/mcp/tools/deploy
cp pkg/mcp/domain/containerization/deploy/*.go pkg/mcp/tools/deploy/ 2>/dev/null || true
mkdir -p pkg/mcp/tools/scan
cp pkg/mcp/domain/containerization/scan/*.go pkg/mcp/tools/scan/ 2>/dev/null || true
mkdir -p pkg/mcp/tools/detectors
cp pkg/mcp/domain/containerization/database_detectors/*.go pkg/mcp/tools/detectors/ 2>/dev/null || true

# Session
mkdir -p pkg/mcp/session
cp pkg/mcp/domain/session/*.go pkg/mcp/session/ 2>/dev/null || true
cp pkg/mcp/services/session/*.go pkg/mcp/session/ 2>/dev/null || true
cp pkg/mcp/domain/containerization/session/*.go pkg/mcp/session/ 2>/dev/null || true

# Workflow
mkdir -p pkg/mcp/workflow
cp pkg/mcp/application/orchestration/workflow/*.go pkg/mcp/workflow/ 2>/dev/null || true
cp pkg/mcp/services/workflow/*.go pkg/mcp/workflow/ 2>/dev/null || true
cp -r pkg/mcp/application/workflows/core/* pkg/mcp/workflow/ 2>/dev/null || true

# Transport
mkdir -p pkg/mcp/transport
cp pkg/mcp/infra/transport/*.go pkg/mcp/transport/ 2>/dev/null || true

# Storage
mkdir -p pkg/mcp/storage
cp -r pkg/mcp/infra/persistence/* pkg/mcp/storage/ 2>/dev/null || true

# Security
mkdir -p pkg/mcp/security
cp pkg/mcp/domain/security/*.go pkg/mcp/security/ 2>/dev/null || true
mkdir -p pkg/mcp/security/validation
cp pkg/mcp/domain/validation/*.go pkg/mcp/security/validation/ 2>/dev/null || true
cp -r pkg/mcp/domain/validation/validators pkg/mcp/security/validation/ 2>/dev/null || true
cp pkg/mcp/services/validation/*.go pkg/mcp/security/validation/ 2>/dev/null || true
mkdir -p pkg/mcp/security/scanner
cp pkg/mcp/services/scanner/*.go pkg/mcp/security/scanner/ 2>/dev/null || true

# Templates - preserve structure
mkdir -p pkg/mcp/templates
cp -r pkg/mcp/infra/templates/* pkg/mcp/templates/ 2>/dev/null || true

# Internal - everything else
mkdir -p pkg/mcp/internal
cp -r pkg/mcp/application/internal/* pkg/mcp/internal/ 2>/dev/null || true
mkdir -p pkg/mcp/internal/errors
cp -r pkg/mcp/domain/errors/* pkg/mcp/internal/errors/ 2>/dev/null || true
cp pkg/mcp/services/errors/*.go pkg/mcp/internal/errors/ 2>/dev/null || true
mkdir -p pkg/mcp/internal/types
cp -r pkg/mcp/domain/types/* pkg/mcp/internal/types/ 2>/dev/null || true
mkdir -p pkg/mcp/internal/utils
cp -r pkg/mcp/domain/utils/* pkg/mcp/internal/utils/ 2>/dev/null || true
mkdir -p pkg/mcp/internal/common
cp -r pkg/mcp/domain/common/* pkg/mcp/internal/common/ 2>/dev/null || true
mkdir -p pkg/mcp/internal/retry
cp -r pkg/mcp/domain/retry/* pkg/mcp/internal/retry/ 2>/dev/null || true
mkdir -p pkg/mcp/internal/logging
cp -r pkg/mcp/domain/logging/* pkg/mcp/internal/logging/ 2>/dev/null || true
mkdir -p pkg/mcp/internal/processing
cp -r pkg/mcp/domain/processing/* pkg/mcp/internal/processing/ 2>/dev/null || true

echo "Package reorganization complete"
