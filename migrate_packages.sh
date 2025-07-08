#!/bin/bash
set -e

echo "Starting package migration from 86 to 10 packages..."

# Phase 1: Move API and core
echo "Phase 1: API and core packages"
if [ -d "pkg/mcp/application/api" ]; then
    cp -r pkg/mcp/application/api/* pkg/mcp/api/ 2>/dev/null || true
fi
if [ -d "pkg/mcp/application/core" ]; then
    cp -r pkg/mcp/application/core/* pkg/mcp/core/ 2>/dev/null || true
fi

# Phase 2: Move tools (all containerization tools)
echo "Phase 2: Tools (containerization)"
if [ -d "pkg/mcp/domain/containerization" ]; then
    cp -r pkg/mcp/domain/containerization/* pkg/mcp/tools/ 2>/dev/null || true
fi

# Phase 3: Move session management
echo "Phase 3: Session management"
if [ -d "pkg/mcp/domain/session" ]; then
    cp -r pkg/mcp/domain/session/* pkg/mcp/session/ 2>/dev/null || true
fi
if [ -d "pkg/mcp/services/session" ]; then
    cp -r pkg/mcp/services/session/* pkg/mcp/session/ 2>/dev/null || true
fi

# Phase 4: Move workflow
echo "Phase 4: Workflow orchestration"
if [ -d "pkg/mcp/application/orchestration/workflow" ]; then
    cp -r pkg/mcp/application/orchestration/workflow/* pkg/mcp/workflow/ 2>/dev/null || true
fi
if [ -d "pkg/mcp/services/workflow" ]; then
    cp -r pkg/mcp/services/workflow/* pkg/mcp/workflow/ 2>/dev/null || true
fi

# Phase 5: Move infrastructure components
echo "Phase 5: Infrastructure (transport, storage, templates)"
if [ -d "pkg/mcp/infra/transport" ]; then
    cp -r pkg/mcp/infra/transport/* pkg/mcp/transport/ 2>/dev/null || true
fi
if [ -d "pkg/mcp/infra/persistence" ]; then
    cp -r pkg/mcp/infra/persistence/* pkg/mcp/storage/ 2>/dev/null || true
fi
if [ -d "pkg/mcp/infra/templates" ]; then
    cp -r pkg/mcp/infra/templates/* pkg/mcp/templates/ 2>/dev/null || true
fi

# Phase 6: Move security and validation
echo "Phase 6: Security and validation"
if [ -d "pkg/mcp/domain/security" ]; then
    cp -r pkg/mcp/domain/security/* pkg/mcp/security/ 2>/dev/null || true
fi
if [ -d "pkg/mcp/domain/validation" ]; then
    cp -r pkg/mcp/domain/validation/* pkg/mcp/security/ 2>/dev/null || true
fi
if [ -d "pkg/mcp/services/validation" ]; then
    cp -r pkg/mcp/services/validation/* pkg/mcp/security/ 2>/dev/null || true
fi
if [ -d "pkg/mcp/services/scanner" ]; then
    cp -r pkg/mcp/services/scanner/* pkg/mcp/security/ 2>/dev/null || true
fi

# Phase 7: Move internal implementation details
echo "Phase 7: Internal implementation details"
if [ -d "pkg/mcp/application/internal" ]; then
    cp -r pkg/mcp/application/internal/* pkg/mcp/internal/ 2>/dev/null || true
fi

# Phase 8: Handle special cases and remaining files
echo "Phase 8: Special cases and consolidation"
# Move registry implementations to core
if [ -d "pkg/mcp/app/registry" ]; then
    cp -r pkg/mcp/app/registry/* pkg/mcp/core/ 2>/dev/null || true
fi
if [ -d "pkg/mcp/application/orchestration/registry" ]; then
    cp -r pkg/mcp/application/orchestration/registry/* pkg/mcp/core/ 2>/dev/null || true
fi
if [ -d "pkg/mcp/services/registry" ]; then
    cp -r pkg/mcp/services/registry/* pkg/mcp/core/ 2>/dev/null || true
fi

# Move errors to internal
if [ -d "pkg/mcp/domain/errors" ]; then
    mkdir -p pkg/mcp/internal/errors
    cp -r pkg/mcp/domain/errors/* pkg/mcp/internal/errors/ 2>/dev/null || true
fi
if [ -d "pkg/mcp/services/errors" ]; then
    cp -r pkg/mcp/services/errors/* pkg/mcp/internal/errors/ 2>/dev/null || true
fi

# Move types to internal
if [ -d "pkg/mcp/domain/types" ]; then
    mkdir -p pkg/mcp/internal/types
    cp -r pkg/mcp/domain/types/* pkg/mcp/internal/types/ 2>/dev/null || true
fi

# Move utilities to internal
if [ -d "pkg/mcp/domain/utils" ]; then
    mkdir -p pkg/mcp/internal/utils
    cp -r pkg/mcp/domain/utils/* pkg/mcp/internal/utils/ 2>/dev/null || true
fi

echo "Package migration complete"