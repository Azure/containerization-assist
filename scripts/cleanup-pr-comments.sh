#!/bin/bash
# Script to manually clean up duplicate PR comments

set -euo pipefail

# Configuration
GITHUB_REPOSITORY=${GITHUB_REPOSITORY:-"Azure/container-kit"}
GITHUB_PR_NUMBER=${1:-""}

if [[ -z "$GITHUB_PR_NUMBER" ]]; then
    echo "Usage: $0 <PR_NUMBER>"
    echo "Example: $0 123"
    exit 1
fi

echo "🧹 Cleaning up old automated comments on PR #$GITHUB_PR_NUMBER..."

# Function to safely delete comments
delete_comments_containing() {
    local search_text="$1"
    local comment_type="$2"

    echo "Removing old $comment_type comments..."

    gh pr view "$GITHUB_PR_NUMBER" --json comments --jq ".comments[] | select(.body | contains(\"$search_text\")) | .id" | while read -r comment_id; do
        if [[ -n "$comment_id" ]]; then
            echo "  - Deleting comment #$comment_id"
            gh api "/repos/$GITHUB_REPOSITORY/issues/comments/$comment_id" --method DELETE || echo "    Warning: Failed to delete comment #$comment_id"
        fi
    done
}

# Remove various types of old automated comments
delete_comments_containing "🧪 Testing Progress Dashboard" "testing dashboard"
delete_comments_containing "Core Package Coverage Report" "coverage report"
delete_comments_containing "🤖 CI Status Summary" "CI status"
delete_comments_containing "Lint Report -" "lint report"
delete_comments_containing "🔒 Security Scan" "security scan"

echo "✅ Cleanup completed for PR #$GITHUB_PR_NUMBER"
echo ""
echo "Now run the testing dashboard script to create a new consolidated comment:"
echo "  ./scripts/generate-testing-report.sh"
