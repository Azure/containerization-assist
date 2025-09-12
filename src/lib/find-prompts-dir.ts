import { existsSync } from 'fs';
import { join } from 'path';

/**
 * Find the prompts directory by checking multiple possible locations
 * This handles both development and production (npm package) scenarios
 */
export function findPromptsDirectory(): string {
  const possiblePaths = [
    // Development: src/prompts
    join(process.cwd(), 'src', 'prompts'),
    // Built ESM: dist/src/prompts
    join(process.cwd(), 'dist', 'src', 'prompts'),
    // Built CJS: dist-cjs/src/prompts
    join(process.cwd(), 'dist-cjs', 'src', 'prompts'),
  ];

  // Try to find prompts relative to the module location for npm package installs
  try {
    // Get the directory where this module is located
    const moduleDir = __dirname;

    // Check various relative paths from the module location
    possiblePaths.push(
      join(moduleDir, '..', 'prompts'), // If we're in lib, prompts is a sibling
      join(moduleDir, '..', '..', 'prompts'), // If we're deeper
      join(moduleDir, '..', '..', 'src', 'prompts'), // Full src path from dist
    );
  } catch {
    // Ignore errors in finding module directory
  }

  // Find the first existing path
  for (const path of possiblePaths) {
    if (existsSync(path)) {
      return path;
    }
  }

  // Default fallback
  return join(process.cwd(), 'src', 'prompts');
}
