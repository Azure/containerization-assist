import { describe, it, expect, beforeEach, afterEach } from '@jest/globals';
import { mkdirSync, writeFileSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { findGitRoot } from '@/lib/git-root';

describe('Git Root Detection', () => {
  let testDir: string;
  let originalCwd: string;

  beforeEach(() => {
    testDir = join(__dirname, 'test-git-root-' + Date.now());
    mkdirSync(testDir, { recursive: true });
    originalCwd = process.cwd();
  });

  afterEach(() => {
    rmSync(testDir, { recursive: true, force: true });
    process.chdir(originalCwd);
  });

  it('should find .git directory in cwd', () => {
    // Setup: Create .git directory in testDir
    const gitDir = join(testDir, '.git');
    mkdirSync(gitDir);

    // Test: findGitRoot should return testDir
    const result = findGitRoot(testDir);
    expect(result).toBe(testDir);
  });

  it('should find .git directory N parents up', () => {
    // Setup: Create nested directory structure with .git at root
    const gitDir = join(testDir, '.git');
    mkdirSync(gitDir);

    const nestedDir = join(testDir, 'src', 'lib', 'utils');
    mkdirSync(nestedDir, { recursive: true });

    // Test: findGitRoot from nested directory should return testDir
    const result = findGitRoot(nestedDir);
    expect(result).toBe(testDir);
  });

  it('should return null when no .git found', () => {
    // Setup: Create a directory without .git
    const emptyDir = join(testDir, 'empty');
    mkdirSync(emptyDir);

    // Test: findGitRoot should return null
    const result = findGitRoot(emptyDir);
    expect(result).toBeNull();
  });

  it('should handle .git as file (git worktree)', () => {
    // Setup: Create .git file with gitdir content (git worktree format)
    const gitFile = join(testDir, '.git');
    writeFileSync(gitFile, 'gitdir: /path/to/actual/worktree/.git\n');

    // Test: findGitRoot should return testDir
    const result = findGitRoot(testDir);
    expect(result).toBe(testDir);
  });

  it('should not infinite loop at filesystem root', () => {
    // Setup: Use a directory that definitely has no .git far up
    const deepDir = join(testDir, 'a', 'b', 'c', 'd', 'e', 'f');
    mkdirSync(deepDir, { recursive: true });

    // Test: findGitRoot should return null without hanging
    const result = findGitRoot(deepDir);
    expect(result).toBeNull();
  });

  it('should find .git from cwd when no startDir provided', () => {
    // Setup: Create .git directory and change to it
    const gitDir = join(testDir, '.git');
    mkdirSync(gitDir);
    process.chdir(testDir);

    // Test: findGitRoot() without arguments should return testDir
    const result = findGitRoot();
    expect(result).toBe(testDir);
  });

  it('should respect MAX_PARENT_DIR_TRAVERSALS limit', () => {
    // Setup: Create very deep directory structure (beyond traversal limit)
    const deepPath = [testDir, ...Array(10).fill('nested')].join('/');
    mkdirSync(deepPath, { recursive: true });

    // Create .git at testDir
    mkdirSync(join(testDir, '.git'));

    // Test: Starting from deep nested dir, should find .git within limit
    // (MAX_PARENT_DIR_TRAVERSALS = 5, so going up 10 levels should not find it)
    const result = findGitRoot(deepPath);
    // With our traversal limit of 5, we can go up at most 5 levels
    // 10 levels deep > 5 levels, so should return null
    expect(result).toBeNull();
  });
});
