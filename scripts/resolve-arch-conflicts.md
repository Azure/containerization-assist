# Resolving Arch Workstream Conflicts

The arch workstream currently has merge conflicts that need to be resolved. Here's the step-by-step process:

## Current Situation
The sync script successfully merged most changes, but there are stashed changes with conflicts in the arch worktree.

## Resolution Steps

### 1. Navigate to the arch worktree
```bash
cd /home/tng/workspace/arch
```

### 2. Check current status
```bash
git status
git stash list
```

### 3. Handle the conflicted stash
The sync script stashed uncommitted changes before merging, but couldn't automatically restore them due to conflicts.

```bash
# Try to apply the stash
git stash pop

# If conflicts occur, you'll see conflict markers in files
# Look for files with conflicts:
git status
```

### 4. Resolve conflicts manually
For each conflicted file (likely `pkg/mcp/internal/core/gomcp_tools.go`):

```bash
# Edit the file to resolve conflicts
# Look for conflict markers like:
# <<<<<<< HEAD
# (current changes)
# =======
# (stashed changes) 
# >>>>>>> Stashed changes

# Remove conflict markers and choose the correct code
```

### 5. Stage resolved files
```bash
git add <resolved-files>
```

### 6. Clean up the stash
```bash
# If you successfully resolved and committed:
git stash drop

# Or if you want to discard the stashed changes:
git stash drop
```

### 7. Verify the state
```bash
git status  # Should show clean working directory
go build ./...  # Verify everything builds
```

## Alternative: Start Fresh
If conflicts are too complex, you can reset and re-apply your arch-specific changes:

```bash
# CAUTION: This discards local changes
git stash drop  # Remove the problematic stash
git reset --hard origin/gambtho/mcp  # Reset to latest base
# Then re-implement your arch-specific changes
```

## Prevention for Future
1. Run sync more frequently (daily)
2. Use `DRY_RUN=true ./scripts/sync-worktrees.sh` to preview changes
3. Coordinate with other teams on shared files like `gomcp_tools.go`