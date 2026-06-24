#!/usr/bin/env node

/**
 * Cross-platform replacement for `rm -rf dist dist-cjs coverage .tsbuildinfo*`.
 * Used by the `clean` npm script so the build works on Windows (where `rm`
 * is not available in cmd/PowerShell).
 *
 * All paths are resolved relative to the repo root (derived from __dirname)
 * rather than process.cwd(), so the script is safe to invoke from any
 * working directory.
 */

const fs = require('fs');
const path = require('path');

const root = path.resolve(__dirname, '..');

const targets = ['dist', 'dist-cjs', 'coverage'];
for (const target of targets) {
  fs.rmSync(path.join(root, target), { recursive: true, force: true });
}

// Remove anything matching .tsbuildinfo* in the project root.
for (const entry of fs.readdirSync(root)) {
  if (entry.startsWith('.tsbuildinfo')) {
    fs.rmSync(path.join(root, entry), { recursive: true, force: true });
  }
}
