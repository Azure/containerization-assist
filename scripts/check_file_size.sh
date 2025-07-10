#!/bin/bash
set -e

echo "=== FILE SIZE CHECKER ==="

# Different limits for different types of files based on their complexity and purpose
default_max_lines=800           # Standard business logic files
interface_max_lines=1200        # Interface definitions, domain logic, helpers
implementation_max_lines=1600   # Complex implementations, consolidated files
consolidated_max_lines=2200     # Large analysis and command implementations

echo "Checking for files exceeding appropriate line limits..."

# Function to get appropriate limit for a file
get_max_lines() {
    local file="$1"
    case "$file" in
        # Large analysis and command files
        *analyze_implementation.go)
            echo $consolidated_max_lines
            ;;
        # Interface and API files
        *interfaces.go|*api.go)
            echo $interface_max_lines
            ;;
        # Implementation, consolidated, and complex files
        *consolidated*.go|*implementation*.go|*_impl.go|*server_impl.go|*tool_registration.go)
            echo $implementation_max_lines
            ;;
        # Domain validation and security files (complex business logic)
        *domain_validators.go|*tags.go|*security*.go)
            echo $interface_max_lines
            ;;
        # Helper and state management files
        *helpers.go|*helper.go|*state_types.go|*context_enrichers.go|*auto_fix_helper.go)
            echo $interface_max_lines
            ;;
        # Infrastructure operations files
        *k8s_operations.go|*operations.go)
            echo $interface_max_lines
            ;;
        *)
            echo $default_max_lines
            ;;
    esac
}

# Check all files and collect violations
violations=$(find pkg/mcp -name "*.go" | while read file; do
    lines=$(wc -l < "$file")
    max_lines=$(get_max_lines "$file")

    if [ "$lines" -gt "$max_lines" ]; then
        echo "$file: $lines lines (exceeds $max_lines)"
    fi
done)

if [ -n "$violations" ]; then
    echo "❌ FAIL: Files exceed appropriate size limits:"
    echo "$violations"
    echo ""
    echo "File size limits by type:"
    echo "  • Default files: $default_max_lines lines"
    echo "  • Interface files: $interface_max_lines lines"
    echo "  • Implementation files: $implementation_max_lines lines"
    echo "  • Large analysis files: $consolidated_max_lines lines"
    echo ""
    echo "Consider breaking very large files into smaller, focused modules"
    exit 1
else
    echo "✅ PASS: All files within appropriate line limits"
    echo "File size limits by type:"
    echo "  • Default files: $default_max_lines lines"
    echo "  • Interface files: $interface_max_lines lines"
    echo "  • Implementation files: $implementation_max_lines lines"
    echo "  • Large analysis files: $consolidated_max_lines lines"
fi
