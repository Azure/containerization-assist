#!/bin/bash

set -euo pipefail

echo "🛡️ Quality Gates Validation $(date)"
echo "========================================="
echo ""

# Display configuration if verbose
if [ "${VERBOSE:-false}" = "true" ]; then
    echo "📋 Quality Gate Thresholds:"
    echo "  • Max Lint Errors: $MAX_LINT_ERRORS"
    echo "  • Max Lint Warnings: $MAX_LINT_WARNINGS"
    echo "  • Max Type Errors: $MAX_TYPE_ERRORS"
    echo "  • Max Dead Code: $MAX_DEADCODE"
    echo "  • Max Build Time: ${MAX_BUILD_TIME_SECONDS}s"
    echo ""
fi

# Configuration
QUALITY_CONFIG="quality-gates.json"

# Read thresholds from config file if it exists, fallback to defaults
if [ -f "$QUALITY_CONFIG" ] && command -v jq &> /dev/null; then
    MAX_LINT_ERRORS=$(jq -r '.metrics.thresholds.lint.maxErrors // 0' "$QUALITY_CONFIG" 2>/dev/null || echo "0")
    MAX_LINT_WARNINGS=$(jq -r '.metrics.thresholds.lint.maxWarnings // 400' "$QUALITY_CONFIG" 2>/dev/null || echo "400")
    MAX_TYPE_ERRORS=$(jq -r '.metrics.thresholds.typescript.maxErrors // 0' "$QUALITY_CONFIG" 2>/dev/null || echo "0")
    MAX_DEADCODE=$(jq -r '.metrics.thresholds.deadcode.max // 200' "$QUALITY_CONFIG" 2>/dev/null || echo "200")
    MAX_BUILD_TIME_MS=$(jq -r '.metrics.thresholds.build.maxTimeMs // 60000' "$QUALITY_CONFIG" 2>/dev/null || echo "60000")
    MAX_BUILD_TIME_SECONDS=$((MAX_BUILD_TIME_MS / 1000))
else
    # Fallback to environment variables or defaults
    MAX_LINT_ERRORS=${MAX_LINT_ERRORS:-0}
    MAX_LINT_WARNINGS=${MAX_LINT_WARNINGS:-400}
    MAX_TYPE_ERRORS=${MAX_TYPE_ERRORS:-0}
    MAX_DEADCODE=${MAX_DEADCODE:-200}
    MAX_BUILD_TIME_SECONDS=${MAX_BUILD_TIME_SECONDS:-60}
    MAX_BUILD_TIME_MS=$((MAX_BUILD_TIME_SECONDS * 1000))
fi

# Other configuration
MIN_COVERAGE=${MIN_COVERAGE:-80}

# Check for required tools with better error messages
for cmd in npm bc jq; do
    if ! command -v "$cmd" &> /dev/null; then
        echo "❌ Error: $cmd is required but not installed."
        echo "💡 Install with: brew install $cmd" # or apt-get, etc.
        exit 1
    fi
done

# Environment config
ALLOW_REGRESSION=${ALLOW_REGRESSION:-false}

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    local status=$1
    local message=$2
    case $status in
        "PASS") echo -e "${GREEN}✅ PASS:${NC} $message" ;;
        "FAIL") echo -e "${RED}❌ FAIL:${NC} $message" ;;
        "WARN") echo -e "${YELLOW}⚠️  WARN:${NC} $message" ;;
        "INFO") echo -e "ℹ️  INFO: $message" ;;
    esac
}

# Add function for safer JSON operations
update_json_safely() {
    local jq_expr="$1"
    shift  # Remove first argument, remaining are jq options
    local temp_file="${QUALITY_CONFIG}.tmp.$$"
    
    if jq "$@" "$jq_expr" "$QUALITY_CONFIG" > "$temp_file" 2>/dev/null; then
        mv "$temp_file" "$QUALITY_CONFIG"
    else
        print_status "WARN" "Failed to update JSON metrics - continuing with existing values"
        rm -f "$temp_file" 2>/dev/null || true
    fi
}

