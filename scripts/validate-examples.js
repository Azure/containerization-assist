#!/usr/bin/env node

/**
 * Validates TypeScript examples for syntax correctness
 *
 * This script is used in CI (test-pipeline.yml) to ensure examples remain
 * syntactically valid TypeScript. It filters out type errors and module
 * resolution issues since examples use package self-reference patterns
 * that require build context.
 *
 * Exits with code 0 if all examples are valid, code 1 if syntax errors found.
 */

import { execSync } from 'child_process';
import { readdirSync } from 'fs';
import { join } from 'path';

const EXAMPLES_DIR = 'docs/examples';

// Error codes to filter out (type checking, not syntax)
const FILTERED_ERROR_CODES = [
  /TS23\d{2}:/, // Type errors (2300-2399)
  /TS24\d{2}:/, // Type assignment errors (2400-2499)
  /Cannot find module/,
  /has no exported member/,
];

function shouldFilterError(line) {
  return FILTERED_ERROR_CODES.some(pattern => pattern.test(line));
}

function validateFile(filename) {
  const filepath = join(EXAMPLES_DIR, filename);

  try {
    const output = execSync(
      `npx tsc --noEmit --skipLibCheck --module ES2022 --target ES2022 --moduleResolution bundler "${filepath}"`,
      { encoding: 'utf-8', stdio: 'pipe' }
    );
    return { filename, errors: [] };
  } catch (error) {
    // TypeScript exits with non-zero on errors
    const output = error.stdout || error.stderr || '';
    const errorLines = output
      .split('\n')
      .filter(line => line.includes('error TS'))
      .filter(line => !shouldFilterError(line));

    return { filename, errors: errorLines };
  }
}

function main() {
  console.log('Validating TypeScript examples syntax...\n');

  const files = readdirSync(EXAMPLES_DIR)
    .filter(f => f.endsWith('.ts'));

  const results = files.map(validateFile);
  const failed = results.filter(r => r.errors.length > 0);

  if (failed.length > 0) {
    console.error('❌ Found compilation errors:\n');
    failed.forEach(({ filename, errors }) => {
      console.error(`${filename}:`);
      errors.forEach(err => console.error(`  ${err}`));
      console.error('');
    });
    process.exit(1);
  }

  console.log(`✅ All ${files.length} examples have valid TypeScript syntax`);
}

main();
