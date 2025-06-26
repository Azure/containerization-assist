#!/bin/bash
# Merge completed workstream into main feature branch

if [ $# -eq 0 ]; then
    echo "Usage: $0 <workstream-name> [workstream-name2] ..."
    echo "Example: $0 core quality ops"
    exit 1
fi

# Save current directory and branch
ORIGINAL_DIR=$(pwd)
CURRENT_BRANCH=$(git branch --show-current)

echo "🔄 Merging completed workstreams into $CURRENT_BRANCH..."

# Fetch latest changes
echo "📥 Fetching latest changes..."
git fetch origin

# Merge each specified workstream
for WORKSTREAM in "$@"; do
    BRANCH_NAME="workstream/$WORKSTREAM"
    echo ""
    echo "🔀 Merging $BRANCH_NAME..."

    # Check if branch exists
    if ! git ls-remote --exit-code --heads origin "$BRANCH_NAME" >/dev/null 2>&1; then
        echo "  ❌ Branch $BRANCH_NAME does not exist on remote"
        continue
    fi

    # Merge the workstream
    if git merge "origin/$BRANCH_NAME" --no-edit; then
        echo "  ✅ Successfully merged $BRANCH_NAME"
    else
        echo "  ⚠️  Merge conflicts detected for $BRANCH_NAME"
        echo "     Resolve conflicts and run 'git commit' to complete the merge"
        echo "     Then re-run this script for remaining workstreams"
        exit 1
    fi
done

echo ""
echo "🔨 Running integration tests..."

# Run tests to ensure integration works
if go test -short ./... >/dev/null 2>&1; then
    echo "✅ Integration tests passed"
else
    echo "⚠️  Integration tests failed - review before pushing"
    echo "Run 'go test -short ./...' for details"
fi

# Run build check
if go build ./... >/dev/null 2>&1; then
    echo "✅ Integration build successful"
else
    echo "⚠️  Integration build failed - review before pushing"
    echo "Run 'go build ./...' for details"
fi

echo ""
echo "🎉 Workstream merge complete!"
echo "💡 Next steps:"
echo "   1. Review the changes: git log --oneline -10"
echo "   2. Push to remote: git push origin $CURRENT_BRANCH"
echo "   3. Run sync script to update other workstreams: ./scripts/sync-worktrees.sh"
