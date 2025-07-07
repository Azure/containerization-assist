#!/bin/bash

# check-boundary-errors.sh
# Local script to check package boundary error handling compliance
# Part of the BETA workstream error unification system

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
RICHIFY_TOOL="./bin/mcp-richify"
BOUNDARIES_FILE="/tmp/local_boundaries.json"
REPORT_FILE="boundary-error-report.txt"

echo -e "${BLUE}🔍 Container Kit Boundary Error Checker${NC}"
echo -e "${BLUE}======================================${NC}"
echo ""

# Build the mcp-richify tool if it doesn't exist
if [ ! -f "$RICHIFY_TOOL" ]; then
    echo -e "${YELLOW}🔧 Building mcp-richify tool...${NC}"
    mkdir -p bin
    go build -o "$RICHIFY_TOOL" ./cmd/mcp-richify
    echo -e "${GREEN}✅ Tool built successfully${NC}"
    echo ""
fi

# Run boundary analysis
echo -e "${BLUE}📊 Analyzing package boundaries...${NC}"
"$RICHIFY_TOOL" boundaries "$BOUNDARIES_FILE"

if [ ! -f "$BOUNDARIES_FILE" ]; then
    echo -e "${RED}❌ Failed to generate boundary analysis${NC}"
    exit 1
fi

# Count different types of errors
TOTAL_ERRORS=$(jq -r 'keys | length' "$BOUNDARIES_FILE")
BOUNDARY_ERRORS=$(jq -r 'to_entries[] | select(.value.type == "BOUNDARY") | .key' "$BOUNDARIES_FILE" | wc -l)
INTERNAL_ERRORS=$(jq -r 'to_entries[] | select(.value.type == "INTERNAL") | .key' "$BOUNDARIES_FILE" | wc -l)

echo -e "${BLUE}📋 Analysis Summary:${NC}"
echo "  Total error locations: $TOTAL_ERRORS"
echo "  Boundary functions: $BOUNDARY_ERRORS (require RichError)"
echo "  Internal functions: $INTERNAL_ERRORS (fmt.Errorf allowed)"
echo ""

# Check for violations
echo -e "${BLUE}⚖️ Checking boundary compliance...${NC}"

VIOLATIONS_FILE="/tmp/violations.txt"
> "$VIOLATIONS_FILE"

# Check if any boundary functions still use fmt.Errorf/errors.New
jq -r 'to_entries[] | select(.value.type == "BOUNDARY") | .key' "$BOUNDARIES_FILE" | while read location; do
    file=$(echo $location | cut -d: -f1)
    line=$(echo $location | cut -d: -f2)

    if [ -f "$file" ]; then
        # Check if the line contains fmt.Errorf or errors.New
        if sed -n "${line}p" "$file" 2>/dev/null | grep -E "(fmt\.Errorf|errors\.New)" > /dev/null; then
            echo "$location" >> "$VIOLATIONS_FILE"
        fi
    fi
done

VIOLATION_COUNT=$(wc -l < "$VIOLATIONS_FILE" 2>/dev/null || echo "0")

if [ "$VIOLATION_COUNT" -gt 0 ]; then
    echo -e "${RED}❌ BOUNDARY ERROR VIOLATIONS DETECTED!${NC}"
    echo ""
    echo -e "${YELLOW}The following $VIOLATION_COUNT boundary functions must use RichError:${NC}"
    echo ""

    while read violation; do
        if [ -n "$violation" ]; then
            file=$(echo $violation | cut -d: -f1)
            line=$(echo $violation | cut -d: -f2)
            echo "  📍 $violation"

            # Show the actual line with context
            if [ -f "$file" ]; then
                echo "     $(sed -n "${line}p" "$file" 2>/dev/null | sed 's/^[[:space:]]*/     /' || echo '     (line not found)')"
            fi
            echo ""
        fi
    done < "$VIOLATIONS_FILE"

    echo -e "${YELLOW}🔧 To fix these violations:${NC}"
    echo ""
    echo "  1. Automatic conversion:"
    echo "     $RICHIFY_TOOL convert $BOUNDARIES_FILE"
    echo ""
    echo "  2. Manual conversion patterns:"
    echo "     fmt.Errorf(...) → mcperrors.NewError().Messagef(...).WithLocation().Build()"
    echo "     errors.New(...) → mcperrors.NewError().Message(...).WithLocation().Build()"
    echo ""
    echo -e "${BLUE}📖 See ADR-006 for the complete boundary error policy${NC}"

    # Generate detailed report
    echo "BOUNDARY ERROR COMPLIANCE REPORT" > "$REPORT_FILE"
    echo "===============================" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    echo "Generated: $(date)" >> "$REPORT_FILE"
    echo "Status: VIOLATIONS DETECTED" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    echo "Summary:" >> "$REPORT_FILE"
    echo "- Total errors: $TOTAL_ERRORS" >> "$REPORT_FILE"
    echo "- Boundary errors: $BOUNDARY_ERRORS" >> "$REPORT_FILE"
    echo "- Violations: $VIOLATION_COUNT" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    echo "Violations:" >> "$REPORT_FILE"
    cat "$VIOLATIONS_FILE" >> "$REPORT_FILE"

    exit 1
else
    echo -e "${GREEN}✅ All boundary functions properly use RichError!${NC}"
    echo -e "${GREEN}📈 Package boundary compliance: 100%${NC}"
    echo ""

    # Generate success report
    echo "BOUNDARY ERROR COMPLIANCE REPORT" > "$REPORT_FILE"
    echo "===============================" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    echo "Generated: $(date)" >> "$REPORT_FILE"
    echo "Status: COMPLIANT" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    echo "Summary:" >> "$REPORT_FILE"
    echo "- Total errors: $TOTAL_ERRORS" >> "$REPORT_FILE"
    echo "- Boundary errors: $BOUNDARY_ERRORS" >> "$REPORT_FILE"
    echo "- Violations: 0" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    echo "✅ All package boundary functions use RichError as required by ADR-006" >> "$REPORT_FILE"
fi

echo -e "${BLUE}📋 Report saved to: $REPORT_FILE${NC}"

# Show top boundary functions by domain
echo ""
echo -e "${BLUE}📊 Boundary Functions by Domain:${NC}"
jq -r 'to_entries[] | select(.value.type == "BOUNDARY") | .key' "$BOUNDARIES_FILE" | \
    cut -d: -f1 | \
    sed 's|^\./||' | \
    cut -d/ -f1-3 | \
    sort | uniq -c | sort -nr | head -10 | \
    while read count path; do
        echo "  $count functions in $path"
    done

echo ""
echo -e "${BLUE}🎯 ADR-006 Boundary Policy Summary:${NC}"
echo "  ✅ Exported functions → RichError required"
echo "  ✅ Interface implementations → RichError required"
echo "  ✅ Public APIs → RichError required"
echo "  ✅ Internal functions → fmt.Errorf allowed"
echo ""

# Cleanup
rm -f "$VIOLATIONS_FILE"
