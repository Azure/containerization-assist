#!/usr/bin/env tsx
/**
 * Verify Build Outputs Script
 *
 * Validates that both ESM and CJS builds produce the expected artifacts:
 * - dist/ directory with ESM modules (.js), declarations (.d.ts), and source maps (.js.map)
 * - dist-cjs/ directory with CJS modules (.js) and declarations (.d.ts)
 * - All package.json exports have corresponding files in both formats
 * - CLI binary is executable
 *
 * Exit codes:
 * - 0: All checks passed
 * - 1: One or more checks failed
 */

import { existsSync, statSync, readFileSync } from 'node:fs';
import { readFile } from 'node:fs/promises';
import { join } from 'node:path';

interface ExportEntry {
  name: string;
  esmPath: string;
  cjsPath: string;
  typesPath: string;
}

let failureCount = 0;

function checkFile(path: string, description: string): boolean {
  if (!existsSync(path)) {
    console.error(`✗ ${description}: ${path} does not exist`);
    failureCount++;
    return false;
  }
  console.log(`✓ ${description}: ${path}`);
  return true;
}

function checkExecutable(path: string, description: string): boolean {
  if (!existsSync(path)) {
    console.error(`✗ ${description}: ${path} does not exist`);
    failureCount++;
    return false;
  }

  try {
    const stats = statSync(path);
    const isExecutable = !!(stats.mode & 0o111);
    if (!isExecutable) {
      console.error(`✗ ${description}: ${path} is not executable`);
      failureCount++;
      return false;
    }
    console.log(`✓ ${description}: ${path}`);
    return true;
  } catch (error) {
    console.error(`✗ ${description}: Failed to check ${path}`, error);
    failureCount++;
    return false;
  }
}

function checkModuleFormat(path: string, expectedFormat: 'esm' | 'cjs', description: string): boolean {
  if (!existsSync(path)) {
    console.error(`✗ ${description}: ${path} does not exist`);
    failureCount++;
    return false;
  }

  try {
    const content = readFileSync(path, 'utf-8');
    const lines = content.split('\n').slice(0, 10).join('\n');

    if (expectedFormat === 'cjs') {
      if (lines.includes('"use strict"') || lines.includes('exports.') || lines.includes('module.exports')) {
        console.log(`✓ ${description}: Correct CJS format`);
        return true;
      } else {
        console.error(`✗ ${description}: Does not appear to be CJS (missing "use strict", exports., or module.exports)`);
        failureCount++;
        return false;
      }
    } else {
      // ESM check - should NOT have "use strict" at the top and should use import/export
      if (lines.includes('"use strict"') && !lines.includes('import ') && !lines.includes('export ')) {
        console.error(`✗ ${description}: Appears to be CJS instead of ESM`);
        failureCount++;
        return false;
      }
      console.log(`✓ ${description}: Correct ESM format`);
      return true;
    }
  } catch (error) {
    console.error(`✗ ${description}: Failed to read ${path}`, error);
    failureCount++;
    return false;
  }
}

async function verifyBuildOutputs(): Promise<number> {
  console.log('=== Verifying Build Outputs ===\n');

  // Read package.json
  const packageJsonPath = join(process.cwd(), 'package.json');
  const packageJson = JSON.parse(await readFile(packageJsonPath, 'utf-8'));

  // Extract exports from package.json
  const exports: ExportEntry[] = [];
  for (const [exportName, exportConfig] of Object.entries(packageJson.exports || {})) {
    if (typeof exportConfig === 'object' && exportConfig !== null) {
      const config = exportConfig as Record<string, string>;
      exports.push({
        name: exportName,
        esmPath: config.import || config.default || '',
        cjsPath: config.require || '',
        typesPath: config.types || '',
      });
    }
  }

  console.log('--- Checking Package Exports ---\n');

  for (const exp of exports) {
    console.log(`Export: ${exp.name}`);

    // Check types (shared between ESM and CJS)
    if (exp.typesPath) {
      checkFile(exp.typesPath, `  Types`);
    }

    // Check ESM
    if (exp.esmPath) {
      checkFile(exp.esmPath, `  ESM module`);
      checkModuleFormat(exp.esmPath, 'esm', `  ESM format`);

      // Check for source map
      const sourceMapPath = exp.esmPath + '.map';
      checkFile(sourceMapPath, `  ESM source map`);
    }

    // Check CJS
    if (exp.cjsPath) {
      checkFile(exp.cjsPath, `  CJS module`);
      checkModuleFormat(exp.cjsPath, 'cjs', `  CJS format`);
    }

    console.log('');
  }

  // Check CLI binaries
  console.log('--- Checking CLI Binaries ---\n');
  for (const [binName, binPath] of Object.entries(packageJson.bin || {})) {
    checkExecutable(binPath as string, `Binary: ${binName}`);
  }
  console.log('');

  // Check directory structure
  console.log('--- Checking Directory Structure ---\n');
  checkFile('dist/src', 'ESM output directory');
  checkFile('dist-cjs/src', 'CJS output directory');
  console.log('');

  // Summary
  console.log('=== Summary ===\n');
  if (failureCount === 0) {
    console.log('✓ All build output checks passed');
    return 0;
  } else {
    console.error(`✗ ${failureCount} check(s) failed`);
    return 1;
  }
}

// Run verification
verifyBuildOutputs()
  .then((exitCode) => {
    process.exit(exitCode);
  })
  .catch((error) => {
    console.error('Fatal error during verification:', error);
    process.exit(1);
  });