#!/bin/bash

# Script to manage cyclomatic complexity baseline and ratcheting

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
BASELINE_FILE="$ROOT_DIR/.complexity-baseline"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to install gocyclo if not present
install_gocyclo() {
    if ! command -v gocyclo &> /dev/null; then
        echo "Installing gocyclo..."
        go install github.com/fzipp/gocyclo/cmd/gocyclo@latest
    fi
}

# Function to generate complexity report
generate_report() {
    local threshold=$1
    echo "Analyzing cyclomatic complexity (threshold: $threshold)..."
    gocyclo -over "$threshold" . | grep -v vendor/ | sort -rn
}

# Function to count issues at different thresholds
count_issues() {
    local threshold=$1
    gocyclo -over "$threshold" . | grep -v vendor/ | wc -l
}

# Main command handling
case "$1" in
    "baseline")
        install_gocyclo
        echo "Setting complexity baseline..."

        # Count at different thresholds
        echo "Functions with complexity > 30: $(count_issues 30)"
        echo "Functions with complexity > 20: $(count_issues 20)"
        echo "Functions with complexity > 15: $(count_issues 15)"
        echo "Functions with complexity > 10: $(count_issues 10)"

        # Save current state
        echo "# Complexity Baseline - $(date)" > "$BASELINE_FILE"
        echo "threshold_30=$(count_issues 30)" >> "$BASELINE_FILE"
        echo "threshold_20=$(count_issues 20)" >> "$BASELINE_FILE"
        echo "threshold_15=$(count_issues 15)" >> "$BASELINE_FILE"
        echo "threshold_10=$(count_issues 10)" >> "$BASELINE_FILE"

        echo -e "${GREEN}✅ Baseline saved to $BASELINE_FILE${NC}"
        ;;

    "check")
        install_gocyclo
        threshold=${2:-15}

        # Load baseline if exists
        if [ -f "$BASELINE_FILE" ]; then
            source "$BASELINE_FILE"
            baseline_count=$(eval echo \$threshold_$threshold)
        else
            baseline_count=0
        fi

        current_count=$(count_issues "$threshold")

        echo "Complexity check (threshold: $threshold)"
        echo "Baseline: $baseline_count functions"
        echo "Current: $current_count functions"

        if [ "$current_count" -gt "$baseline_count" ]; then
            echo -e "${RED}❌ FAILED: Complexity increased by $((current_count - baseline_count)) functions${NC}"
            echo ""
            echo "New complex functions:"
            generate_report "$threshold" | head -20
            exit 1
        else
            improvement=$((baseline_count - current_count))
            echo -e "${GREEN}✅ PASSED: Complexity improved by $improvement functions${NC}"
        fi
        ;;

    "report")
        install_gocyclo
        threshold=${2:-15}
        generate_report "$threshold"
        ;;

    "top")
        install_gocyclo
        count=${2:-20}
        echo "Top $count most complex functions:"
        gocyclo . | grep -v vendor/ | sort -rn | head -"$count"
        ;;

    *)
        echo "Usage: $0 {baseline|check [threshold]|report [threshold]|top [count]}"
        echo ""
        echo "Commands:"
        echo "  baseline          Set current complexity as baseline"
        echo "  check [threshold] Check if complexity improved (default: 15)"
        echo "  report [threshold] Show functions above threshold (default: 15)"
        echo "  top [count]       Show top N complex functions (default: 20)"
        exit 1
        ;;
esac
