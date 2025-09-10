#!/bin/bash

echo "Fixing remaining relative imports..."

# Fix imports within tools folder (same-folder imports are OK, but cross-folder should use aliases)
find src/tools -name "*.ts" -type f -exec sed -i "s|from '\.\./analyze-repo'|from '@tools/analyze-repo'|g" {} \;
find src/tools -name "*.ts" -type f -exec sed -i "s|from '\.\./generate-dockerfile'|from '@tools/generate-dockerfile'|g" {} \;
find src/tools -name "*.ts" -type f -exec sed -i "s|from '\.\./build-image'|from '@tools/build-image'|g" {} \;
find src/tools -name "*.ts" -type f -exec sed -i "s|from '\.\./scan'|from '@tools/scan'|g" {} \;
find src/tools -name "*.ts" -type f -exec sed -i "s|from '\.\./push-image'|from '@tools/push-image'|g" {} \;
find src/tools -name "*.ts" -type f -exec sed -i "s|from '\.\./tag-image'|from '@tools/tag-image'|g" {} \;
find src/tools -name "*.ts" -type f -exec sed -i "s|from '\.\./fix-dockerfile'|from '@tools/fix-dockerfile'|g" {} \;
find src/tools -name "*.ts" -type f -exec sed -i "s|from '\.\./resolve-base-images'|from '@tools/resolve-base-images'|g" {} \;
find src/tools -name "*.ts" -type f -exec sed -i "s|from '\.\./prepare-cluster'|from '@tools/prepare-cluster'|g" {} \;
find src/tools -name "*.ts" -type f -exec sed -i "s|from '\.\./deploy'|from '@tools/deploy'|g" {} \;
find src/tools -name "*.ts" -type f -exec sed -i "s|from '\.\./generate-k8s-manifests'|from '@tools/generate-k8s-manifests'|g" {} \;
find src/tools -name "*.ts" -type f -exec sed -i "s|from '\.\./verify-deployment'|from '@tools/verify-deployment'|g" {} \;
find src/tools -name "*.ts" -type f -exec sed -i "s|from '\.\./analysis-perspectives'|from '@tools/analysis-perspectives'|g" {} \;
find src/tools -name "*.ts" -type f -exec sed -i "s|from '\.\./types'|from '@tools/types'|g" {} \;

# Fix imports within workflows folder
find src/workflows -name "*.ts" -type f -exec sed -i "s|from '\.\./containerization-workflow'|from '@workflows/containerization-workflow'|g" {} \;
find src/workflows -name "*.ts" -type f -exec sed -i "s|from '\.\./intelligent-orchestration'|from '@workflows/intelligent-orchestration'|g" {} \;

# Fix imports within mcp folder
find src/mcp -name "*.ts" -type f -exec sed -i "s|from '\.\./core/errors'|from '@mcp/core/errors'|g" {} \;
find src/mcp -name "*.ts" -type f -exec sed -i "s|from '\.\./client/mcp-client'|from '@mcp/client/mcp-client'|g" {} \;

# Fix any remaining cross-module imports with triple-depth
find src -name "*.ts" -type f -exec sed -i "s|from '\.\./\.\./\.\./domain/|from '@domain/|g" {} \;
find src -name "*.ts" -type f -exec sed -i "s|from '\.\./\.\./\.\./lib/|from '@lib/|g" {} \;
find src -name "*.ts" -type f -exec sed -i "s|from '\.\./\.\./\.\./config/|from '@config/|g" {} \;
find src -name "*.ts" -type f -exec sed -i "s|from '\.\./\.\./\.\./mcp/|from '@mcp/|g" {} \;
find src -name "*.ts" -type f -exec sed -i "s|from '\.\./\.\./\.\./tools/|from '@tools/|g" {} \;
find src -name "*.ts" -type f -exec sed -i "s|from '\.\./\.\./\.\./workflows/|from '@workflows/|g" {} \;

# Fix any remaining cross-module imports with quad-depth
find src -name "*.ts" -type f -exec sed -i "s|from '\.\./\.\./\.\./\.\./domain/|from '@domain/|g" {} \;
find src -name "*.ts" -type f -exec sed -i "s|from '\.\./\.\./\.\./\.\./lib/|from '@lib/|g" {} \;
find src -name "*.ts" -type f -exec sed -i "s|from '\.\./\.\./\.\./\.\./config/|from '@config/|g" {} \;

echo "Import paths updated!"