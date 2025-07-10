#!/bin/bash

# Extract interface definitions from source code
echo "=== Extracting Interface Definitions ==="

OUTPUT_FILE="docs/api/extracted-interfaces.md"

cat > "$OUTPUT_FILE" << 'EOF'
# Extracted Interface Definitions

Generated on: $(date)

This document contains interface definitions extracted directly from the source code.

EOF

# Extract from api/interfaces.go
echo "## From pkg/mcp/application/api/interfaces.go" >> "$OUTPUT_FILE"
echo '```go' >> "$OUTPUT_FILE"

if [ -f "pkg/mcp/application/api/interfaces.go" ]; then
    # Extract interface definitions
    awk '/^type .* interface {/,/^}/' pkg/mcp/application/api/interfaces.go >> "$OUTPUT_FILE"
else
    echo "// File not found" >> "$OUTPUT_FILE"
fi

echo '```' >> "$OUTPUT_FILE"

# Extract from services/interfaces.go
echo -e "\n## From pkg/mcp/application/services/interfaces.go" >> "$OUTPUT_FILE"
echo '```go' >> "$OUTPUT_FILE"

if [ -f "pkg/mcp/application/services/interfaces.go" ]; then
    awk '/^type .* interface {/,/^}/' pkg/mcp/application/services/interfaces.go >> "$OUTPUT_FILE"
else
    echo "// File not found" >> "$OUTPUT_FILE"
fi

echo '```' >> "$OUTPUT_FILE"

# Count interfaces
echo -e "\n## Interface Statistics" >> "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"

if [ -f "pkg/mcp/application/api/interfaces.go" ]; then
    API_COUNT=$(grep -c "^type .* interface {" pkg/mcp/application/api/interfaces.go)
    echo "- API Interfaces: $API_COUNT" >> "$OUTPUT_FILE"
fi

if [ -f "pkg/mcp/application/services/interfaces.go" ]; then
    SERVICE_COUNT=$(grep -c "^type .* interface {" pkg/mcp/application/services/interfaces.go)
    echo "- Service Interfaces: $SERVICE_COUNT" >> "$OUTPUT_FILE"
fi

echo "" >> "$OUTPUT_FILE"
echo "✅ Interface extraction complete: $OUTPUT_FILE"
