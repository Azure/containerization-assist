/**
 * Module URL Utilities
 *
 * Safely capture import.meta.url in both ESM and CJS builds.
 *
 * ## Why Use Function Constructor?
 *
 * import.meta is only available in ES modules. When TypeScript compiles to
 * CommonJS (CJS), or when bundlers like esbuild/webpack process the code,
 * they will error if they encounter import.meta syntax during static analysis.
 *
 * By wrapping the check in a Function constructor, we:
 * 1. Defer evaluation to runtime (bypassing static analysis)
 * 2. Return undefined gracefully in CJS environments
 * 3. Get the correct module URL in ESM environments
 *
 * ## ESM vs CJS Builds
 *
 * This package supports both module formats:
 * - **ESM** (dist/): import.meta.url is available, returns module URL
 * - **CJS** (dist-cjs/): import.meta.url is not available, returns undefined
 *
 * The module path resolver uses this URL (when available) to find resources
 * like policies and knowledge packs relative to the installed package location.
 *
 * @module lib/module-url
 */

/**
 * Capture import.meta.url at the call site
 *
 * Returns the URL of the calling module in ESM builds, or undefined in CJS builds.
 *
 * ## Usage
 *
 * Call at module scope (top-level) to capture the URL where it's invoked:
 *
 * ```typescript
 * import { getModuleUrl } from '@/lib/module-url';
 *
 * // Capture at module scope
 * const MODULE_URL = getModuleUrl();
 *
 * // Later, pass to functions that need it
 * function myFunction() {
 *   const paths = resolveModulePaths({
 *     relativePath: 'policies',
 *     logger,
 *     moduleUrl: MODULE_URL,
 *   });
 * }
 * ```
 *
 * ## Why Not Just Use import.meta.url Directly?
 *
 * ```typescript
 * // ❌ This breaks CJS builds
 * const MODULE_URL = import.meta?.url;
 *
 * // ❌ This also breaks (TypeScript/bundler error)
 * const MODULE_URL = typeof import.meta !== 'undefined' ? import.meta.url : undefined;
 *
 * // ✅ This works (runtime evaluation)
 * const MODULE_URL = getModuleUrl();
 * ```
 *
 * ## How It Works
 *
 * 1. Function constructor creates a function from a string at runtime
 * 2. The function checks if import.meta exists and has a url property
 * 3. In ESM: import.meta.url returns the module URL (file:// URL)
 * 4. In CJS: import.meta is undefined, returns undefined
 * 5. On error: catch block returns undefined (graceful fallback)
 *
 * @returns Module URL string in ESM builds, undefined in CJS builds
 *
 * @example
 * ```typescript
 * // ESM build (dist/)
 * const url = getModuleUrl();
 * console.log(url); // 'file:///path/to/dist/src/app/index.js'
 *
 * // CJS build (dist-cjs/)
 * const url = getModuleUrl();
 * console.log(url); // undefined
 * ```
 */
export function getModuleUrl(): string | undefined {
  try {
    // Use Function constructor to prevent static analysis of import.meta
    // This avoids TypeScript/bundler errors in CJS builds where import.meta is not available
    return new Function(
      'return typeof import.meta !== "undefined" && import.meta.url ? import.meta.url : undefined',
    )();
  } catch {
    // Gracefully handle any evaluation errors
    return undefined;
  }
}

/**
 * Check if running in ESM context
 *
 * Useful for conditional logic based on module format.
 *
 * @returns true if import.meta.url is available (ESM), false otherwise (CJS)
 *
 * @example
 * ```typescript
 * if (isESMContext()) {
 *   console.log('Running in ESM mode');
 * } else {
 *   console.log('Running in CJS mode');
 * }
 * ```
 */
export function isESMContext(): boolean {
  return getModuleUrl() !== undefined;
}
