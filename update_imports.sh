#!/bin/bash
set -e

echo "=== UPDATING IMPORT STATEMENTS ==="

# Phase 1: Update API imports
echo "Phase 1: Updating API imports..."
find pkg/mcp -name "*.go" -exec sed -i 's|pkg/mcp/application/api|pkg/mcp/api|g' {} \;

# Phase 2: Update Core imports
echo "Phase 2: Updating Core imports..."
find pkg/mcp -name "*.go" -exec sed -i 's|pkg/mcp/application/core|pkg/mcp/core|g' {} \;
find pkg/mcp -name "*.go" -exec sed -i 's|pkg/mcp/app/registry|pkg/mcp/core/registry|g' {} \;

# Phase 3: Update Tools imports (containerization)
echo "Phase 3: Updating Tools imports..."
find pkg/mcp -name "*.go" -exec sed -i 's|pkg/mcp/domain/containerization/analyze|pkg/mcp/tools/analyze|g' {} \;
find pkg/mcp -name "*.go" -exec sed -i 's|pkg/mcp/domain/containerization/build|pkg/mcp/tools/build|g' {} \;
find pkg/mcp -name "*.go" -exec sed -i 's|pkg/mcp/domain/containerization/deploy|pkg/mcp/tools/deploy|g' {} \;
find pkg/mcp -name "*.go" -exec sed -i 's|pkg/mcp/domain/containerization/scan|pkg/mcp/tools/scan|g' {} \;
find pkg/mcp -name "*.go" -exec sed -i 's|pkg/mcp/domain/containerization|pkg/mcp/tools|g' {} \;

# Phase 4: Update Session imports
echo "Phase 4: Updating Session imports..."
find pkg/mcp -name "*.go" -exec sed -i 's|pkg/mcp/domain/session|pkg/mcp/session|g' {} \;
find pkg/mcp -name "*.go" -exec sed -i 's|pkg/mcp/services/session|pkg/mcp/session|g' {} \;

# Phase 5: Update Workflow imports
echo "Phase 5: Updating Workflow imports..."
find pkg/mcp -name "*.go" -exec sed -i 's|pkg/mcp/application/orchestration/workflow|pkg/mcp/workflow|g' {} \;
find pkg/mcp -name "*.go" -exec sed -i 's|pkg/mcp/services/workflow|pkg/mcp/workflow|g' {} \;
find pkg/mcp -name "*.go" -exec sed -i 's|pkg/mcp/application/workflows|pkg/mcp/workflow|g' {} \;

# Phase 6: Update Infrastructure imports
echo "Phase 6: Updating Infrastructure imports..."
find pkg/mcp -name "*.go" -exec sed -i 's|pkg/mcp/infra/transport|pkg/mcp/transport|g' {} \;
find pkg/mcp -name "*.go" -exec sed -i 's|pkg/mcp/infra/persistence|pkg/mcp/storage|g' {} \;
find pkg/mcp -name "*.go" -exec sed -i 's|pkg/mcp/infra/templates|pkg/mcp/templates|g' {} \;

# Phase 7: Update Security imports
echo "Phase 7: Updating Security imports..."
find pkg/mcp -name "*.go" -exec sed -i 's|pkg/mcp/domain/security|pkg/mcp/security|g' {} \;
find pkg/mcp -name "*.go" -exec sed -i 's|pkg/mcp/domain/validation|pkg/mcp/security/validation|g' {} \;
find pkg/mcp -name "*.go" -exec sed -i 's|pkg/mcp/services/validation|pkg/mcp/security/validation|g' {} \;
find pkg/mcp -name "*.go" -exec sed -i 's|pkg/mcp/services/scanner|pkg/mcp/security/scanner|g' {} \;

# Phase 8: Update Internal imports
echo "Phase 8: Updating Internal imports..."
find pkg/mcp -name "*.go" -exec sed -i 's|pkg/mcp/application/internal|pkg/mcp/internal|g' {} \;
find pkg/mcp -name "*.go" -exec sed -i 's|pkg/mcp/domain/errors|pkg/mcp/internal/errors|g' {} \;
find pkg/mcp -name "*.go" -exec sed -i 's|pkg/mcp/services/errors|pkg/mcp/internal/errors|g' {} \;
find pkg/mcp -name "*.go" -exec sed -i 's|pkg/mcp/domain/types|pkg/mcp/internal/types|g' {} \;
find pkg/mcp -name "*.go" -exec sed -i 's|pkg/mcp/domain/utils|pkg/mcp/internal/utils|g' {} \;
find pkg/mcp -name "*.go" -exec sed -i 's|pkg/mcp/domain/common|pkg/mcp/internal/common|g' {} \;
find pkg/mcp -name "*.go" -exec sed -i 's|pkg/mcp/domain/retry|pkg/mcp/internal/retry|g' {} \;
find pkg/mcp -name "*.go" -exec sed -i 's|pkg/mcp/domain/logging|pkg/mcp/internal/logging|g' {} \;
find pkg/mcp -name "*.go" -exec sed -i 's|pkg/mcp/domain/processing|pkg/mcp/internal/processing|g' {} \;

# Phase 9: Update registry imports
echo "Phase 9: Updating registry imports..."
find pkg/mcp -name "*.go" -exec sed -i 's|pkg/mcp/application/orchestration/registry|pkg/mcp/core/registry|g' {} \;
find pkg/mcp -name "*.go" -exec sed -i 's|pkg/mcp/services/registry|pkg/mcp/core/registry|g' {} \;

echo "Import updates complete"
