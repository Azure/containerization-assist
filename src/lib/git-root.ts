/**
 * Git Root Detection Utility
 * Walks upward from a starting directory to find the nearest .git directory
 * Handles both normal repositories (.git/) and git worktrees (.git file)
 */

import { existsSync, statSync, readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';

/**
 * Maximum number of parent directories to traverse when searching for .git
 * Prevents infinite loops at filesystem root
 */
const MAX_PARENT_DIR_TRAVERSALS = 5;

/**
 * Find the git root directory by walking upward from startDir
 *
 * Searches for either:
 * - A `.git/` directory (normal repository)
 * - A `.git` file (git worktree, containing `gitdir: ...`)
 *
 * @param startDir - Starting directory (defaults to process.cwd())
 * @returns Absolute path to the directory containing .git, or null if not found
 *
 * @example
 * // From /home/user/project/src/lib:
 * findGitRoot() // Returns '/home/user/project'
 *
 * @example
 * // No .git found:
 * findGitRoot() // Returns null
 */
export function findGitRoot(startDir?: string): string | null {
  let currentDir = startDir ? resolve(startDir) : process.cwd();

  for (let i = 0; i < MAX_PARENT_DIR_TRAVERSALS; i++) {
    const gitPath = resolve(currentDir, '.git');

    // Check if .git exists (either directory or file)
    if (existsSync(gitPath)) {
      try {
        const stats = statSync(gitPath);

        // Handle .git as directory (normal repository)
        if (stats.isDirectory()) {
          return currentDir;
        }

        // Handle .git as file (git worktree)
        if (stats.isFile()) {
          // For worktrees, .git is a file containing "gitdir: <path>"
          // We still return the current directory as the root
          const content = readFileSync(gitPath, 'utf-8');
          if (content.includes('gitdir:')) {
            return currentDir;
          }
        }
      } catch {
        // If we can't stat or read the file, continue searching
      }
    }

    // Walk to parent directory
    const parentDir = dirname(currentDir);
    if (parentDir === currentDir) {
      // Reached filesystem root
      break;
    }
    currentDir = parentDir;
  }

  return null;
}
