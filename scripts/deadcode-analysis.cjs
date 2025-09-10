#!/usr/bin/env node

/**
 * Accurate Dead Code Analysis
 * 
 * Filters ts-prune output to exclude:
 * - Public API exports (src/index.ts, src/exports/*)
 * - MCP tool handlers (dynamically registered)
 * - Schema exports (required for validation)
 */

const { execSync } = require('child_process');

// Configuration
const PUBLIC_API_PATTERNS = [
  /^src\/index\.ts:\d+/,
  /^src\/exports\//,
];

const MCP_TOOL_PATTERNS = [
  /src\/tools\/[^/]+\/(tool|schema|index)\.ts:\d+ - \w+(Tool|Schema|Params|Result)/,
  /src\/mcp\/server\/schemas\.ts:\d+ - \w+Schema/,
];

const KEEP_PATTERNS = [
  /\(used in module\)$/, // Keep internal module exports
  ...PUBLIC_API_PATTERNS,
  ...MCP_TOOL_PATTERNS,
];

try {
  // Run ts-prune and get output
  const tsPruneOutput = execSync('npx ts-prune', { encoding: 'utf-8' });
  const lines = tsPruneOutput.trim().split('\n');
  
  // Filter out patterns we want to keep
  const actualDeadCode = lines.filter(line => {
    return !KEEP_PATTERNS.some(pattern => pattern.test(line));
  });
  
  // Statistics
  const totalExports = lines.length;
  const publicApiExports = lines.filter(line => 
    PUBLIC_API_PATTERNS.some(pattern => pattern.test(line))
  ).length;
  const mcpExports = lines.filter(line =>
    MCP_TOOL_PATTERNS.some(pattern => pattern.test(line))
  ).length;
  const internalExports = lines.filter(line => 
    /\(used in module\)$/.test(line)
  ).length;
  const deadExports = actualDeadCode.length;
  
  console.log('=== Dead Code Analysis Report ===');
  console.log(`Total exports found: ${totalExports}`);
  console.log(`├─ Internal usage: ${internalExports}`);
  console.log(`├─ Public API: ${publicApiExports}`);
  console.log(`├─ MCP tools: ${mcpExports}`);
  console.log(`└─ Actually dead: ${deadExports}`);
  console.log();
  
  if (deadExports > 0) {
    console.log('=== Potentially Removable Exports ===');
    actualDeadCode.slice(0, 20).forEach(line => console.log(line));
    if (deadExports > 20) {
      console.log(`... and ${deadExports - 20} more`);
    }
  }
  
  // Exit with count of dead exports for CI
  process.exit(deadExports > 50 ? 1 : 0);
  
} catch (error) {
  console.error('Failed to run deadcode analysis:', error.message);
  process.exit(1);
}