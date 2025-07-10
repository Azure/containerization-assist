#!/bin/bash

# Documentation coverage checker
echo "=== Documentation Coverage Check ==="

REPORT_FILE="docs/DOCUMENTATION_COVERAGE.md"

# Create report header
cat > "$REPORT_FILE" << 'EOF'
# Documentation Coverage Report

Generated on: $(date)

## Summary

This report shows the documentation coverage for Container Kit's public APIs.

EOF

# Check for README files
echo "## README Coverage" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"

DIRS=("pkg/mcp/domain" "pkg/mcp/application" "pkg/mcp/infra")
for dir in "${DIRS[@]}"; do
    if [ -f "$dir/README.md" ]; then
        echo "- ✅ $dir/README.md exists" >> "$REPORT_FILE"
    else
        echo "- ❌ $dir/README.md missing" >> "$REPORT_FILE"
    fi
done

# Check for interface documentation
echo -e "\n## Interface Documentation" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"

# Count interfaces in api/interfaces.go
if [ -f "pkg/mcp/application/api/interfaces.go" ]; then
    INTERFACE_COUNT=$(grep -c "^type .* interface {" pkg/mcp/application/api/interfaces.go || echo "0")
    echo "- Total interfaces defined: $INTERFACE_COUNT" >> "$REPORT_FILE"
    
    # Check if each interface has documentation
    DOCUMENTED_COUNT=$(grep -B1 "^type .* interface {" pkg/mcp/application/api/interfaces.go | grep -c "^//" || echo "0")
    echo "- Interfaces with comments: $DOCUMENTED_COUNT" >> "$REPORT_FILE"
    
    COVERAGE=$((DOCUMENTED_COUNT * 100 / INTERFACE_COUNT))
    echo "- Documentation coverage: $COVERAGE%" >> "$REPORT_FILE"
fi

# Check for example files
echo -e "\n## Example Coverage" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"

EXAMPLE_COUNT=$(find docs/examples -name "*.md" -type f | wc -l)
echo "- Example documents: $EXAMPLE_COUNT" >> "$REPORT_FILE"

# Check for test files
echo -e "\n## Test Documentation" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"

TEST_COUNT=$(find pkg/mcp -name "*_test.go" -type f | wc -l)
echo "- Test files: $TEST_COUNT" >> "$REPORT_FILE"

# Generate recommendations
echo -e "\n## Recommendations" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"

if [ "$COVERAGE" -lt 100 ]; then
    echo "1. Add documentation comments to all public interfaces" >> "$REPORT_FILE"
fi

if [ "$EXAMPLE_COUNT" -lt 5 ]; then
    echo "2. Add more example documentation" >> "$REPORT_FILE"
fi

echo "3. Ensure all public APIs have usage examples" >> "$REPORT_FILE"
echo "4. Keep documentation synchronized with code changes" >> "$REPORT_FILE"

# Summary
echo -e "\n---" >> "$REPORT_FILE"
if [ "$COVERAGE" -ge 80 ]; then
    echo "✅ Documentation coverage is good ($COVERAGE%)" >> "$REPORT_FILE"
else
    echo "⚠️  Documentation coverage needs improvement ($COVERAGE%)" >> "$REPORT_FILE"
fi

echo ""
echo "✅ Documentation check complete"
echo "Report saved to: $REPORT_FILE"
cat "$REPORT_FILE"