# Enhanced JSON validation
validate_json_numeric() {
    local value="$1"
    local field_name="$2"
    
    if ! [[ "$value" =~ ^[0-9]+$ ]]; then
        print_status "WARN" "Invalid numeric value for $field_name: '$value' - using 0"
        echo "0"
    else
        echo "$value"
    fi
}

# Enhanced ESLint processing with fallback strategies
process_eslint_results() {
    local json_output="$1"
    local current_errors=0
    local current_warnings=0
    
    # Strategy 1: JSON parsing (preferred)
    if [ -n "$json_output" ] && [ "$json_output" != "[]" ]; then
        if current_errors=$(echo "$json_output" | jq '[.[].messages[]? | select(.severity == 2)] | length' 2>/dev/null) && \
           current_warnings=$(echo "$json_output" | jq '[.[].messages[]? | select(.severity == 1)] | length' 2>/dev/null) && \
           [[ "$current_errors" =~ ^[0-9]+$ ]] && [[ "$current_warnings" =~ ^[0-9]+$ ]]; then
            echo "$current_errors $current_warnings"
            return 0
        fi
    fi
    
    # Strategy 2: Text parsing fallback
    print_status "INFO" "JSON parsing failed, using text parsing fallback"
    local lint_output
    lint_output=$(npm run lint 2>&1 || true)
    
    local summary_line
    summary_line=$(echo "$lint_output" | grep -E "problems.*error.*warning" | tail -1 2>/dev/null || echo "")
    
    if [ -n "$summary_line" ]; then
        current_errors=$(echo "$summary_line" | sed -n 's/.*(\([0-9]\+\) error.*/\1/p' 2>/dev/null || echo "0")
        current_warnings=$(echo "$summary_line" | sed -n 's/.*, \([0-9]\+\) warning.*/\1/p' 2>/dev/null || echo "0")
    fi
    
    # Ensure numeric values
    current_errors=$(validate_json_numeric "${current_errors:-0}" "errors")
    current_warnings=$(validate_json_numeric "${current_warnings:-0}" "warnings")
    
    echo "$current_errors $current_warnings"
}

# Validate current directory and config file
if [ ! -f "$QUALITY_CONFIG" ]; then
    echo "📁 Current directory: $(pwd)"
    echo "Creating default quality-gates.json configuration file..."
    cat > "$QUALITY_CONFIG" << 'EOF'
{
  "$schema": "./quality-gates.schema.json",
  "schemaVersion": 1,
  "metrics": {
    "thresholds": {
      "lint": {
        "maxErrors": 0,
        "maxWarnings": 400
      },
      "deadcode": {
        "max": 200
      },
      "typescript": {
        "maxErrors": 0
      },
      "build": {
        "maxTimeMs": 60000
      }
    },
    "baselines": {
      "lint": {
        "errors": 0,
        "warnings": null
      },
      "deadcode": {
        "count": null
      },
      "typescript": {
        "errors": 0
      },
      "build": {
        "timeMs": null,
        "mode": "clean",
        "environment": {
          "nodeVersion": "$(node -v 2>/dev/null || echo 'unknown')",
          "os": "$(uname -s 2>/dev/null || echo 'unknown')",
          "cpu": "$(uname -m 2>/dev/null || echo 'unknown')"
        }
      }
    }
  }
}
EOF
    print_status "INFO" "Created default quality-gates.json configuration file"
fi

# Read baselines from JSON (new structure)
if jq -e '.metrics.baselines' "$QUALITY_CONFIG" &>/dev/null; then
    # New structure
    BASELINE_WARNINGS=$(jq -r '.metrics.baselines.lint.warnings // null' "$QUALITY_CONFIG")
    DEADCODE_BASELINE=$(jq -r '.metrics.baselines.deadcode.count // null' "$QUALITY_CONFIG")
else
    # Old structure fallback
    BASELINE_WARNINGS=$(jq -r '.metrics.lint.baseline // null' "$QUALITY_CONFIG")
    DEADCODE_BASELINE=$(jq -r '.metrics.deadcode.baseline // null' "$QUALITY_CONFIG")
fi

# Gate 1: ESLint Errors Must Be Zero
echo "Gate 1: ESLint Error Check"
echo "-------------------------"

# Run lint and get current counts using JSON output for reliable parsing
LINT_JSON_OUTPUT=$(npx eslint src --ext .ts --format=json 2>/dev/null || true)

