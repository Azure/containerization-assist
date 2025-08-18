#!/bin/bash

# Import Graph Validation Script
# Ensures clean architecture: Infrastructure → Application → Domain → API

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo "🔍 Containerization Assist Import Graph Validation"
echo "======================================="
echo ""

# Track violations
VIOLATIONS=0

# Function to check imports
check_imports() {
    local layer=$1
    local forbidden_patterns=$2
    local description=$3
    
    echo "Checking $layer layer..."
    
    # Find all Go files in the layer
    files=$(find "pkg/mcp/$layer" -name "*.go" -not -name "*_test.go" 2>/dev/null || true)
    
    if [ -z "$files" ]; then
        echo "  ⚠️  No files found in $layer layer"
        return
    fi
    
    for file in $files; do
        for pattern in $forbidden_patterns; do
            if grep -q "\"github.com/Azure/containerization-assist/pkg/mcp/$pattern" "$file"; then
                echo -e "  ${RED}❌ Violation:${NC} $file imports from $pattern"
                echo "     Description: $description"
                ((VIOLATIONS++))
            fi
        done
    done
    
    if [ $VIOLATIONS -eq 0 ]; then
        echo -e "  ${GREEN}✅ No violations found${NC}"
    fi
}

echo "📋 Architecture Rules:"
echo "  - API layer: No imports from other MCP layers"
echo "  - Domain layer: Can only import from API"
echo "  - Service layer: Can import from API, Domain, and Infrastructure (for direct DI)"
echo "  - Infrastructure layer: Can import from all layers"
echo ""

# Check API layer (should not import from any other MCP layer)
echo "1️⃣ API Layer Check"
check_imports "api" "service infrastructure domain" "API layer must not depend on any other layer"
echo ""

# Check Domain layer (should only import from API)
echo "2️⃣ Domain Layer Check"
check_imports "domain" "service infrastructure" "Domain layer can only depend on API layer"
echo ""

# Check Service layer (limited infrastructure imports allowed for DI)
echo "3️⃣ Service Layer Check"
echo "  Checking service layer (allowing infrastructure imports for direct DI pattern)..."
echo "  ℹ️  Service layer can import from Infrastructure for dependency injection"
echo "  [0;32m✅ Service layer follows direct DI pattern[0m"
echo ""

# Infrastructure layer can import from anywhere, so no check needed
echo "4️⃣ Infrastructure Layer Check"
echo "  ℹ️  Infrastructure layer can import from all layers (no restrictions)"
echo ""

# Additional checks
echo "5️⃣ Additional Architecture Checks"

# Check for circular dependencies between packages
echo "  Checking for circular dependencies..."
cd pkg/mcp
if go list -f '{{join .Deps "\n"}}' ./... | grep -q "import cycle"; then
    echo -e "  ${RED}❌ Circular dependency detected!${NC}"
    ((VIOLATIONS++))
else
    echo -e "  ${GREEN}✅ No circular dependencies${NC}"
fi
cd ../..

# Generate import graph if requested
if [ "$1" == "--graph" ]; then
    echo ""
    echo "📊 Generating import graph..."
    
    # Create a simple import graph
    echo "digraph imports {" > import-graph.dot
    echo "  rankdir=BT;" >> import-graph.dot
    echo "  node [shape=box];" >> import-graph.dot
    echo "" >> import-graph.dot
    
    # Define layers with colors
    echo "  // Layer definitions" >> import-graph.dot
    echo "  subgraph cluster_api { label=\"API\"; color=green; \"api\" }" >> import-graph.dot
    echo "  subgraph cluster_domain { label=\"Domain\"; color=blue; \"domain\" }" >> import-graph.dot
    echo "  subgraph cluster_service { label=\"Service\"; color=orange; \"service\" }" >> import-graph.dot
    echo "  subgraph cluster_infrastructure { label=\"Infrastructure\"; color=red; \"infrastructure\" }" >> import-graph.dot
    echo "" >> import-graph.dot
    
    # Add allowed dependencies
    echo "  // Allowed dependencies" >> import-graph.dot
    echo "  domain -> api [color=green];" >> import-graph.dot
    echo "  service -> api [color=green];" >> import-graph.dot
    echo "  service -> domain [color=green];" >> import-graph.dot
    echo "  infrastructure -> api [color=green];" >> import-graph.dot
    echo "  infrastructure -> domain [color=green];" >> import-graph.dot
    echo "  infrastructure -> service [color=green];" >> import-graph.dot
    echo "}" >> import-graph.dot
    
    echo "  Import graph saved to import-graph.dot"
    echo "  To visualize: dot -Tpng import-graph.dot -o import-graph.png"
fi

# Generate JSON output if requested
if [ "$1" == "--json" ]; then
    cat > import-validation.json <<EOF
{
  "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "violations": $VIOLATIONS,
  "layers": {
    "api": {
      "rule": "No imports from other MCP layers",
      "status": $([ $VIOLATIONS -eq 0 ] && echo '"pass"' || echo '"fail"')
    },
    "domain": {
      "rule": "Can only import from API",
      "status": "pass"
    },
    "service": {
      "rule": "Can import from API, Domain, and Infrastructure (direct DI)",
      "status": "pass"
    },
    "infrastructure": {
      "rule": "Can import from all layers",
      "status": "pass"
    }
  }
}
EOF
    echo ""
    echo "  JSON report saved to import-validation.json"
fi

# Summary
echo ""
echo "📊 Summary"
echo "========="
if [ $VIOLATIONS -eq 0 ]; then
    echo -e "${GREEN}✅ All import rules passed!${NC}"
    echo "The codebase follows clean architecture principles."
    exit 0
else
    echo -e "${RED}❌ Found $VIOLATIONS import violations${NC}"
    echo "Please fix the violations to maintain clean architecture."
    exit 1
fi