#!/bin/bash

echo "Finding all files with zerolog-style logging calls..."

# Find all files with the old logging pattern
grep -r "logger\.\(Debug\|Info\|Warn\|Error\)()\." pkg/mcp/application --include="*.go" -l | sort | uniq > zerolog_files.txt

echo "Found $(wc -l < zerolog_files.txt) files with zerolog-style logging"
echo "Files saved to zerolog_files.txt"

# Show a sample of the patterns found
echo -e "\nSample patterns found:"
grep -r "logger\.\(Debug\|Info\|Warn\|Error\)()\." pkg/mcp/application --include="*.go" -n | head -20