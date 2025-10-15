#!/usr/bin/env node
/**
 * Validate policy YAML files against the schema
 */

import * as fs from 'node:fs';
import * as path from 'node:path';
import * as yaml from 'js-yaml';
import { validatePolicy } from '../src/config/policy-io.js';

const policiesDir = path.join(process.cwd(), 'policies');

async function validatePolicyFiles() {
  console.log('🔍 Validating policy files...\n');

  const files = fs
    .readdirSync(policiesDir)
    .filter((f) => f.endsWith('.yaml') || f.endsWith('.yml'));

  if (files.length === 0) {
    console.log('⚠️  No policy files found in policies/ directory');
    process.exit(1);
  }

  let hasErrors = false;

  for (const file of files) {
    const filePath = path.join(policiesDir, file);
    console.log(`📄 Validating ${file}...`);

    try {
      const content = fs.readFileSync(filePath, 'utf8');
      const parsed = yaml.load(content);

      const result = validatePolicy(parsed);

      if (result.ok) {
        console.log(`  ✅ Valid - ${result.value.rules?.length || 0} rules found`);
        if (result.value.metadata?.name) {
          console.log(`     Name: ${result.value.metadata.name}`);
        }
        if (result.value.metadata?.description) {
          console.log(`     Description: ${result.value.metadata.description}`);
        }
      } else {
        console.log(`  ❌ Invalid: ${result.error}`);
        hasErrors = true;
      }
    } catch (error) {
      console.log(`  ❌ Error reading/parsing: ${error}`);
      hasErrors = true;
    }

    console.log();
  }

  if (hasErrors) {
    console.log('❌ Policy validation failed');
    process.exit(1);
  } else {
    console.log('✅ All policy files are valid!');
    process.exit(0);
  }
}

validatePolicyFiles().catch((error) => {
  console.error('Fatal error:', error);
  process.exit(1);
});
