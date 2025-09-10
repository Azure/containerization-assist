#!/bin/bash

echo "Converting relative imports to path aliases..."

# Convert domain imports
find src -name "*.ts" -type f -exec sed -i "s|from '\.\./\.\./domain/|from '@domain/|g" {} \;
find src -name "*.ts" -type f -exec sed -i "s|from '\.\./domain/|from '@domain/|g" {} \;

# Convert lib imports
find src -name "*.ts" -type f -exec sed -i "s|from '\.\./\.\./lib/|from '@lib/|g" {} \;
find src -name "*.ts" -type f -exec sed -i "s|from '\.\./lib/|from '@lib/|g" {} \;

# Convert config imports
find src -name "*.ts" -type f -exec sed -i "s|from '\.\./\.\./config/|from '@config/|g" {} \;
find src -name "*.ts" -type f -exec sed -i "s|from '\.\./config/|from '@config/|g" {} \;

# Convert app imports
find src -name "*.ts" -type f -exec sed -i "s|from '\.\./\.\./app/|from '@app/|g" {} \;
find src -name "*.ts" -type f -exec sed -i "s|from '\.\./app/|from '@app/|g" {} \;

# Convert mcp imports
find src -name "*.ts" -type f -exec sed -i "s|from '\.\./\.\./mcp/|from '@mcp/|g" {} \;
find src -name "*.ts" -type f -exec sed -i "s|from '\.\./mcp/|from '@mcp/|g" {} \;

# Convert tools imports
find src -name "*.ts" -type f -exec sed -i "s|from '\.\./\.\./tools/|from '@tools/|g" {} \;
find src -name "*.ts" -type f -exec sed -i "s|from '\.\./tools/|from '@tools/|g" {} \;

# Convert workflows imports
find src -name "*.ts" -type f -exec sed -i "s|from '\.\./\.\./workflows/|from '@workflows/|g" {} \;
find src -name "*.ts" -type f -exec sed -i "s|from '\.\./workflows/|from '@workflows/|g" {} \;

# Convert prompts imports
find src -name "*.ts" -type f -exec sed -i "s|from '\.\./\.\./prompts/|from '@prompts/|g" {} \;
find src -name "*.ts" -type f -exec sed -i "s|from '\.\./prompts/|from '@prompts/|g" {} \;

# Convert resources imports
find src -name "*.ts" -type f -exec sed -i "s|from '\.\./\.\./resources/|from '@resources/|g" {} \;
find src -name "*.ts" -type f -exec sed -i "s|from '\.\./resources/|from '@resources/|g" {} \;

# Special case: @types alias
find src -name "*.ts" -type f -exec sed -i "s|from '@domain/types'|from '@types'|g" {} \;

echo "Conversion complete!"