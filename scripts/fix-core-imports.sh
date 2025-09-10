#!/bin/bash

# Fix imports from core/types to domain/types
find src test -name "*.ts" -type f -exec sed -i "s|from '\.\./core/types'|from '../domain/types'|g" {} \;
find src test -name "*.ts" -type f -exec sed -i "s|from '\.\./\.\./core/types'|from '../../domain/types'|g" {} \;
find src test -name "*.ts" -type f -exec sed -i "s|from '\.\./\.\./\.\./core/types'|from '../../../domain/types'|g" {} \;
find src test -name "*.ts" -type f -exec sed -i "s|from '\.\./\.\./\.\./\.\./core/types'|from '../../../../domain/types'|g" {} \;
find src test -name "*.ts" -type f -exec sed -i "s|from '\.\./\.\./\.\./\.\./\.\./core/types'|from '../../../../../domain/types'|g" {} \;

# Fix imports from config/validation to domain/validators
find src test -name "*.ts" -type f -exec sed -i "s|from '\.\./config/validation'|from '../domain/validators'|g" {} \;
find src test -name "*.ts" -type f -exec sed -i "s|from '\.\./\.\./config/validation'|from '../../domain/validators'|g" {} \;
find src test -name "*.ts" -type f -exec sed -i "s|from '\.\./\.\./\.\./config/validation'|from '../../../domain/validators'|g" {} \;

# Fix imports from mcp/prompts to prompts
find src test -name "*.ts" -type f -exec sed -i "s|from '\.\./mcp/prompts/prompt-registry'|from '../prompts/prompt-registry'|g" {} \;
find src test -name "*.ts" -type f -exec sed -i "s|from '\.\./\.\./mcp/prompts/prompt-registry'|from '../../prompts/prompt-registry'|g" {} \;
find src test -name "*.ts" -type f -exec sed -i "s|from '\.\./prompts/prompt-registry'|from '../prompts/prompt-registry'|g" {} \;

# Fix imports from mcp/resources to resources
find src test -name "*.ts" -type f -exec sed -i "s|from '\.\./mcp/resources/|from '../resources/|g" {} \;
find src test -name "*.ts" -type f -exec sed -i "s|from '\.\./\.\./mcp/resources/|from '../../resources/|g" {} \;
find src test -name "*.ts" -type f -exec sed -i "s|from '\.\./resources/|from '../resources/|g" {} \;

echo "Import paths updated"