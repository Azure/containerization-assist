#!/bin/bash

# Fix Messagef with %w directive by converting to Message().Cause() pattern
# This script handles the conversion of error-wrapping patterns in RichError

set -e

echo "🔧 Fixing Messagef error-wrapping patterns..."

# Create a temporary file to store the sed script
cat > /tmp/fix_messagef.sed << 'EOF'
# Pattern 1: Simple case with only %w at the end
s/\.Messagef("\([^"]*\): %w", err)/\.Message("\1").Cause(err)/g
s/\.Messagef("\([^"]*\) %w", err)/\.Message("\1").Cause(err)/g
s/\.Messagef("\([^"]*\)%w", err)/\.Message("\1").Cause(err)/g
s/\.Messagef('\([^']*\): %w', err)/\.Message('\1').Cause(err)/g
s/\.Messagef('\([^']*\) %w', err)/\.Message('\1').Cause(err)/g

# Pattern 2: With different error variable names
s/\.Messagef("\([^"]*\): %w", \([a-zA-Z0-9_]*\))/\.Message("\1").Cause(\2)/g
s/\.Messagef("\([^"]*\) %w", \([a-zA-Z0-9_]*\))/\.Message("\1").Cause(\2)/g
s/\.Messagef("\([^"]*\)%w", \([a-zA-Z0-9_]*\))/\.Message("\1").Cause(\2)/g

# Pattern 3: With line breaks (multiline)
s/\.Messagef("\([^"]*\): %w",$/\.Message("\1").Cause(/g
s/\.Messagef("\([^"]*\) %w",$/\.Message("\1").Cause(/g
s/\.Messagef("\([^"]*\)%w",$/\.Message("\1").Cause(/g
EOF

# Find all Go files in pkg/mcp that might have the pattern
find pkg/mcp -name "*.go" -type f | while read -r file; do
    # Check if file contains Messagef with %w
    if grep -q 'Messagef.*%w' "$file" 2>/dev/null; then
        echo "Processing: $file"

        # Create a backup
        cp "$file" "$file.bak"

        # First pass: handle simple cases with sed
        sed -i -f /tmp/fix_messagef.sed "$file"

        # Second pass: handle complex cases with a Go program
        go run - "$file" << 'GOEOF'
package main

import (
    "bufio"
    "fmt"
    "os"
    "regexp"
    "strings"
)

func main() {
    if len(os.Args) < 2 {
        fmt.Println("Usage: fix_messagef <file>")
        os.Exit(1)
    }

    filename := os.Args[1]
    content, err := os.ReadFile(filename)
    if err != nil {
        fmt.Printf("Error reading file: %v\n", err)
        os.Exit(1)
    }

    lines := strings.Split(string(content), "\n")

    // Pattern to match Messagef with format parameters and %w
    messagePattern := regexp.MustCompile(`\.Messagef\("([^"]*?)(%[^w][^"]*?)*%w"(.*?)\)`)
    messagePatternWithArgs := regexp.MustCompile(`\.Messagef\("([^"]*?)%w"(,\s*)([^,\)]+)(.*?)\)`)
    complexPattern := regexp.MustCompile(`\.Messagef\("([^"]+)"(,\s*[^,\)]+)*,\s*([a-zA-Z0-9_]+)\)`)

    for i, line := range lines {
        // Skip if already fixed
        if strings.Contains(line, ".Cause(") {
            continue
        }

        // Check for Messagef with %w
        if strings.Contains(line, "Messagef") && strings.Contains(line, "%w") {
            // Handle cases with other format verbs
            if strings.Count(line, "%") > 1 {
                // Extract the format string and arguments
                if match := complexPattern.FindStringSubmatch(line); match != nil {
                    format := match[1]
                    args := match[2]
                    errVar := match[3]

                    // Check if format contains %w
                    if strings.Contains(format, "%w") {
                        // Remove %w and its separator
                        newFormat := strings.Replace(format, ": %w", "", 1)
                        newFormat = strings.Replace(newFormat, " %w", "", 1)
                        newFormat = strings.Replace(newFormat, "%w", "", 1)

                        // Remove the error variable from args
                        newArgs := strings.TrimSuffix(args, ", "+errVar)

                        // Build the new line
                        if newArgs != "" {
                            line = strings.Replace(line,
                                fmt.Sprintf(".Messagef(\"%s\"%s, %s)", format, args, errVar),
                                fmt.Sprintf(".Message(fmt.Sprintf(\"%s\"%s)).Cause(%s)", newFormat, newArgs, errVar),
                                1)
                        } else {
                            line = strings.Replace(line,
                                fmt.Sprintf(".Messagef(\"%s\", %s)", format, errVar),
                                fmt.Sprintf(".Message(\"%s\").Cause(%s)", newFormat, errVar),
                                1)
                        }
                    }
                }
            }

            lines[i] = line
        }
    }

    // Write back
    output := strings.Join(lines, "\n")
    err = os.WriteFile(filename, []byte(output), 0644)
    if err != nil {
        fmt.Printf("Error writing file: %v\n", err)
        os.Exit(1)
    }
}
GOEOF

        # Clean up backup if successful
        if [ $? -eq 0 ]; then
            rm -f "$file.bak"
        else
            echo "Error processing $file, restoring backup"
            mv "$file.bak" "$file"
        fi
    fi
done

# Clean up
rm -f /tmp/fix_messagef.sed

echo "✅ Messagef error-wrapping patterns fixed!"
echo ""
echo "📋 Next steps:"
echo "1. Review the changes with: git diff"
echo "2. Run tests to ensure everything works: make test"
echo "3. Commit the changes if tests pass"
