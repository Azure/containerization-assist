#!/bin/bash
set -e

RESULTS_FILE=$1

# Parse results
FILE_LENGTH_PASSED=$(jq -r '.file_length.passed' $RESULTS_FILE)
COMPLEXITY_PASSED=$(jq -r '.complexity.passed' $RESULTS_FILE)
CYCLES_PASSED=$(jq -r '.import_cycles.passed' $RESULTS_FILE)
DEPTH_PASSED=$(jq -r '.package_depth.passed' $RESULTS_FILE)
CONSTRUCTOR_PASSED=$(jq -r '.constructors.passed' $RESULTS_FILE)
PRINT_PASSED=$(jq -r '.print_statements.passed' $RESULTS_FILE)
DOC_COVERAGE=$(jq -r '.documentation.coverage' $RESULTS_FILE)

# Count violations
FILE_LENGTH_COUNT=$(jq -r '.file_length.violations | length' $RESULTS_FILE)
COMPLEXITY_COUNT=$(jq -r '.complexity.violations | length' $RESULTS_FILE)
CONSTRUCTOR_COUNT=$(jq -r '.constructors.violations | length' $RESULTS_FILE)
PRINT_COUNT=$(jq -r '.print_statements.violations | length' $RESULTS_FILE)
CONTEXT_COUNT=$(jq -r '.context.violations | length' $RESULTS_FILE)
DEPTH_COUNT=$(jq -r '.package_depth.violations | length' $RESULTS_FILE)

# Generate summary
cat << EOF
### Quality Gates Results

| Check | Status | Details |
|-------|--------|---------|
| File Length (≤800) | $([ "$FILE_LENGTH_PASSED" = "true" ] && echo "✅ Pass" || echo "❌ Fail") | $FILE_LENGTH_COUNT violations |
| Complexity (≤15) | $([ "$COMPLEXITY_PASSED" = "true" ] && echo "✅ Pass" || echo "❌ Fail") | $COMPLEXITY_COUNT violations |
| Import Cycles | $([ "$CYCLES_PASSED" = "true" ] && echo "✅ Pass" || echo "❌ Fail") | $([ "$CYCLES_PASSED" = "true" ] && echo "None detected" || echo "Cycles found") |
| Package Depth (≤5) | $([ "$DEPTH_PASSED" = "true" ] && echo "✅ Pass" || echo "❌ Fail") | $DEPTH_COUNT violations |
| Constructors (≤5 params) | $([ "$CONSTRUCTOR_PASSED" = "true" ] && echo "✅ Pass" || echo "❌ Fail") | $CONSTRUCTOR_COUNT violations |
| No Print Statements | $([ "$PRINT_PASSED" = "true" ] && echo "✅ Pass" || echo "❌ Fail") | $PRINT_COUNT found |
| Documentation | $([ $DOC_COVERAGE -ge 80 ] && echo "✅ Pass" || echo "⚠️ Warning") | ${DOC_COVERAGE}% coverage |
| Context Usage | $([ $CONTEXT_COUNT -eq 0 ] && echo "✅ Pass" || echo "⚠️ Warning") | $CONTEXT_COUNT context.Background() calls |

EOF

# Add violation details if any
if [ "$FILE_LENGTH_PASSED" != "true" ] || [ "$COMPLEXITY_PASSED" != "true" ] || [ "$CONSTRUCTOR_PASSED" != "true" ] || [ "$DEPTH_PASSED" != "true" ]; then
  echo "<details>"
  echo "<summary>📋 Violation Details</summary>"
  echo ""

  if [ "$FILE_LENGTH_PASSED" != "true" ] && [ "$FILE_LENGTH_COUNT" -gt 0 ]; then
    echo "#### Files Exceeding 800 Lines:"
    jq -r '.file_length.violations[] | "- \(.file): \(.lines) lines"' $RESULTS_FILE 2>/dev/null || true
    echo ""
  fi

  if [ "$COMPLEXITY_PASSED" != "true" ] && [ "$COMPLEXITY_COUNT" -gt 0 ]; then
    echo "#### Functions Exceeding Complexity 15:"
    jq -r '.complexity.violations[] | "- \(.function): complexity \(.complexity)"' $RESULTS_FILE 2>/dev/null || true
    echo ""
  fi

  if [ "$CONSTRUCTOR_PASSED" != "true" ] && [ "$CONSTRUCTOR_COUNT" -gt 0 ]; then
    echo "#### Constructors with >5 Parameters:"
    jq -r '.constructors.violations[] | "- \(.file): \(.params) parameters"' $RESULTS_FILE 2>/dev/null || true
    echo ""
  fi

  if [ "$DEPTH_PASSED" != "true" ] && [ "$DEPTH_COUNT" -gt 0 ]; then
    echo "#### Packages Exceeding Depth 5:"
    jq -r '.package_depth.violations[] | "- \(.path): depth \(.depth)"' $RESULTS_FILE 2>/dev/null || true
    echo ""
  fi

  echo "</details>"
fi

# Add context usage details if any
if [ "$CONTEXT_COUNT" -gt 0 ]; then
  echo ""
  echo "<details>"
  echo "<summary>⚠️ Context Usage Warnings</summary>"
  echo ""
  echo "#### context.Background() Usage:"
  jq -r '.context.violations[] | "- \(.location)"' $RESULTS_FILE 2>/dev/null || true
  echo ""
  echo "_Consider propagating context from parent functions instead of using context.Background()_"
  echo "</details>"
fi
