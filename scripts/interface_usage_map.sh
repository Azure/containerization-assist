#!/bin/bash
# Interface Usage Mapping Script for Workstream Beta
# This script analyzes all interface usage patterns to guide migration

echo "=== INTERFACE USAGE MAPPING ===" > interface_usage_report.txt
echo "Date: $(date)" >> interface_usage_report.txt
echo "" >> interface_usage_report.txt

# Function to count interface usage
count_interface_usage() {
    local interface_name=$1
    local count=$(grep -r "$interface_name" pkg/mcp --include="*.go" 2>/dev/null | wc -l)
    echo "$count"
}

# Map Tool interface usage
echo "=== TOOL INTERFACE USAGE ===" >> interface_usage_report.txt
echo "Current Tool interfaces found:" >> interface_usage_report.txt
grep -r "type.*Tool.*interface" pkg/mcp --include="*.go" 2>/dev/null | grep -v "interfaces.go" | cut -d: -f1-2 >> interface_usage_report.txt
echo "" >> interface_usage_report.txt

# Find files implementing Tool interfaces
echo "Files implementing Tool interfaces:" >> interface_usage_report.txt
grep -r "implements.*Tool\|var.*Tool\s*=\|.*Tool\s*interface" pkg/mcp --include="*.go" 2>/dev/null | cut -d: -f1 | sort -u >> interface_usage_report.txt
echo "" >> interface_usage_report.txt

# Map specific tool interface types
echo "=== TOOL INTERFACE USAGE COUNTS ===" >> interface_usage_report.txt

# List of tool interfaces to check (based on workstream prompt)
tool_interfaces=(
    "ToolWithContext"
    "StreamingTool"
    "BatchTool"
    "AsyncTool"
    "StatefulTool"
    "ConfigurableTool"
    "ToolWithValidation"
    "TimeoutTool"
    "CircuitBreakerTool"
    "CacheableTool"
    "MetricsTool"
    "ObservableTool"
    "RetryableTool"
    "VersionedTool"
    "ComposableTool"
    "ValidatableTool"
)

for interface in "${tool_interfaces[@]}"; do
    count=$(count_interface_usage "$interface")
    echo "$interface: $count usages" >> interface_usage_report.txt
done

echo "" >> interface_usage_report.txt

# Map Registry interface usage
echo "=== REGISTRY INTERFACE USAGE ===" >> interface_usage_report.txt
echo "Current Registry interfaces found:" >> interface_usage_report.txt
grep -r "type.*Registry.*interface" pkg/mcp --include="*.go" 2>/dev/null | grep -v "interfaces.go" | cut -d: -f1-2 >> interface_usage_report.txt
echo "" >> interface_usage_report.txt

# List of registry interfaces to check
registry_interfaces=(
    "TypedRegistry"
    "ToolRegistry"
    "AsyncRegistry"
    "ConcurrentRegistry"
    "VersionedRegistry"
    "NamespacedRegistry"
    "MetricsRegistry"
    "ObservableRegistry"
    "PluginRegistry"
    "BatchRegistry"
    "CachingRegistry"
    "FilterableRegistry"
    "PersistentRegistry"
    "HierarchicalRegistry"
    "EventRegistry"
)

echo "=== REGISTRY INTERFACE USAGE COUNTS ===" >> interface_usage_report.txt
for interface in "${registry_interfaces[@]}"; do
    count=$(count_interface_usage "$interface")
    echo "$interface: $count usages" >> interface_usage_report.txt
done

echo "" >> interface_usage_report.txt

# Find ValidationResult definitions
echo "=== VALIDATION RESULT DEFINITIONS ===" >> interface_usage_report.txt
echo "Files with ValidationResult definitions:" >> interface_usage_report.txt
find pkg/mcp -name "*.go" -exec grep -l "type ValidationResult" {} \; 2>/dev/null >> interface_usage_report.txt
echo "" >> interface_usage_report.txt

# Priority mapping - most used interfaces first
echo "=== MIGRATION PRIORITY (by usage count) ===" >> interface_usage_report.txt
echo "Tool interfaces by usage:" >> interface_usage_report.txt
for interface in "${tool_interfaces[@]}"; do
    count=$(count_interface_usage "$interface")
    echo "$count $interface"
done | sort -rn >> interface_usage_report.txt

echo "" >> interface_usage_report.txt
echo "Registry interfaces by usage:" >> interface_usage_report.txt
for interface in "${registry_interfaces[@]}"; do
    count=$(count_interface_usage "$interface")
    echo "$count $interface"
done | sort -rn >> interface_usage_report.txt

echo "" >> interface_usage_report.txt

# Summary statistics
echo "=== SUMMARY STATISTICS ===" >> interface_usage_report.txt
echo "Total Tool interfaces: $(grep -r "type.*Tool.*interface" pkg/mcp --include="*.go" 2>/dev/null | wc -l)" >> interface_usage_report.txt
echo "Total Registry interfaces: $(grep -r "type.*Registry.*interface" pkg/mcp --include="*.go" 2>/dev/null | wc -l)" >> interface_usage_report.txt
echo "Total ValidationResult definitions: $(find pkg/mcp -name "*.go" -exec grep -l "type ValidationResult" {} \; 2>/dev/null | wc -l)" >> interface_usage_report.txt
echo "Total interfaces in pkg/mcp: $(find pkg/mcp -name "*.go" -exec grep -c "interface {" {} \; 2>/dev/null | paste -sd+ | bc)" >> interface_usage_report.txt

echo "" >> interface_usage_report.txt
echo "Report generated successfully: interface_usage_report.txt"
