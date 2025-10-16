import tmp from 'tmp';
import type { DirResult, FileResult } from 'tmp';

/**
 * Creates a temporary directory for testing with automatic cleanup.
 *
 * @param prefix - Optional prefix for the directory name
 * @returns Object with directory path and cleanup function
 *
 * @example
 * const { dir, cleanup } = createTestTempDir('my-test-');
 * // Use dir.name
 * await cleanup(); // Optional: cleanup() is called automatically on exit
 */
export function createTestTempDir(prefix = 'test-'): {
  dir: DirResult;
  cleanup: () => Promise<void>;
} {
  const dir = tmp.dirSync({
    prefix,
    unsafeCleanup: true, // Remove dir even if not empty
    keep: false, // Delete on exit
  });

  const cleanup = async (): Promise<void> => {
    try {
      dir.removeCallback();
    } catch (error) {
      // Graceful degradation - already cleaned up
      console.warn(`Temp dir cleanup already completed: ${dir.name}`);
    }
  };

  return { dir, cleanup };
}

/**
 * Creates a temporary file for testing with automatic cleanup.
 *
 * @param prefix - Optional prefix for the file name
 * @param postfix - Optional postfix (e.g., '.json')
 * @returns Object with file path and cleanup function
 */
export function createTestTempFile(
  prefix = 'test-',
  postfix = ''
): {
  file: FileResult;
  cleanup: () => Promise<void>;
} {
  const file = tmp.fileSync({
    prefix,
    postfix,
    keep: false,
  });

  const cleanup = async (): Promise<void> => {
    try {
      file.removeCallback();
    } catch (error) {
      console.warn(`Temp file cleanup already completed: ${file.name}`);
    }
  };

  return { file, cleanup };
}

/**
 * Creates a temporary directory that persists for debugging.
 * Useful for integration/e2e tests where you want to inspect results.
 *
 * @param prefix - Optional prefix for the directory name
 * @returns Object with directory path and manual cleanup function
 */
export function createPersistentTestDir(prefix = 'test-persist-'): {
  dir: DirResult;
  cleanup: () => Promise<void>;
} {
  const dir = tmp.dirSync({
    prefix,
    unsafeCleanup: true,
    keep: true, // Don't auto-delete
  });

  const cleanup = async (): Promise<void> => {
    dir.removeCallback();
  };

  return { dir, cleanup };
}
