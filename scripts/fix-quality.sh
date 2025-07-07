#!/bin/bash
set -e

echo "🔧 Container Kit Quality Auto-Fix"
echo "================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Check if we're in the right directory
if [ ! -f "go.mod" ] || [ ! -d "pkg/mcp" ]; then
    print_error "Must be run from project root directory"
    exit 1
fi

echo "🔍 Analyzing codebase for auto-fixable issues..."

# 1. Format and organize imports
echo ""
echo "📝 Formatting code and organizing imports..."
if command -v gofumpt >/dev/null 2>&1; then
    gofumpt -w pkg/mcp/
    print_status "Code formatted with gofumpt"
else
    gofmt -w pkg/mcp/
    print_status "Code formatted with gofmt"
fi

if command -v goimports >/dev/null 2>&1; then
    goimports -w pkg/mcp/
    print_status "Imports organized"
else
    print_warning "goimports not found - install with: go install golang.org/x/tools/cmd/goimports@latest"
fi

# 2. Fix simple linting issues
echo ""
echo "🔧 Running auto-fixable linters..."
if command -v golangci-lint >/dev/null 2>&1; then
    golangci-lint run --fix ./pkg/mcp/... || true
    print_status "Auto-fixable lint issues resolved"
else
    print_warning "golangci-lint not found - install from https://golangci-lint.run/"
fi

# 3. Check for common issues and suggest fixes
echo ""
echo "🔍 Analyzing for common quality issues..."

# Check for oversized files
oversized_files=$(find pkg/mcp -name '*.go' -not -path '*/vendor/*' -exec wc -l {} + | awk '$1>800{print $1, $2}')
if [ -n "$oversized_files" ]; then
    print_warning "Found oversized files (>800 LOC):"
    echo "$oversized_files"
    echo ""
    echo "Suggested actions:"
    echo "1. Extract types to separate files"
    echo "2. Split by functional responsibility"
    echo "3. Create focused, testable modules"
    echo ""
fi

# Check for complex functions
if command -v gocyclo >/dev/null 2>&1; then
    complex_functions=$(gocyclo -over 15 pkg/mcp)
    if [ -n "$complex_functions" ]; then
        print_warning "Found complex functions (CC > 15):"
        echo "$complex_functions"
        echo ""
        echo "Suggested actions:"
        echo "1. Extract helper functions"
        echo "2. Use early returns"
        echo "3. Replace nested conditions with guard clauses"
        echo ""
    fi
else
    print_warning "gocyclo not found - install with: go install github.com/fzipp/gocyclo/cmd/gocyclo@latest"
fi

# Check for long constructors
long_constructors=$(grep -r "func New.*(" pkg/mcp --include="*.go" | awk -F',' '{if(NF>5) print}' || true)
if [ -n "$long_constructors" ]; then
    print_warning "Found constructors with >5 parameters:"
    echo "$long_constructors"
    echo ""
    echo "Suggested actions:"
    echo "1. Use functional options pattern"
    echo "2. Create configuration structs"
    echo "3. Provide sensible defaults"
    echo ""
fi

# Check for deep package nesting
deep_packages=$(find pkg/mcp -type d -mindepth 6 -not -path '*/vendor/*')
if [ -n "$deep_packages" ]; then
    print_warning "Found deeply nested packages (>5 levels):"
    echo "$deep_packages"
    echo ""
    echo "Suggested actions:"
    echo "1. Apply bounded-context principle"
    echo "2. Move packages up in hierarchy"
    echo "3. Consolidate micro-packages"
    echo ""
fi

# 4. Verify fixes didn't break anything
echo ""
echo "🧪 Verifying fixes..."

echo "Building all packages..."
if go build ./pkg/mcp/...; then
    print_status "Build successful"
else
    print_error "Build failed after fixes"
    exit 1
fi

echo "Running quick tests..."
if go test -short ./pkg/mcp/...; then
    print_status "Tests passing"
else
    print_error "Tests failed after fixes"
    exit 1
fi

# 5. Final quality check
echo ""
echo "📊 Final quality metrics:"

# File count
total_files=$(find pkg/mcp -name '*.go' -not -path '*/vendor/*' | wc -l)
oversized_count=$(find pkg/mcp -name '*.go' -not -path '*/vendor/*' -exec wc -l {} + | awk '$1>800' | wc -l)
echo "Files: $total_files total, $oversized_count oversized (>800 LOC)"

# Package count and depth
package_count=$(go list ./pkg/mcp/... | wc -l)
deep_package_count=$(find pkg/mcp -type d -mindepth 6 -not -path '*/vendor/*' | wc -l)
echo "Packages: $package_count total, $deep_package_count too deep (>5 levels)"

# Complexity
if command -v gocyclo >/dev/null 2>&1; then
    complex_count=$(gocyclo -over 15 pkg/mcp | wc -l)
    echo "Complex functions: $complex_count (CC > 15)"
fi

echo ""
if [ "$oversized_count" -eq 0 ] && [ "$deep_package_count" -eq 0 ]; then
    print_status "Quality auto-fix complete! 🎉"
    echo ""
    echo "Next steps:"
    echo "1. Review changes: git diff"
    echo "2. Run full test suite: make test-all"
    echo "3. Commit improvements: git add . && git commit -m 'quality: auto-fix formatting and simple issues'"
else
    print_warning "Auto-fix complete, but manual intervention needed for:"
    [ "$oversized_count" -gt 0 ] && echo "  - $oversized_count oversized files"
    [ "$deep_package_count" -gt 0 ] && echo "  - $deep_package_count deeply nested packages"
    echo ""
    echo "Please address these manually and re-run the script."
fi

echo ""
echo "🔧 Quality auto-fix script completed"
