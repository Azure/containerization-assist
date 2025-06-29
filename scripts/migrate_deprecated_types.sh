#!/bin/bash
# migrate_deprecated_types.sh
# Migrates deprecated types from pkg/mcp/types to pkg/mcp unified interfaces

set -e

echo "🚀 Starting migration of deprecated types to unified interfaces..."

# Create backup
echo "📦 Creating backup of current state..."
cp -r pkg/mcp/internal pkg/mcp/internal.backup
cp pkg/mcp/types/interfaces.go pkg/mcp/types/interfaces.go.backup

# Count initial state
INITIAL_TYPES_IMPORTS=$(grep -r "pkg/mcp/types" pkg/mcp/internal/ | wc -l)
echo "📊 Initial state: $INITIAL_TYPES_IMPORTS imports of deprecated types package"

echo ""
echo "🔄 Phase 1: Replace deprecated type imports..."

# Replace deprecated type imports in internal packages
echo "   Updating import statements..."
find pkg/mcp/internal -name "*.go" -exec sed -i 's|github.com/Azure/container-kit/pkg/mcp/types|github.com/Azure/container-kit/pkg/mcp|g' {} \;

# Check for files that now have both imports (need manual cleanup)
echo "   Checking for files with duplicate imports..."
DUPLICATE_IMPORTS=$(find pkg/mcp/internal -name "*.go" -exec grep -l "github.com/Azure/container-kit/pkg/mcp" {} \; | xargs grep -l "\"github.com/Azure/container-kit/pkg/mcp\".*\"github.com/Azure/container-kit/pkg/mcp\"" 2>/dev/null | wc -l)
if [ "$DUPLICATE_IMPORTS" -gt 0 ]; then
    echo "⚠️  Found files with duplicate imports - will need manual cleanup"
fi

echo ""
echo "🔄 Phase 2: Replace specific type references..."

# Priority 1: Error Handling Types (654 total uses)
echo "   Migrating error handling types..."
find pkg/mcp/internal -name "*.go" -exec sed -i 's|types\.NewRichError|mcp.NewRichError|g' {} \;
find pkg/mcp/internal -name "*.go" -exec sed -i 's|types\.RichError|mcp.RichError|g' {} \;
find pkg/mcp/internal -name "*.go" -exec sed -i 's|types\.NewValidationErrorBuilder|mcp.NewValidationErrorBuilder|g' {} \;
find pkg/mcp/internal -name "*.go" -exec sed -i 's|types\.NewErrorBuilder|mcp.NewErrorBuilder|g' {} \;

# Priority 2: Tool Metadata (95 uses)
echo "   Migrating tool metadata types..."
find pkg/mcp/internal -name "*.go" -exec sed -i 's|types\.ToolMetadata|mcp.ToolMetadata|g' {} \;

# Priority 3: Session & Workflow Types (263 total uses)
echo "   Migrating session and workflow types..."
find pkg/mcp/internal -name "*.go" -exec sed -i 's|types\.SessionState|mcp.SessionState|g' {} \;
find pkg/mcp/internal -name "*.go" -exec sed -i 's|types\.AIAnalyzer|mcp.AIAnalyzer|g' {} \;
find pkg/mcp/internal -name "*.go" -exec sed -i 's|types\.ToolSessionManager|mcp.ToolSessionManager|g' {} \;
find pkg/mcp/internal -name "*.go" -exec sed -i 's|types\.FixAttempt|mcp.FixAttempt|g' {} \;
find pkg/mcp/internal -name "*.go" -exec sed -i 's|types\.ConversationStage|mcp.ConversationStage|g' {} \;

# Additional common types
echo "   Migrating additional common types..."
find pkg/mcp/internal -name "*.go" -exec sed -i 's|types\.MCPRequest|mcp.MCPRequest|g' {} \;
find pkg/mcp/internal -name "*.go" -exec sed -i 's|types\.MCPResponse|mcp.MCPResponse|g' {} \;
find pkg/mcp/internal -name "*.go" -exec sed -i 's|types\.MCPError|mcp.MCPError|g' {} \;
find pkg/mcp/internal -name "*.go" -exec sed -i 's|types\.ProgressStage|mcp.ProgressStage|g' {} \;

echo ""
echo "🔍 Phase 3: Validation and testing..."

# Test build after migration
echo "   Testing build after migration..."
if go build -tags mcp ./pkg/mcp/... 2>/dev/null; then
    echo "✅ Build successful after migration"
else
    echo "❌ Build failed after migration - checking errors..."
    BUILD_ERRORS=$(go build -tags mcp ./pkg/mcp/... 2>&1)
    echo "$BUILD_ERRORS" | head -10
    echo ""
    echo "💡 This is expected - we need to move the actual type definitions"
    echo "   The script has updated references, now types need to be moved to main package"
fi

# Count remaining types imports
REMAINING_TYPES_IMPORTS=$(grep -r "pkg/mcp/types" pkg/mcp/internal/ 2>/dev/null | wc -l || echo "0")
echo "📊 Remaining imports of types package: $REMAINING_TYPES_IMPORTS"

# Count remaining types.* references
REMAINING_TYPES_REFS=$(grep -r "types\." pkg/mcp/internal/ 2>/dev/null | wc -l || echo "0")
echo "📊 Remaining types.* references: $REMAINING_TYPES_REFS"

if [ "$REMAINING_TYPES_REFS" -eq 0 ]; then
    echo "✅ All types.* references successfully migrated to mcp.*"
else
    echo "⚠️  Some types.* references remain - may need manual review"
    echo "   Top remaining references:"
    grep -r "types\." pkg/mcp/internal/ 2>/dev/null | cut -d: -f2 | grep -o "types\.[A-Za-z]*" | sort | uniq -c | sort -nr | head -5 || true
fi

echo ""
echo "📋 Next steps:"
echo "1. Move type definitions from pkg/mcp/types/interfaces.go to pkg/mcp/interfaces.go"
echo "2. Remove local interface files in internal packages"
echo "3. Test and validate complete build"
echo "4. Run the validation script"

echo ""
echo "🎉 Migration script completed!"
echo "📁 Backup created at: pkg/mcp/internal.backup and pkg/mcp/types/interfaces.go.backup"
