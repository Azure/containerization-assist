#!/usr/bin/env node
/**
 * Validates that all public API exports are present
 * This script ensures critical exports used in documentation examples are not removed
 */

import { existsSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';

interface PublicExport {
  file: string;
  exportName: string;
  exportType: 'value' | 'type';
  critical: boolean; // Used in docs/examples
}

/**
 * Critical public API exports that must be protected
 * These are used in documentation examples and must not be removed
 */
const PUBLIC_EXPORTS: PublicExport[] = [
  // Primary API - Used in docs/examples
  { file: 'src/index.ts', exportName: 'createApp', exportType: 'value', critical: true },
  { file: 'src/tools/index.ts', exportName: 'ALL_TOOLS', exportType: 'value', critical: true },

  // Core types - Public API
  { file: 'src/index.ts', exportName: 'Result', exportType: 'type', critical: true },
  { file: 'src/index.ts', exportName: 'Success', exportType: 'type', critical: true },
  { file: 'src/index.ts', exportName: 'Failure', exportType: 'type', critical: true },
  { file: 'src/index.ts', exportName: 'Tool', exportType: 'type', critical: true },

  // Runtime types - Public API
  { file: 'src/index.ts', exportName: 'AppRuntime', exportType: 'type', critical: true },
  { file: 'src/index.ts', exportName: 'AppRuntimeConfig', exportType: 'type', critical: true },
  { file: 'src/index.ts', exportName: 'TransportConfig', exportType: 'type', critical: true },

  // Tool creation helper
  { file: 'src/index.ts', exportName: 'tool', exportType: 'value', critical: true },

  // Individual tool exports - Public API
  { file: 'src/index.ts', exportName: 'analyzeRepoTool', exportType: 'value', critical: false },
  { file: 'src/index.ts', exportName: 'buildImageTool', exportType: 'value', critical: false },
  { file: 'src/index.ts', exportName: 'fixDockerfileTool', exportType: 'value', critical: false },
  { file: 'src/index.ts', exportName: 'generateDockerfileTool', exportType: 'value', critical: false },
  { file: 'src/index.ts', exportName: 'generateK8sManifestsTool', exportType: 'value', critical: false },
  { file: 'src/index.ts', exportName: 'opsTool', exportType: 'value', critical: false },
  { file: 'src/index.ts', exportName: 'prepareClusterTool', exportType: 'value', critical: false },
  { file: 'src/index.ts', exportName: 'pushImageTool', exportType: 'value', critical: false },
  { file: 'src/index.ts', exportName: 'scanImageTool', exportType: 'value', critical: false },
  { file: 'src/index.ts', exportName: 'tagImageTool', exportType: 'value', critical: false },
  { file: 'src/index.ts', exportName: 'verifyDeployTool', exportType: 'value', critical: false },

  // Utility exports - Public API
  { file: 'src/index.ts', exportName: 'extractSchemaShape', exportType: 'value', critical: false },
  { file: 'src/index.ts', exportName: 'ZodRawShape', exportType: 'type', critical: false },
];

function checkExportExists(filePath: string, exportName: string, exportType: 'value' | 'type'): boolean {
  const absolutePath = resolve(process.cwd(), filePath);

  if (!existsSync(absolutePath)) {
    return false;
  }

  const content = readFileSync(absolutePath, 'utf-8');

  // Check for different export patterns
  const patterns = [
    // export { name }
    new RegExp(`export\\s*{[^}]*\\b${exportName}\\b[^}]*}`, 'm'),
    // export type { name }
    new RegExp(`export\\s+type\\s*{[^}]*\\b${exportName}\\b[^}]*}`, 'm'),
    // export const name
    new RegExp(`export\\s+const\\s+${exportName}\\b`, 'm'),
    // export function name
    new RegExp(`export\\s+function\\s+${exportName}\\b`, 'm'),
    // export type name
    new RegExp(`export\\s+type\\s+${exportName}\\b`, 'm'),
    // export interface name
    new RegExp(`export\\s+interface\\s+${exportName}\\b`, 'm'),
    // /** @public */ export type { name }
    new RegExp(`@public[^\\n]*\\n[^\\n]*export\\s+type\\s*{[^}]*\\b${exportName}\\b`, 'm'),
  ];

  return patterns.some(pattern => pattern.test(content));
}

function validatePublicAPI(): { success: boolean; errors: string[] } {
  const errors: string[] = [];
  let criticalFailures = 0;
  let warnings = 0;

  console.log('🔍 Validating public API exports...\n');

  for (const exportDef of PUBLIC_EXPORTS) {
    const exists = checkExportExists(exportDef.file, exportDef.exportName, exportDef.exportType);

    if (!exists) {
      const severity = exportDef.critical ? '❌ CRITICAL' : '⚠️  WARNING';
      const message = `${severity}: Export missing: ${exportDef.exportName} (${exportDef.exportType}) in ${exportDef.file}`;
      errors.push(message);

      if (exportDef.critical) {
        criticalFailures++;
      } else {
        warnings++;
      }
    }
  }

  if (errors.length === 0) {
    console.log('✅ All public API exports present');
    console.log(`   Validated ${PUBLIC_EXPORTS.length} exports\n`);
    return { success: true, errors: [] };
  }

  console.error('Public API validation failed:\n');
  errors.forEach(error => console.error(`  ${error}`));
  console.error('');
  console.error(`Summary: ${criticalFailures} critical failures, ${warnings} warnings`);
  console.error('');

  if (criticalFailures > 0) {
    console.error('❌ CRITICAL: Documentation examples will break!');
    console.error('   These exports are used in docs/examples/');
    console.error('   DO NOT remove these exports!');
    console.error('');
  }

  return { success: criticalFailures === 0, errors };
}

// Run validation
const result = validatePublicAPI();

if (!result.success) {
  process.exit(1);
}
