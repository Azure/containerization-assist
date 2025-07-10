#!/bin/bash

# Test generation helper script
echo "=== Container Kit Test Generator ==="

PACKAGE_PATH="$1"
TEST_TYPE="${2:-unit}"

if [ -z "$PACKAGE_PATH" ]; then
    echo "Usage: $0 <package_path> [unit|integration|benchmark]"
    echo ""
    echo "Examples:"
    echo "  $0 pkg/mcp/domain/errors unit"
    echo "  $0 pkg/mcp/application/core integration"
    echo "  $0 pkg/mcp/domain/security benchmark"
    exit 1
fi

# Validate package exists
if [ ! -d "$PACKAGE_PATH" ]; then
    echo "❌ Package directory not found: $PACKAGE_PATH"
    exit 1
fi

# Extract package name
PACKAGE_NAME=$(basename "$PACKAGE_PATH")
FULL_PACKAGE_NAME=$(echo "$PACKAGE_PATH" | sed 's/^pkg\//github.com\/Azure\/container-kit\/pkg\//')

# Determine test file name
case "$TEST_TYPE" in
    "unit")
        TEST_FILE="${PACKAGE_PATH}/${PACKAGE_NAME}_test.go"
        TEMPLATE_FILE="test/templates/unit_test_template.go"
        ;;
    "integration")
        TEST_FILE="${PACKAGE_PATH}/${PACKAGE_NAME}_integration_test.go"
        TEMPLATE_FILE="test/templates/integration_test_template.go"
        ;;
    "benchmark")
        TEST_FILE="${PACKAGE_PATH}/${PACKAGE_NAME}_bench_test.go"
        TEMPLATE_FILE="test/templates/unit_test_template.go" # Use unit template for now
        ;;
    *)
        echo "❌ Invalid test type: $TEST_TYPE"
        echo "Valid types: unit, integration, benchmark"
        exit 1
        ;;
esac

# Check if test file already exists
if [ -f "$TEST_FILE" ]; then
    echo "⚠️  Test file already exists: $TEST_FILE"
    read -p "Overwrite? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Cancelled"
        exit 0
    fi
fi

# Check if template exists
if [ ! -f "$TEMPLATE_FILE" ]; then
    echo "❌ Template file not found: $TEMPLATE_FILE"
    exit 1
fi

# Find functions to test in the package
echo "Analyzing package for testable functions..."
FUNCTIONS=()

# Look for exported functions in .go files (excluding test files)
while IFS= read -r func_line; do
    if [[ $func_line =~ ^func[[:space:]]+([A-Z][a-zA-Z0-9_]*)\( ]]; then
        FUNCTION_NAME=${BASH_REMATCH[1]}
        FUNCTIONS+=("$FUNCTION_NAME")
        echo "  Found function: $FUNCTION_NAME"
    fi
done < <(find "$PACKAGE_PATH" -name "*.go" -not -name "*_test.go" -exec grep -h "^func [A-Z]" {} \;)

# Look for exported methods
while IFS= read -r method_line; do
    if [[ $method_line =~ ^func[[:space:]]+\([^)]+\)[[:space:]]+([A-Z][a-zA-Z0-9_]*)\( ]]; then
        METHOD_NAME=${BASH_REMATCH[1]}
        FUNCTIONS+=("$METHOD_NAME")
        echo "  Found method: $METHOD_NAME"
    fi
done < <(find "$PACKAGE_PATH" -name "*.go" -not -name "*_test.go" -exec grep -h "^func ([^)]*) [A-Z]" {} \;)

if [ ${#FUNCTIONS[@]} -eq 0 ]; then
    echo "  No exported functions found"
    MAIN_FUNCTION="ExampleFunction"
else
    MAIN_FUNCTION="${FUNCTIONS[0]}"
fi

echo "Using main function: $MAIN_FUNCTION"

# Generate test file from template
echo "Generating test file: $TEST_FILE"

# Create test file with substitutions
sed \
    -e "s/PACKAGE_NAME/$PACKAGE_NAME/g" \
    -e "s/FUNCTION_NAME/$MAIN_FUNCTION/g" \
    "$TEMPLATE_FILE" > "$TEST_FILE"

# Add package import if needed
if grep -q "github.com/Azure/container-kit" "$TEST_FILE"; then
    # Add actual import path
    sed -i "1a\\import \"$FULL_PACKAGE_NAME\"" "$TEST_FILE"
fi

# Generate additional test functions for other functions found
if [ ${#FUNCTIONS[@]} -gt 1 ]; then
    echo "" >> "$TEST_FILE"
    echo "// Additional test functions for other exported functions" >> "$TEST_FILE"

    for func in "${FUNCTIONS[@]:1}"; do
        cat >> "$TEST_FILE" << EOF

func Test${func}_Success(t *testing.T) {
    // TODO: Implement test for $func
    t.Skip("Test not implemented yet")
}

func Test${func}_Error(t *testing.T) {
    // TODO: Implement error test for $func
    t.Skip("Test not implemented yet")
}
EOF
    done
fi

# Format the generated file
if command -v gofmt >/dev/null 2>&1; then
    echo "Formatting generated test file..."
    gofmt -w "$TEST_FILE"
fi

# Validate the generated test compiles
echo "Validating generated test..."
if go test -c "$PACKAGE_PATH" -o /dev/null >/dev/null 2>&1; then
    echo "✅ Test file generated successfully: $TEST_FILE"

    # Show what to do next
    echo ""
    echo "Next steps:"
    echo "1. Review and customize the generated test: $TEST_FILE"
    echo "2. Implement actual test logic for your functions"
    echo "3. Run tests: go test $PACKAGE_PATH"
    echo "4. Check coverage: go test -cover $PACKAGE_PATH"

    if [ ${#FUNCTIONS[@]} -gt 1 ]; then
        echo "5. Implement tests for additional functions:"
        for func in "${FUNCTIONS[@]:1}"; do
            echo "   - $func"
        done
    fi
else
    echo "⚠️  Test file generated but has compilation errors"
    echo "Please review and fix: $TEST_FILE"
fi

# Add to git if in a git repository
if git rev-parse --git-dir >/dev/null 2>&1; then
    echo ""
    echo "Adding test file to git..."
    git add "$TEST_FILE"
    echo "✅ Test file added to git staging"
fi
