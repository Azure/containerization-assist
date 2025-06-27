#!/bin/bash

# Utility Consolidation Script
# This script helps identify and consolidate duplicate utility functions

echo "=== Utility Function Consolidation Report ==="
echo ""

echo "1. String Manipulation Function Duplicates:"
echo "   toSnakeCase implementations found in:"
grep -r "func.*toSnakeCase" pkg/mcp --include="*.go" | head -5

echo ""
echo "2. Error Handling Function Duplicates:"
echo "   WrapError implementations found in:"
grep -r "func.*WrapError" pkg/mcp --include="*.go" | head -5

echo ""
echo "3. Validation Function Duplicates:"
echo "   Validation-related functions found in:"
grep -r "func.*Valid" pkg/mcp --include="*.go" | wc -l
echo "   validation functions found across the codebase"

echo ""
echo "4. File System Function Duplicates:"
echo "   File existence checks found in:"
grep -r "func.*FileExists\|func.*DirectoryExists" pkg/mcp --include="*.go" | wc -l
echo "   file existence functions found"

echo ""
echo "5. String Formatting Function Duplicates:"
echo "   FormatBytes implementations found in:"
grep -r "func.*FormatBytes" pkg/mcp --include="*.go"

echo ""
echo "=== Consolidation Actions Taken ==="
echo ""
echo "✅ Created pkg/mcp/utils/string_utils.go with:"
echo "   - ToSnakeCase (consolidated from 3+ implementations)"
echo "   - ToCamelCase, ToKebabCase"
echo "   - String manipulation utilities"
echo ""
echo "✅ Created pkg/mcp/utils/error_utils.go with:"
echo "   - WrapError, WrapErrorWithContext"
echo "   - ErrorChain for multiple errors"
echo "   - Error classification utilities"
echo ""

echo "=== Next Steps ==="
echo ""
echo "1. Update imports to use consolidated utilities:"
echo "   import \"github.com/Azure/container-kit/pkg/mcp/utils\""
echo ""
echo "2. Replace duplicate implementations:"
echo "   - Replace toSnakeCase calls with utils.ToSnakeCase"
echo "   - Replace WrapError calls with utils.WrapError"
echo "   - Update validation error handling"
echo ""
echo "3. Remove duplicate function definitions after migration"
echo ""

echo "=== Impact Analysis ==="
echo ""
echo "Files that can be simplified by using consolidated utilities:"
find pkg/mcp -name "*.go" -exec grep -l "toSnakeCase\|WrapError\|FormatBytes" {} \; | wc -l
echo "files contain duplicate utility patterns"

echo ""
echo "Estimated lines of code that can be removed:"
grep -r "func.*toSnakeCase\|func.*WrapError" pkg/mcp --include="*.go" | wc -l
echo "duplicate function definitions found"

echo ""
echo "=== Example Migration Commands ==="
echo ""
echo "# Replace toSnakeCase calls:"
echo "find pkg/mcp -name '*.go' -exec sed -i 's/toSnakeCase(/utils.ToSnakeCase(/g' {} \;"
echo ""
echo "# Replace WrapError calls:"
echo "find pkg/mcp -name '*.go' -exec sed -i 's/WrapError(/utils.WrapError(/g' {} \;"
echo ""
echo "# Add utils import where needed:"
echo "find pkg/mcp -name '*.go' -exec grep -l 'utils\.ToSnakeCase\|utils\.WrapError' {} \; | xargs -I {} sed -i '/^import (/a\\timportutils \"github.com/Azure/container-kit/pkg/mcp/utils\"' {}"
