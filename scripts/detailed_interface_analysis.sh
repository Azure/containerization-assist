#!/bin/bash
# Detailed Interface Analysis for 75% Reduction Planning

echo "=== DETAILED INTERFACE ANALYSIS FOR 75% REDUCTION ==="
echo "Date: $(date)"
echo ""

# Create comprehensive interface inventory
echo "🔍 COMPREHENSIVE INTERFACE INVENTORY:"
echo ""

# Tool interfaces breakdown
echo "=== TOOL INTERFACES BREAKDOWN ==="
echo "Tool interfaces by file:"
grep -r "type.*Tool.*interface" pkg/mcp --include="*.go" | while IFS=: read -r file line; do
    interface_name=$(echo "$line" | sed 's/.*type \([^ ]*\).*/\1/')
    usage_count=$(grep -r "$interface_name" pkg/mcp --include="*.go" | wc -l)
    echo "  $interface_name ($file): $usage_count usages"
done | sort -k3 -nr

echo ""
echo "=== REGISTRY INTERFACES BREAKDOWN ==="
echo "Registry interfaces by file:"
grep -r "type.*Registry.*interface" pkg/mcp --include="*.go" | while IFS=: read -r file line; do
    interface_name=$(echo "$line" | sed 's/.*type \([^ ]*\).*/\1/')
    usage_count=$(grep -r "$interface_name" pkg/mcp --include="*.go" | wc -l)
    echo "  $interface_name ($file): $usage_count usages"
done | sort -k3 -nr

echo ""
echo "=== ALL INTERFACES BY PACKAGE ==="
for package in $(find pkg/mcp -type d -name "*" | sort); do
    if [ -n "$(find $package -maxdepth 1 -name "*.go" 2>/dev/null)" ]; then
        interface_count=$(find $package -maxdepth 1 -name "*.go" -exec grep -c "interface {" {} \; 2>/dev/null | paste -sd+ | bc 2>/dev/null || echo 0)
        if [ "$interface_count" -gt 0 ]; then
            echo "  $package: $interface_count interfaces"
        fi
    fi
done

echo ""
echo "=== INTERFACE COMPLEXITY ANALYSIS ==="
echo "Large interfaces (>5 methods):"
find pkg/mcp -name "*.go" -exec awk '
/^type .* interface \{/ {
    interface_name = $2;
    method_count = 0;
    in_interface = 1
}
in_interface && /^\s*[A-Z].*\(.*\)/ {
    method_count++
}
in_interface && /^\}/ {
    if (method_count > 5) {
        print FILENAME ":" interface_name ": " method_count " methods"
    }
    in_interface = 0
}' {} \;

echo ""
echo "=== MIGRATION COMPLEXITY ASSESSMENT ==="
echo "Interfaces by migration difficulty:"
echo ""
echo "🟢 EASY (0-2 usages, deprecated/unused):"
grep -r "type.*Tool.*interface\|type.*Registry.*interface" pkg/mcp --include="*.go" | while IFS=: read -r file line; do
    interface_name=$(echo "$line" | sed 's/.*type \([^ ]*\).*/\1/')
    usage_count=$(grep -r "$interface_name" pkg/mcp --include="*.go" | grep -v "type $interface_name" | wc -l)
    if [ "$usage_count" -le 2 ]; then
        echo "  $interface_name: $usage_count usages"
    fi
done

echo ""
echo "🟡 MEDIUM (3-10 usages, straightforward replacement):"
grep -r "type.*Tool.*interface\|type.*Registry.*interface" pkg/mcp --include="*.go" | while IFS=: read -r file line; do
    interface_name=$(echo "$line" | sed 's/.*type \([^ ]*\).*/\1/')
    usage_count=$(grep -r "$interface_name" pkg/mcp --include="*.go" | grep -v "type $interface_name" | wc -l)
    if [ "$usage_count" -ge 3 ] && [ "$usage_count" -le 10 ]; then
        echo "  $interface_name: $usage_count usages"
    fi
done

echo ""
echo "🔴 HARD (10+ usages, complex dependencies):"
grep -r "type.*Tool.*interface\|type.*Registry.*interface" pkg/mcp --include="*.go" | while IFS=: read -r file line; do
    interface_name=$(echo "$line" | sed 's/.*type \([^ ]*\).*/\1/')
    usage_count=$(grep -r "$interface_name" pkg/mcp --include="*.go" | grep -v "type $interface_name" | wc -l)
    if [ "$usage_count" -gt 10 ]; then
        echo "  $interface_name: $usage_count usages"
    fi
done

echo ""
echo "=== REDUCTION TARGETS ==="
echo "Current: 170 interfaces"
echo "Target: <50 interfaces (75% reduction)"
echo "Need to remove: 120+ interfaces"
echo ""
echo "Breakdown of removal strategy needed:"
echo "- Tool interfaces: 22 → 4 (remove 18)"
echo "- Registry interfaces: 9 → 1 (remove 8)"
echo "- Other interfaces: ~139 → ~45 (remove ~94)"
