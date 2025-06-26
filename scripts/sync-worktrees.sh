#!/bin/bash
# Sync all worktrees with gambtho/mcp branch (ONE-WAY ONLY)
# Uses rebase for cleaner history and better conflict resolution

echo "🔄 Syncing all worktrees with gambtho/mcp..."

# Configuration
SYNC_METHOD="${SYNC_METHOD:-rebase}"  # rebase (default) or merge
DRY_RUN="${DRY_RUN:-false}"           # Set to true for testing

# Save current directory
ORIGINAL_DIR=$(pwd)

# Find all worktrees
WORKTREES=$(git worktree list --porcelain | grep "worktree " | cut -d' ' -f2)

for WORKTREE in $WORKTREES; do
    echo ""
    echo "📂 Syncing worktree: $WORKTREE"
    cd "$WORKTREE"

    # Get current branch
    CURRENT_BRANCH=$(git branch --show-current)

    # Skip if we're on gambtho/mcp itself to prevent circular merges
    if [ "$CURRENT_BRANCH" = "gambtho/mcp" ]; then
        echo "  ⏭️  Skipping gambtho/mcp branch (base branch should not be modified)"
        continue
    fi

    # Check if this is a dry run
    if [ "$DRY_RUN" = "true" ]; then
        echo "  🧪 DRY RUN: Would sync $CURRENT_BRANCH with origin/gambtho/mcp"
        continue
    fi

    # Check for uncommitted changes
    HAS_UNCOMMITTED=false
    if ! git diff --quiet || ! git diff --cached --quiet; then
        echo "  💾 Stashing uncommitted changes..."
        git stash push -m "Auto-stash before sync $(date)"
        HAS_UNCOMMITTED=true
    fi

    # Fetch latest
    echo "  📥 Fetching latest changes..."
    git fetch origin

    # Check if there are changes to sync
    LOCAL_COMMIT=$(git rev-parse HEAD)
    REMOTE_COMMIT=$(git rev-parse origin/gambtho/mcp)

    if [ "$LOCAL_COMMIT" = "$REMOTE_COMMIT" ]; then
        echo "  ✅ Already up to date with origin/gambtho/mcp"
    else
        # Apply sync method (rebase or merge)
        if [ "$SYNC_METHOD" = "rebase" ]; then
            echo "  🔄 Rebasing $CURRENT_BRANCH onto origin/gambtho/mcp..."
            if git rebase origin/gambtho/mcp; then
                echo "  ✅ Rebase successful"
            else
                echo "  ⚠️  Rebase conflicts detected - manual resolution required"
                echo "     In $WORKTREE:"
                echo "     1. Resolve conflicts in the files shown by 'git status'"
                echo "     2. Run 'git add <resolved-files>'"
                echo "     3. Run 'git rebase --continue'"
                echo "     4. Or run 'git rebase --abort' to cancel"
                continue
            fi
        else
            echo "  🔀 Merging origin/gambtho/mcp into $CURRENT_BRANCH..."
            if git merge origin/gambtho/mcp --no-edit; then
                echo "  ✅ Merge successful"
            else
                echo "  ⚠️  Merge conflicts detected - manual resolution required"
                echo "     In $WORKTREE:"
                echo "     1. Resolve conflicts in the files shown by 'git status'"
                echo "     2. Run 'git add <resolved-files>'"
                echo "     3. Run 'git commit'"
                continue
            fi
        fi
    fi

    # Pop stash if we stashed
    if [ "$HAS_UNCOMMITTED" = "true" ]; then
        if git stash list | grep -q "Auto-stash before sync"; then
            echo "  📤 Restoring stashed changes..."
            if ! git stash pop; then
                echo "  ⚠️  Stash conflicts detected"
                echo "     In $WORKTREE:"
                echo "     1. Resolve any stash conflicts manually"
                echo "     2. Run 'git stash drop' to discard the auto-stash if needed"
                continue
            fi
        fi
    fi

    # Quick build check (optional)
    if command -v go >/dev/null 2>&1; then
        echo "  🔨 Quick build check..."
        if timeout 30s go build ./... >/dev/null 2>&1; then
            echo "  ✅ Build check passed"
        else
            echo "  ⚠️  Build check failed - may need attention"
        fi
    fi

    # Safety check: Ensure we never accidentally push to gambtho/mcp
    if [ "$CURRENT_BRANCH" != "gambtho/mcp" ]; then
        echo "  🔒 Safety check: Branch $CURRENT_BRANCH confirmed (not gambtho/mcp)"
    fi

    echo "  ✅ Worktree synced!"
done

# Return to original directory
cd "$ORIGINAL_DIR"
echo ""
echo "✨ Sync process completed!"
echo ""
echo "📋 Usage examples:"
echo "   SYNC_METHOD=merge ./scripts/sync-worktrees.sh    # Use merge instead of rebase"
echo "   DRY_RUN=true ./scripts/sync-worktrees.sh         # Preview changes without applying"
echo ""
echo "🔒 NOTE: This script only pulls FROM gambtho/mcp. Never push TO gambtho/mcp from worktrees."
