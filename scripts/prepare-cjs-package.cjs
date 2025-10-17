#!/usr/bin/env node

/**
 * Script to prepare package.json for CommonJS-only publishing
 * This is run automatically by npm during the publish process
 */

const fs = require('fs');
const path = require('path');

const packagePath = path.join(__dirname, '..', 'package.json');
const backupPath = path.join(__dirname, '..', '.package.json.backup');

// Check if we're in publish mode (prepack/prepublishOnly was called)
const isPublishing = process.env.npm_lifecycle_event === 'prepack' || 
                     process.env.npm_lifecycle_event === 'prepublishOnly';

if (!isPublishing) {
  console.log('Not in publish mode, skipping package.json modification');
  process.exit(0);
}

console.log('📦 Preparing package.json for CommonJS-only publish...');

// Read current package.json
const pkg = JSON.parse(fs.readFileSync(packagePath, 'utf8'));

// Backup original package.json
fs.writeFileSync(backupPath, JSON.stringify(pkg, null, 2));

// Modify for CommonJS only
delete pkg.type; // Remove "module" type
pkg.main = './dist-cjs/src/index.js';

// Update exports to only include CommonJS
for (const key in pkg.exports) {
  if (typeof pkg.exports[key] === 'object') {
    const cjsPath = pkg.exports[key].require;
    if (cjsPath) {
      pkg.exports[key] = {
        types: pkg.exports[key].types?.replace('dist/', 'dist-cjs/'),
        require: cjsPath,
        default: cjsPath
      };
      delete pkg.exports[key].import;
    }
  }
}

// Keep binary paths as ESM (they use import.meta)
// No need to update binary paths

// Include both ESM and CJS files
if (!pkg.files.includes('dist/**/*')) {
  pkg.files.push('dist/**/*');
}
if (!pkg.files.includes('dist-cjs/**/*')) {
  pkg.files.push('dist-cjs/**/*');
}

// Write modified package.json
fs.writeFileSync(packagePath, JSON.stringify(pkg, null, 2));

console.log('✅ package.json prepared for CommonJS-only publish');