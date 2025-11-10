#!/usr/bin/env tsx
/**
 * Validate Script: Validate Rego Policy Files
 *
 * Validates all .rego policy files (built-in and examples) using OPA check.
 * Runs as part of CI/CD pipeline to ensure policy syntax correctness.
 *
 * Requires: OPA CLI (development dependency only)
 */

import { readdir } from 'node:fs/promises';
import { existsSync } from 'node:fs';
import { join } from 'node:path';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';

const execFileAsync = promisify(execFile);

const POLICY_DIRECTORIES = [
  { path: 'policies', label: 'Built-in Policies' },
  { path: 'policies.user.examples', label: 'Example Policies' },
];

interface ValidationResult {
  directory: string;
  file: string;
  success: boolean;
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
 * Find all .rego policy files in a directory (excluding test files)
 */
async function findPolicyFiles(dir: string): Promise<string[]> {
  if (!existsSync(dir)) {
    return [];
  }

  const entries = await readdir(dir, { withFileTypes: true });
  return entries
    .filter(entry => entry.isFile() && entry.name.endsWith('.rego') && !entry.name.endsWith('_test.rego'))
    .map(entry => join(dir, entry.name));
}

/**
 * Validate a single policy file using OPA check
 */
async function validatePolicyFile(
  filePath: string,
  directory: string
): Promise<ValidationResult> {
  const fileName = filePath.split('/').pop() || filePath;

  try {
    const opaBinary = getOpaBinaryPath();
    await execFileAsync(opaBinary, ['check', filePath]);

    return {
      directory,
      file: fileName,
      success: true,
    };
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    return {
      directory,
      file: fileName,
      success: false,
      error: message,
    };
  }
}

/**
 * Validate all policies in a directory
 */
async function validateDirectory(dir: string, label: string): Promise<ValidationResult[]> {
  console.log(`\n📂 ${label} (${dir}/):`);

  const files = await findPolicyFiles(dir);

  if (files.length === 0) {
    console.log('  No .rego files found');
    return [];
  }

  console.log(`  Found ${files.length} policy file(s)\n`);

  const results: ValidationResult[] = [];
  for (const file of files) {
    const fileName = file.split('/').pop() || file;
    process.stdout.write(`  Checking ${fileName}... `);

    const result = await validatePolicyFile(file, dir);
    results.push(result);

    if (result.success) {
      console.log('✓');
    } else {
      console.log('✗');
      if (result.error) {
        console.error(`    Error: ${result.error}`);
      }
    }
  }

  return results;
}

/**
 * Main validation function
 */
async function main() {
  console.log('🔍 Validating Policy Files\n');

  // Check OPA availability
  const opaAvailable = await checkOpaAvailable();
  if (!opaAvailable) {
    console.error('❌ OPA binary not found');
    console.error('   Install OPA: https://www.openpolicyagent.org/docs/latest/#running-opa');
    process.exit(1);
  }

  const opaBinary = getOpaBinaryPath();
  console.log(`Using OPA binary: ${opaBinary}`);

  // Validate all directories
  const allResults: ValidationResult[] = [];

  for (const { path, label } of POLICY_DIRECTORIES) {
    const results = await validateDirectory(path, label);
    allResults.push(...results);
  }

  // Summary
  console.log('\n📊 Validation Summary:');

  const totalFiles = allResults.length;
  const successCount = allResults.filter(r => r.success).length;
  const failureCount = totalFiles - successCount;

  console.log(`  Total files:    ${totalFiles}`);
  console.log(`  ✓ Passed:       ${successCount}`);
  console.log(`  ✗ Failed:       ${failureCount}`);

  if (failureCount > 0) {
    console.log('\n❌ Policy validation failed!');
    console.log('\nFailed files:');
    allResults
      .filter(r => !r.success)
      .forEach(r => {
        console.log(`  - ${r.directory}/${r.file}`);
      });
    process.exit(1);
  }

  console.log('\n✅ All policies validated successfully!');
}

main().catch(error => {
  console.error('Validation failed:', error);
  process.exit(1);
});
