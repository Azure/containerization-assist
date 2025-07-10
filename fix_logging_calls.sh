#!/bin/bash

# Script to fix logging calls from zerolog style to slog style

echo "Fixing logging calls in application layer..."

# Find all Go files in the application layer
find pkg/mcp/application -name "*.go" -type f | while read -r file; do
    # Skip if file doesn't contain the old logging pattern
    if ! grep -q "logger\.\(Debug\|Info\|Warn\|Error\)()" "$file" 2>/dev/null; then
        continue
    fi
    
    echo "Processing: $file"
    
    # Create temporary file
    tmp_file="${file}.tmp"
    
    # Process the file with sed to fix simple cases
    sed -E '
        # Match logger.Level() pattern and convert to logger.Level("message",
        s/([[:space:]]*)([a-zA-Z]+\.)?logger\.(Debug|Info|Warn|Error)\(\)[[:space:]]*\./\1\2logger.\3("PLACEHOLDER",/g
    ' "$file" > "$tmp_file"
    
    # Move temporary file back
    mv "$tmp_file" "$file"
done

echo "Now you need to manually fix the PLACEHOLDER messages and convert the chained calls to slog style"
echo "Example:"
echo "  Old: logger.Info().Str(\"key\", value).Msg(\"message\")"
echo "  New: logger.Info(\"message\", \"key\", value)"