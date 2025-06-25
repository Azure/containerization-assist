#!/bin/bash

# Batch migration script for fmt.Errorf to types.NewRichError
# This script handles common error patterns systematically

echo "Starting batch error migration..."

# Function to add mcptypes import if not already present
add_mcptypes_import() {
    local file=$1
    if ! grep -q "mcptypes.*github.com/Azure/container-copilot/pkg/mcp/types" "$file"; then
        # Check if there's already an import block
        if grep -q "^import (" "$file"; then
            # Add to existing import block
            sed -i '/^import (/a\	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"' "$file"
        else
            # Add new import block after package declaration
            sed -i '/^package/a\\nimport (\n\tmcptypes "github.com/Azure/container-copilot/pkg/mcp/types"\n)' "$file"
        fi
        echo "Added mcptypes import to $file"
    fi
}

# Function to migrate common error patterns
migrate_common_patterns() {
    local file=$1
    
    # Pattern 1: Simple validation errors
    sed -i 's/fmt\.Errorf("validation failed: %v", \([^)]*\))/mcptypes.WrapRichError(\1, "VALIDATION_FAILED", "validation failed", "validation_error")/g' "$file"
    sed -i 's/fmt\.Errorf("invalid .* format")/mcptypes.NewRichError("INVALID_FORMAT", "&", "validation_error")/g' "$file"
    
    # Pattern 2: File/IO errors
    sed -i 's/fmt\.Errorf("failed to read .*: %w", \([^)]*\))/mcptypes.WrapRichError(\1, "READ_FAILED", "failed to read file", "io_error")/g' "$file"
    sed -i 's/fmt\.Errorf("failed to write .*: %w", \([^)]*\))/mcptypes.WrapRichError(\1, "WRITE_FAILED", "failed to write file", "io_error")/g' "$file"
    
    # Pattern 3: Configuration errors
    sed -i 's/fmt\.Errorf("missing required field: %s", \([^)]*\))/mcptypes.NewRichError("MISSING_REQUIRED_FIELD", fmt.Sprintf("missing required field: %s", \1), "configuration_error")/g' "$file"
    
    # Pattern 4: Network/connection errors
    sed -i 's/fmt\.Errorf("connection failed: %w", \([^)]*\))/mcptypes.WrapRichError(\1, "CONNECTION_FAILED", "connection failed", "network_error")/g' "$file"
    
    echo "Migrated common patterns in $file"
}

# Find all .go files in pkg/mcp/ and migrate them
find pkg/mcp/ -name "*.go" -not -path "*/vendor/*" | while read -r file; do
    # Skip test files for now
    if [[ "$file" == *"_test.go" ]]; then
        continue
    fi
    
    # Check if file has fmt.Errorf instances
    if grep -q "fmt\.Errorf" "$file"; then
        echo "Processing $file..."
        
        # Add import first
        add_mcptypes_import "$file"
        
        # Apply migrations
        migrate_common_patterns "$file"
        
        # Count remaining instances
        remaining=$(grep -c "fmt\.Errorf" "$file" 2>/dev/null || echo "0")
        echo "  Remaining fmt.Errorf instances: $remaining"
    fi
done

echo "Batch migration completed!"

# Show summary
echo "Summary:"
total_errorf=$(find pkg/mcp/ -name "*.go" -not -name "*_test.go" -exec grep -c "fmt\.Errorf" {} + 2>/dev/null | awk '{sum += $1} END {print sum}')
total_rich=$(find pkg/mcp/ -name "*.go" -not -name "*_test.go" -exec grep -c "mcptypes\..*RichError" {} + 2>/dev/null | awk '{sum += $1} END {print sum}')

echo "Total fmt.Errorf instances: $total_errorf"
echo "Total rich error instances: $total_rich"
echo "Adoption rate: $(echo "scale=1; $total_rich * 100 / ($total_errorf + $total_rich)" | bc -l)%"