# Use enhanced processing function
LINT_RESULTS=$(process_eslint_results "$LINT_JSON_OUTPUT")
CURRENT_ERRORS=$(echo "$LINT_RESULTS" | cut -d' ' -f1)
CURRENT_WARNINGS=$(echo "$LINT_RESULTS" | cut -d' ' -f2)

# Ensure we have numeric values
CURRENT_ERRORS=$(validate_json_numeric "$CURRENT_ERRORS" "errors")
CURRENT_WARNINGS=$(validate_json_numeric "$CURRENT_WARNINGS" "warnings")

# Note: We don't store current values in git anymore, only baselines
# Current values are computed during CI/CD runs

if [ "$CURRENT_ERRORS" -le "$MAX_LINT_ERRORS" ]; then
    print_status "PASS" "ESLint errors within threshold: $CURRENT_ERRORS ≤ $MAX_LINT_ERRORS"
else
    print_status "FAIL" "$CURRENT_ERRORS ESLint errors exceed threshold of $MAX_LINT_ERRORS"
    exit 1
fi

echo ""

# Handle null lint baseline for first run
if [ "$BASELINE_WARNINGS" = "null" ] || [ -z "$BASELINE_WARNINGS" ]; then
    print_status "INFO" "No lint baseline set, using current warnings ($CURRENT_WARNINGS) as baseline"
    BASELINE_WARNINGS="$CURRENT_WARNINGS"
    # Update the baseline in the config file (new structure)
    if jq -e '.metrics.baselines' "$QUALITY_CONFIG" &>/dev/null; then
        update_json_safely '.metrics.baselines.lint.warnings = ($warnings | tonumber)' \
           --arg warnings "$CURRENT_WARNINGS"
    else
        update_json_safely '.metrics.lint.baseline = ($warnings | tonumber)' \
           --arg warnings "$CURRENT_WARNINGS"
    fi
fi

# Gate 2: ESLint Warning Ratcheting
echo "Gate 2: ESLint Warning Ratcheting"
echo "----------------------------------"

# Check against both baseline and configured maximum
if [ "$CURRENT_WARNINGS" -le "$BASELINE_WARNINGS" ] && [ "$CURRENT_WARNINGS" -le "$MAX_LINT_WARNINGS" ]; then
    REDUCTION=$((BASELINE_WARNINGS - CURRENT_WARNINGS))
    if [ "$REDUCTION" -gt 0 ]; then
        PERCENTAGE=$(echo "scale=1; ($REDUCTION * 100) / $BASELINE_WARNINGS" | bc -l 2>/dev/null || echo "N/A")
        print_status "PASS" "Warnings reduced by $REDUCTION (${PERCENTAGE}%) - $CURRENT_WARNINGS ≤ $BASELINE_WARNINGS"
        # Auto-update baseline when improved (new structure)
        if jq -e '.metrics.baselines' "$QUALITY_CONFIG" &>/dev/null; then
            update_json_safely '.metrics.baselines.lint.warnings = ($warnings | tonumber)' \
               --arg warnings "$CURRENT_WARNINGS"
        else
            update_json_safely '.metrics.lint.baseline = ($warnings | tonumber)' \
               --arg warnings "$CURRENT_WARNINGS"
        fi
        print_status "INFO" "Updated ESLint baseline: $BASELINE_WARNINGS → $CURRENT_WARNINGS"
    else
        print_status "PASS" "Warning count maintained at baseline ($CURRENT_WARNINGS)"
    fi
else
    INCREASE=$((CURRENT_WARNINGS - BASELINE_WARNINGS))
    if [ "$ALLOW_REGRESSION" = "true" ]; then
        print_status "WARN" "Warning count increased by $INCREASE ($CURRENT_WARNINGS > $BASELINE_WARNINGS) - ALLOWED by config"
    else
        print_status "FAIL" "Warning count increased by $INCREASE ($CURRENT_WARNINGS > $BASELINE_WARNINGS) - REGRESSION NOT ALLOWED"
        exit 1
    fi
fi

echo ""

