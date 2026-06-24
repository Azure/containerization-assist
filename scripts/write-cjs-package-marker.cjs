#!/usr/bin/env node

/**
 * Cross-platform replacement for:
 *   printf '{"type": "commonjs"}\n' > dist-cjs/package.json
 *
 * Tells Node that .js files under dist-cjs/ are CommonJS, regardless of the
 * top-level package.json "type" field. Used as the final step of `build:cjs`
 * so the build works on Windows (where `printf` is not available in
 * cmd/PowerShell).
 */

const fs = require('fs');
const path = require('path');

const outDir = path.resolve(__dirname, '..', 'dist-cjs');
fs.mkdirSync(outDir, { recursive: true });
fs.writeFileSync(path.join(outDir, 'package.json'), '{"type": "commonjs"}\n');
