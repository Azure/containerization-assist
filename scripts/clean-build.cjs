#!/usr/bin/env node

/**
 * Cross-platform replacement for `rm -rf dist dist-cjs coverage .tsbuildinfo*`.
 * Used by the `clean` npm script so the build works on Windows (where `rm`
 * is not available in cmd/PowerShell).
 */

const fs = require('fs');
const path = require('path');

const targets = ['dist', 'dist-cjs', 'coverage'];
for (const target of targets) {
  fs.rmSync(target, { recursive: true, force: true });
}

// Remove anything matching .tsbuildinfo* in the project root.
const root = path.resolve(__dirname, '..');
for (const entry of fs.readdirSync(root)) {
  if (entry.startsWith('.tsbuildinfo')) {
    fs.rmSync(path.join(root, entry), { recursive: true, force: true });
  }
}
