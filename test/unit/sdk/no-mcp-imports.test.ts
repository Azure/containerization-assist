/**
 * SDK No-MCP-Imports Tests
 *
 * Verifies that the SDK and core modules have no dependencies on MCP packages.
 * This is critical for consumers who want to use tools without pulling in MCP deps.
 */

import { describe, test, expect } from '@jest/globals';
import { readFileSync, readdirSync, existsSync } from 'node:fs';
import { join } from 'node:path';

const ROOT_DIR = process.cwd();
const SDK_DIR = join(ROOT_DIR, 'src/sdk');
const CORE_DIR = join(ROOT_DIR, 'src/core');

function getTypeScriptFiles(dir: string): string[] {
  if (!existsSync(dir)) {
    return [];
  }
  try {
    return readdirSync(dir)
      .filter((f) => f.endsWith('.ts') && !f.endsWith('.test.ts'))
      .map((f) => join(dir, f));
  } catch {
    return [];
  }
}

function checkFileForMCPImports(filePath: string): string[] {
  const content = readFileSync(filePath, 'utf-8');
  const violations: string[] = [];
  const lines = content.split('\n');

  lines.forEach((line, index) => {
    // Skip comments
    if (line.trim().startsWith('//') || line.trim().startsWith('*')) {
      return;
    }

    // Check for direct MCP SDK imports
    if (line.includes('@modelcontextprotocol') && line.includes('import')) {
      violations.push(`${filePath}:${index + 1}: imports from @modelcontextprotocol`);
    }

    // Check for imports from @/mcp (internal MCP module)
    if (line.includes("from '@/mcp") && line.includes('import')) {
      violations.push(`${filePath}:${index + 1}: imports from @/mcp`);
    }

    // Check for relative imports to mcp directory
    if (line.includes("from '../mcp") || line.includes("from '../../mcp")) {
      violations.push(`${filePath}:${index + 1}: relative import from mcp directory`);
    }
  });

  return violations;
}

describe('SDK MCP Independence', () => {
  describe('src/sdk/', () => {
    test('SDK directory exists', () => {
      expect(existsSync(SDK_DIR)).toBe(true);
    });

    test('has no MCP imports in any file', () => {
      const files = getTypeScriptFiles(SDK_DIR);
      expect(files.length).toBeGreaterThan(0);

      const allViolations = files.flatMap(checkFileForMCPImports);

      if (allViolations.length > 0) {
        console.error('MCP import violations found:');
        allViolations.forEach((v) => console.error(`  ${v}`));
      }

      expect(allViolations).toEqual([]);
    });

    test('executor.ts has no MCP imports', () => {
      const executorPath = join(SDK_DIR, 'executor.ts');
      expect(existsSync(executorPath)).toBe(true);

      const violations = checkFileForMCPImports(executorPath);
      expect(violations).toEqual([]);
    });

    test('index.ts has no MCP imports', () => {
      const indexPath = join(SDK_DIR, 'index.ts');
      expect(existsSync(indexPath)).toBe(true);

      const violations = checkFileForMCPImports(indexPath);
      expect(violations).toEqual([]);
    });

    test('types.ts has no MCP imports', () => {
      const typesPath = join(SDK_DIR, 'types.ts');
      expect(existsSync(typesPath)).toBe(true);

      const violations = checkFileForMCPImports(typesPath);
      expect(violations).toEqual([]);
    });
  });

  describe('src/core/', () => {
    test('core directory exists', () => {
      expect(existsSync(CORE_DIR)).toBe(true);
    });

    test('has no MCP imports in any file', () => {
      const files = getTypeScriptFiles(CORE_DIR);
      expect(files.length).toBeGreaterThan(0);

      const allViolations = files.flatMap(checkFileForMCPImports);

      if (allViolations.length > 0) {
        console.error('MCP import violations found:');
        allViolations.forEach((v) => console.error(`  ${v}`));
      }

      expect(allViolations).toEqual([]);
    });

    test('context.ts has no MCP imports', () => {
      const contextPath = join(CORE_DIR, 'context.ts');
      expect(existsSync(contextPath)).toBe(true);

      const violations = checkFileForMCPImports(contextPath);
      expect(violations).toEqual([]);
    });
  });
});

describe('SDK File Structure', () => {
  test('SDK has expected files', () => {
    const expectedFiles = ['executor.ts', 'index.ts', 'types.ts'];

    expectedFiles.forEach((file) => {
      const filePath = join(SDK_DIR, file);
      expect(existsSync(filePath)).toBe(true);
    });
  });

  test('Core has expected files', () => {
    const expectedFiles = ['context.ts', 'index.ts'];

    expectedFiles.forEach((file) => {
      const filePath = join(CORE_DIR, file);
      expect(existsSync(filePath)).toBe(true);
    });
  });
});

describe('SDK Import Chain Validation', () => {
  test('SDK imports from @/core/context, not @/mcp/context', () => {
    const executorPath = join(SDK_DIR, 'executor.ts');
    const content = readFileSync(executorPath, 'utf-8');

    // Should import from core
    expect(content).toContain("from '@/core/context'");

    // Should NOT import from mcp
    expect(content).not.toMatch(/import.*from\s+['"]@\/mcp\/context['"]/);
  });

  test('SDK types imports from @/core/context', () => {
    const typesPath = join(SDK_DIR, 'types.ts');
    const content = readFileSync(typesPath, 'utf-8');

    // Should import from core
    expect(content).toContain("from '@/core/context'");
  });
});
