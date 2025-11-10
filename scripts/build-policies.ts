#!/usr/bin/env tsx
/**
 * Build Script: Compile Rego Policies to WASM
 *
 * Compiles all .rego policy files in the policies/ directory to .wasm bundles
 * for fast, zero-dependency runtime evaluation.
 *
 * Requires: OPA CLI (development dependency only)
 */

import { readdir, mkdir, readFile, writeFile, rm, stat } from 'node:fs/promises';
import { existsSync } from 'node:fs';
import { join } from 'node:path';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import * as tar from 'tar';

const execFileAsync = promisify(execFile);

const POLICIES_DIR = 'policies';
const OUTPUT_DIR = 'policies/compiled';
// Use result as entrypoint - this gives us the full policy structure
const ENTRYPOINTS = [
  'containerization/security/result',
  'containerization/base_images/result',
  'containerization/best_practices/result',
];

interface BuildResult {
  policy: string;
  success: boolean;
  wasmPath?: string;
  error?: string;
}

/**
 * Get OPA binary path
 */
function getOpaBinaryPath(): string {
  const opaBinaryName = process.platform === 'win32' ? 'opa.exe' : 'opa';
  const localOpa = join(process.cwd(), 'node_modules', '.bin', opaBinaryName);
  if (existsSync(localOpa)) {
    return localOpa;
  }
  return opaBinaryName;
}

/**
 * Check if OPA is available
 */
async function checkOpaAvailable(): Promise<boolean> {
  try {
    const opaBinary = getOpaBinaryPath();
    await execFileAsync(opaBinary, ['version']);
    return true;
  } catch {
    return false;
  }
}

/**
 * Compile all .rego policies to a single .wasm bundle
 */
async function compilePoliciesToWasm(regoPaths: string[]): Promise<BuildResult> {
  const policyName = 'all-policies';

  try {
    console.log(`  Compiling ${regoPaths.length} policies into single bundle...`);

    const opaBinary = getOpaBinaryPath();
    const bundlePath = join(OUTPUT_DIR, 'policies.tar.gz');

    // Compile all policies to a single WASM bundle
    // Use multiple entrypoints for each policy module's result
    const entrypointArgs = ENTRYPOINTS.flatMap(e => ['-e', e]);
    await execFileAsync(opaBinary, [
      'build',
      '-t', 'wasm',
      ...entrypointArgs,
      ...regoPaths,
      '-o', bundlePath,
    ]);

    // Extract policy.wasm from the bundle
    const wasmPath = join(OUTPUT_DIR, 'policies.wasm');
    await extractWasmFromBundle(bundlePath, wasmPath);

    console.log(`  ✓ policies.wasm (${await getFileSize(wasmPath)})`);

    return {
      policy: policyName,
      success: true,
      wasmPath,
    };
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    console.error(`  ✗ Failed: ${message}`);
    return {
      policy: policyName,
      success: false,
      error: message,
    };
  }
}

/**
 * Extract policy.wasm from OPA bundle (.tar.gz)
 */
async function extractWasmFromBundle(bundlePath: string, outputPath: string): Promise<void> {
  const tempDir = join(OUTPUT_DIR, 'temp');

  try {
    // Create temp directory
    await mkdir(tempDir, { recursive: true });

    // Extract bundle to temp directory
    await tar.x({
      file: bundlePath,
      cwd: tempDir,
    });

    // Read policy.wasm from temp directory
    const wasmPath = join(tempDir, 'policy.wasm');
    if (!existsSync(wasmPath)) {
      throw new Error('policy.wasm not found in bundle');
    }

    const wasmData = await readFile(wasmPath);
    await writeFile(outputPath, wasmData);

    // Clean up
    await rm(tempDir, { recursive: true, force: true });
  } catch (error) {
    // Clean up on error
    await rm(tempDir, { recursive: true, force: true }).catch(err =>
      console.debug('Temp cleanup failed:', err)
    );
    throw error;
  }
}

/**
 * Get human-readable file size
 */
async function getFileSize(path: string): Promise<string> {
  const stats = await stat(path);
  const bytes = stats.size;
  const kb = bytes / 1024;
  if (kb < 1024) {
    return `${kb.toFixed(1)} KB`;
  }
  return `${(kb / 1024).toFixed(1)} MB`;
}

/**
 * Find all .rego policy files (excluding test files)
 */
async function findPolicyFiles(): Promise<string[]> {
  const entries = await readdir(POLICIES_DIR, { withFileTypes: true });
  return entries
    .filter(entry => entry.isFile() && entry.name.endsWith('.rego') && !entry.name.endsWith('_test.rego'))
    .map(entry => join(POLICIES_DIR, entry.name));
}

/**
 * Main build function
 */
async function main() {
  console.log('🔨 Building Policy WASM Bundles\n');

  // Check if WASM bundle already exists (CI/production builds)
  const wasmPath = join(OUTPUT_DIR, 'policies.wasm');
  if (existsSync(wasmPath)) {
    console.log('✓ Pre-built WASM bundle found - skipping compilation');
    console.log(`  Using existing: ${wasmPath}`);
    console.log('\n💡 To rebuild policies, delete policies/compiled/*.wasm and run again');
    return;
  }

  // Check OPA availability for development builds
  const opaAvailable = await checkOpaAvailable();
  if (!opaAvailable) {
    console.error('❌ OPA binary not found');
    console.error('   Install OPA: https://www.openpolicyagent.org/docs/latest/#running-opa');
    console.error('   Or use pre-built WASM from git (committed for CI/production)');
    console.error('\n💡 If you are in CI and seeing this, ensure policies/compiled/*.wasm is committed to git');
    process.exit(1);
  }

  // Create output directory
  if (!existsSync(OUTPUT_DIR)) {
    await mkdir(OUTPUT_DIR, { recursive: true });
    console.log(`📁 Created output directory: ${OUTPUT_DIR}\n`);
  }

  // Find all policy files
  const policyFiles = await findPolicyFiles();
  if (policyFiles.length === 0) {
    console.log('⚠️  No policy files found in policies/');
    return;
  }

  console.log(`Found ${policyFiles.length} policy file(s):\n`);

  // Compile all policies into a single bundle
  const result = await compilePoliciesToWasm(policyFiles);

  // Summary
  console.log('\n📊 Build Summary:');
  if (!result.success) {
    console.log(`  ✗ Failed: ${result.error}`);
    process.exit(1);
  }

  console.log(`\n✨ All policies compiled successfully!`);
  console.log(`   Output: ${OUTPUT_DIR}/`);
}

main().catch(error => {
  console.error('Build failed:', error);
  process.exit(1);
});
