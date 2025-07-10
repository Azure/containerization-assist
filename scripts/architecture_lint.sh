#!/bin/bash

# Architecture Linting Script
# Comprehensive architecture boundary and design rule enforcement

set -e

echo "🏗️  Container Kit Architecture Linter"
echo "======================================"

# Configuration
MAX_IMPORT_DEPTH=3
PKG_ROOT="pkg/mcp"
SCRIPTS_DIR="scripts"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Track overall status
OVERALL_STATUS=0

echo ""
echo "${BLUE}📊 Running Architecture Checks...${NC}"
echo ""

# 1. Import Depth Check
echo "1️⃣  Checking import depth (max: $MAX_IMPORT_DEPTH levels)..."
if go run "$SCRIPTS_DIR/check_import_depth.go" "$PKG_ROOT" > /tmp/import_depth_results.txt 2>&1; then
    violations=$(grep "Found.*violations" /tmp/import_depth_results.txt | grep -o '[0-9]\+' || echo "0")
    if [ "$violations" -eq 0 ]; then
        echo "   ✅ All imports within depth limit ($MAX_IMPORT_DEPTH)"
    else
        echo "   ❌ Found $violations import depth violations"
        cat /tmp/import_depth_results.txt
        OVERALL_STATUS=1
    fi
else
    echo "   ❌ Import depth check failed"
    cat /tmp/import_depth_results.txt
    OVERALL_STATUS=1
fi

echo ""

# 2. Architecture Boundary Check
echo "2️⃣  Checking architecture boundaries..."
if go run "$SCRIPTS_DIR/check_architecture_boundaries.go" "$PKG_ROOT" > /tmp/boundaries_results.txt 2>&1; then
    echo "   ✅ All architecture boundaries maintained"
else
    echo "   ❌ Architecture boundary violations found"
    cat /tmp/boundaries_results.txt
    OVERALL_STATUS=1
fi

echo ""

# 3. Circular Dependency Check
echo "3️⃣  Checking for circular dependencies..."
if go run "$SCRIPTS_DIR/detect_circular_dependencies.go" "$PKG_ROOT" > /tmp/circular_deps_results.txt 2>&1; then
    echo "   ✅ No circular dependencies found"

    # Show some useful stats
    avg_deps=$(grep "Average dependencies" /tmp/circular_deps_results.txt | grep -o '[0-9.]\+' || echo "0")
    leaf_count=$(grep "Leaf packages" /tmp/circular_deps_results.txt | grep -o '[0-9]\+' || echo "0")
    echo "   📈 Average deps/package: $avg_deps, Leaf packages: $leaf_count"
else
    echo "   ❌ Circular dependencies detected"
    cat /tmp/circular_deps_results.txt
    OVERALL_STATUS=1
fi

echo ""

# 4. Forbidden Pattern Check
echo "4️⃣  Checking for forbidden patterns..."

# Check for old flattened package imports that should no longer exist
forbidden_patterns=(
    "pkg/mcp/domain/errors/"
    "pkg/mcp/domain/session/"
    "pkg/mcp/domain/shared/"
    "pkg/mcp/domain/config/"
    "pkg/mcp/domain/tools/"
    "pkg/mcp/domain/containerization/"
    "pkg/mcp/application/api/"
    "pkg/mcp/application/services/"
    "pkg/mcp/application/logging/"
    "pkg/mcp/domain/internal/"
)

forbidden_found=0
for pattern in "${forbidden_patterns[@]}"; do
    if grep -r "github.com/Azure/container-kit/$pattern" "$PKG_ROOT" --include="*.go" 2>/dev/null | grep -v "^Binary file" > /dev/null; then
        if [ $forbidden_found -eq 0 ]; then
            echo "   ❌ Found imports to old/forbidden packages:"
            forbidden_found=1
        fi
        echo "      - Imports to: $pattern"
        grep -r "github.com/Azure/container-kit/$pattern" "$PKG_ROOT" --include="*.go" | head -3
        OVERALL_STATUS=1
    fi
done

if [ $forbidden_found -eq 0 ]; then
    echo "   ✅ No forbidden import patterns found"
fi

echo ""

# 5. Package Naming Convention Check
echo "5️⃣  Checking package naming conventions..."

naming_violations=0

# Check for packages that should be flattened (depth > 3)
while IFS= read -r -d '' dir; do
    rel_path=$(realpath --relative-to="$PKG_ROOT" "$dir")
    depth=$(echo "$rel_path" | tr '/' '\n' | wc -l)

    if [ "$depth" -gt 3 ]; then
        if [ $naming_violations -eq 0 ]; then
            echo "   ❌ Found packages exceeding depth limit:"
            naming_violations=1
        fi
        echo "      - $rel_path (depth: $depth)"
        OVERALL_STATUS=1
    fi
done < <(find "$PKG_ROOT" -type d -name "*.go" -prune -o -type d -print0)

if [ $naming_violations -eq 0 ]; then
    echo "   ✅ All package names follow conventions"
fi

echo ""

# 6. Internal Package Access Check
echo "6️⃣  Checking internal package access..."

internal_violations=0
while IFS= read -r line; do
    if [[ $line == *"/internal/"* ]]; then
        file=$(echo "$line" | cut -d: -f1)
        import=$(echo "$line" | cut -d'"' -f2)

        # Check if this is cross-package access to internal
        file_pkg_path=$(dirname "$file")
        import_relative=$(echo "$import" | sed 's|github.com/Azure/container-kit/||')
        import_pkg_path=$(echo "$import_relative" | sed 's|/internal/.*||')

        if [[ "$file_pkg_path" != *"$import_pkg_path"* ]]; then
            if [ $internal_violations -eq 0 ]; then
                echo "   ❌ Found cross-package internal imports:"
                internal_violations=1
            fi
            echo "      - $file imports $import"
            OVERALL_STATUS=1
        fi
    fi
done < <(grep -r "github.com/Azure/container-kit.*internal" "$PKG_ROOT" --include="*.go" 2>/dev/null || true)

if [ $internal_violations -eq 0 ]; then
    echo "   ✅ No improper internal package access"
fi

echo ""

# Summary
echo "📋 Architecture Lint Summary"
echo "============================"

if [ $OVERALL_STATUS -eq 0 ]; then
    echo "${GREEN}✅ All architecture checks PASSED${NC}"
    echo ""
    echo "🎉 Your code follows all architecture guidelines!"
    echo ""
    echo "Architecture Quality Metrics:"
    echo "- ✅ Import depth ≤ $MAX_IMPORT_DEPTH levels"
    echo "- ✅ Clean architecture boundaries maintained"
    echo "- ✅ No circular dependencies"
    echo "- ✅ Proper package organization"
    echo "- ✅ No forbidden patterns"
    echo ""
else
    echo "${RED}❌ Architecture lint FAILED${NC}"
    echo ""
    echo "🚨 Please fix the architecture violations before committing."
    echo ""
    echo "Common fixes:"
    echo "1. Use flattened packages (e.g., 'errors' not 'domain/errors')"
    echo "2. Follow clean architecture: domain ← application ← infrastructure"
    echo "3. Avoid deep package nesting (max 3 levels)"
    echo "4. Extract interfaces to break circular dependencies"
    echo ""
fi

# Clean up temp files
rm -f /tmp/import_depth_results.txt /tmp/boundaries_results.txt /tmp/circular_deps_results.txt

exit $OVERALL_STATUS
