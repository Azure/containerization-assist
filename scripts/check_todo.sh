#!/bin/bash
# Check for TODO comments in code (excluding test files and internal packages)
# Exit with 1 if any TODOs found to fail CI

# Path to allowed TODOs file
ALLOWED_TODOS_FILE=".allowed-todos"

# Find TODO comments but exclude TODO? (which is allowed)
# Also exclude documentation files that talk about TODOs
TODOS=$(grep -R --line-number 'TODO' pkg/mcp | \
  grep -v 'TODO?' | \
  grep -vE 'pkg/mcp/internal|_test.go|\.md:')

# Filter out allowed TODOs
UNTRACKED_TODOS=""
while IFS= read -r line; do
  if [ -n "$line" ]; then
    # Extract filename and check if this TODO is allowed
    filename=$(echo "$line" | cut -d: -f1)
    is_allowed=false

    # Check against allowed TODOs
    if [ -f "$ALLOWED_TODOS_FILE" ]; then
      while IFS= read -r allowed; do
        # Skip comments and empty lines
        if [[ "$allowed" =~ ^#.*$ ]] || [ -z "$allowed" ]; then
          continue
        fi

        allowed_file=$(echo "$allowed" | cut -d: -f1)
        allowed_pattern=$(echo "$allowed" | cut -d: -f2)

        if [[ "$filename" == "$allowed_file" ]] && [[ "$line" == *"$allowed_pattern"* ]]; then
          is_allowed=true
          break
        fi
      done < "$ALLOWED_TODOS_FILE"
    fi

    if [ "$is_allowed" = false ]; then
      UNTRACKED_TODOS="${UNTRACKED_TODOS}${line}\n"
    fi
  fi
done <<< "$TODOS"

# Count untracked TODOs
if [ -n "$UNTRACKED_TODOS" ]; then
  TODO_COUNT=$(echo -e "$UNTRACKED_TODOS" | grep -c .)
  echo "❌ Found $TODO_COUNT untracked TODO comments in code:"
  echo ""
  echo -e "$UNTRACKED_TODOS"
  echo ""
  echo "Please either:"
  echo "1. Convert these TODOs to GitHub issues, or"
  echo "2. Add them to TODO.md and .allowed-todos if they're legitimate placeholders"
  exit 1
else
  echo "✅ All TODO comments are tracked in TODO.md"
  exit 0
fi
