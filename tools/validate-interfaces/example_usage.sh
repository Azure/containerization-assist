#!/bin/bash

# Example usage of the enhanced validate-interfaces tool with metrics tracking

echo "Running Interface Validation with Metrics Tracking"
echo "================================================="

# Run basic validation
echo "1. Running basic validation..."
go run . --verbose

echo -e "\n2. Running validation with metrics generation..."
# Run with metrics generation
go run . --metrics --metrics-output=detailed_interface_metrics.json --verbose

echo -e "\n3. Generated metrics files:"
ls -la *.json

echo -e "\n4. Quick metrics summary:"
if [ -f "detailed_interface_metrics.json" ]; then
    echo "Total interfaces found: $(jq '.total_interfaces' detailed_interface_metrics.json)"
    echo "Total implementors found: $(jq '.total_implementors' detailed_interface_metrics.json)"
    echo "Overall compliance: $(jq '.compliance_report.overall_compliance' detailed_interface_metrics.json)%"
    echo "Migration rate: $(jq '.pattern_analysis.pattern_migration_rate' detailed_interface_metrics.json)%"
    
    echo -e "\nTop interface by adoption:"
    jq -r '.interface_stats | to_entries | sort_by(.value.implementor_count) | reverse | .[0] | "\(.key): \(.value.implementor_count) implementations"' detailed_interface_metrics.json
    
    echo -e "\nRecommendations:"
    jq -r '.recommendations[]' detailed_interface_metrics.json | sed 's/^/  • /'
fi