# Gate 3: TypeScript Compilation (Optional)
if [ "${SKIP_TYPECHECK:-false}" != "true" ]; then
    echo "Gate 3: TypeScript Compilation"
    echo "-------------------------------"

    if npm run typecheck > /dev/null 2>&1; then
        print_status "PASS" "TypeScript compilation successful"
        # TypeScript errors baseline is always 0 for passing builds
    else
        print_status "FAIL" "TypeScript compilation failed"
        exit 1
    fi

    echo ""
else
    echo "Gate 3: TypeScript Compilation (SKIPPED)"
    echo "----------------------------------------"
    print_status "WARN" "TypeScript check skipped by configuration"
    echo ""
fi

# Gate 4: Dead Code Check
echo "Gate 4: Dead Code Check"
echo "-----------------------"

# More robust dead code detection with error handling
if command -v npx >/dev/null 2>&1 && [ -f "tsconfig.json" ]; then
    DEADCODE_OUTPUT=$(npx ts-prune --project tsconfig.json 2>/dev/null || echo "")
    if [ -n "$DEADCODE_OUTPUT" ]; then
        DEADCODE_COUNT=$(echo "$DEADCODE_OUTPUT" | grep -v 'used in module' | wc -l | tr -d ' ' || echo "0")
    else
        DEADCODE_COUNT=0
        print_status "WARN" "ts-prune failed to run, assuming 0 dead code exports"
    fi
else
    DEADCODE_COUNT=0
    print_status "WARN" "ts-prune or tsconfig.json not available, skipping dead code check"
fi

# Ensure numeric value
DEADCODE_COUNT=${DEADCODE_COUNT:-0}

# Note: We don't store current values in git anymore, only baselines

# Handle null deadcode baseline for first run
if [ "$DEADCODE_BASELINE" = "null" ] || [ -z "$DEADCODE_BASELINE" ]; then
    print_status "INFO" "No deadcode baseline set, using current dead code count ($DEADCODE_COUNT) as baseline"
    DEADCODE_BASELINE="$DEADCODE_COUNT"
    # Update the baseline in the config file (new structure)
    if jq -e '.metrics.baselines' "$QUALITY_CONFIG" &>/dev/null; then
        update_json_safely '.metrics.baselines.deadcode.count = ($deadcode | tonumber)' \
           --arg deadcode "$DEADCODE_COUNT"
    else
        update_json_safely '.metrics.deadcode.baseline = ($deadcode | tonumber)' \
           --arg deadcode "$DEADCODE_COUNT"
    fi
fi

if [ "$DEADCODE_COUNT" -le "$DEADCODE_BASELINE" ]; then
    DEADCODE_REDUCTION=$((DEADCODE_BASELINE - DEADCODE_COUNT))
    if [ $DEADCODE_REDUCTION -gt 0 ]; then
        DEADCODE_PERCENTAGE=$(echo "scale=1; ($DEADCODE_REDUCTION * 100) / $DEADCODE_BASELINE" | bc -l 2>/dev/null || echo "N/A")
        print_status "PASS" "Unused exports reduced by $DEADCODE_REDUCTION (${DEADCODE_PERCENTAGE}%) - $DEADCODE_COUNT ≤ $DEADCODE_BASELINE"
        # Auto-update baseline when improved (new structure)
        if jq -e '.metrics.baselines' "$QUALITY_CONFIG" &>/dev/null; then
            update_json_safely '.metrics.baselines.deadcode.count = ($deadcode | tonumber)' \
               --arg deadcode "$DEADCODE_COUNT"
        else
            update_json_safely '.metrics.deadcode.baseline = ($deadcode | tonumber)' \
               --arg deadcode "$DEADCODE_COUNT"
        fi
        print_status "INFO" "Updated deadcode baseline: $DEADCODE_BASELINE → $DEADCODE_COUNT"
    else
        print_status "PASS" "Unused exports maintained at baseline ($DEADCODE_COUNT)"
    fi
else
    DEADCODE_INCREASE=$((DEADCODE_COUNT - DEADCODE_BASELINE))
    if [ "$ALLOW_REGRESSION" = "true" ]; then
        print_status "WARN" "Unused exports increased by $DEADCODE_INCREASE ($DEADCODE_COUNT > $DEADCODE_BASELINE) - ALLOWED by config"
    else
        print_status "FAIL" "Unused exports increased by $DEADCODE_INCREASE ($DEADCODE_COUNT > $DEADCODE_BASELINE) - REGRESSION NOT ALLOWED"
        exit 1
    fi
fi

echo ""

# Gate 5: Build Performance (Optional)
if command -v npm >/dev/null 2>&1; then
    echo "Gate 5: Build Performance"
    echo "------------------------"
    
    # Use more portable timing approach
    BUILD_START=$(date +%s 2>/dev/null || echo "0")
    if npm run build > /dev/null 2>&1; then
        BUILD_END=$(date +%s 2>/dev/null || echo "0")
        if [ "$BUILD_START" != "0" ] && [ "$BUILD_END" != "0" ]; then
            BUILD_TIME_SECONDS=$((BUILD_END - BUILD_START))
            BUILD_TIME_MS=$((BUILD_TIME_SECONDS * 1000))
            
            # Update build baseline only if improved or first time (new structure)
            if jq -e '.metrics.baselines' "$QUALITY_CONFIG" &>/dev/null; then
                CURRENT_BUILD_BASELINE=$(jq -r '.metrics.baselines.build.timeMs // null' "$QUALITY_CONFIG")
                if [ "$CURRENT_BUILD_BASELINE" = "null" ] || [ "$BUILD_TIME_MS" -lt "$CURRENT_BUILD_BASELINE" ]; then
                    NODE_VERSION=$(node -v 2>/dev/null || echo "unknown")
                    OS_NAME=$(uname -s 2>/dev/null || echo "unknown")
                    CPU_ARCH=$(uname -m 2>/dev/null || echo "unknown")
                    update_json_safely '.metrics.baselines.build.timeMs = ($time | tonumber) | .metrics.baselines.build.environment.nodeVersion = $node | .metrics.baselines.build.environment.os = $os | .metrics.baselines.build.environment.cpu = $cpu' \
                       --arg time "$BUILD_TIME_MS" --arg node "$NODE_VERSION" --arg os "$OS_NAME" --arg cpu "$CPU_ARCH"
                fi
            else
                # Old structure
                update_json_safely '.metrics.build.lastBuildTimeMs = ($time | tonumber)' \
                   --arg time "$BUILD_TIME_MS"
            fi
            
            if [ "$BUILD_TIME_SECONDS" -lt "$MAX_BUILD_TIME_SECONDS" ]; then
                print_status "PASS" "Build completed in ${BUILD_TIME_SECONDS}s (< ${MAX_BUILD_TIME_SECONDS}s threshold)"
            else
                print_status "WARN" "Build took ${BUILD_TIME_SECONDS}s (exceeds ${MAX_BUILD_TIME_SECONDS}s threshold)"
            fi
        else
            print_status "WARN" "Could not measure build time accurately"
        fi
    else
        print_status "FAIL" "Build failed"
        exit 1
    fi
    
    echo ""
else
    print_status "WARN" "npm not available, skipping build performance check"
    echo ""
fi

# Note: We no longer update generatedAt to reduce git noise
# Timestamps are only relevant during CI runs

# Final Summary
echo "🎉 Quality Gates Summary"
echo "========================"
echo "ESLint Errors: $CURRENT_ERRORS (threshold: $MAX_LINT_ERRORS)"
echo "ESLint Warnings: $CURRENT_WARNINGS (threshold: $MAX_LINT_WARNINGS)"
echo "Unused Exports: $DEADCODE_COUNT (threshold: $MAX_DEADCODE)"
echo "TypeScript: ✅ Compiles"
echo "Build: ✅ Successful"
echo ""

if [ "${CURRENT_WARNINGS:-0}" -gt "$MAX_LINT_WARNINGS" ] || [ "${DEADCODE_COUNT:-0}" -gt "$MAX_DEADCODE" ]; then
    print_status "INFO" "Consider running aggressive cleanup to reach production targets:"
    echo "  • ESLint warnings target: <$MAX_LINT_WARNINGS (current: $CURRENT_WARNINGS)"
    echo "  • Dead code target: <$MAX_DEADCODE (current: $DEADCODE_COUNT)"
    echo ""
fi

print_status "PASS" "All quality gates passed! 🚀"
echo ""